package wallpaper

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true,
	".mov": true, ".webm": true, ".m4v": true, ".flv": true,
	".gif": true,
}

func IsVideoFile(path string) bool {
	return videoExts[strings.ToLower(filepath.Ext(path))]
}

type VideoTarget struct {
	Path string
	X, Y int
	W, H int
}

// ── Session state ─────────────────────────────────────────────────────────────
// Tracks the currently running mpv processes and desktop windows so that a new
// call to RunVideoWallpapers can cleanly tear down the previous session before
// starting a fresh one.

const wmClose uintptr = 0x0010

var (
	sessionMu    sync.Mutex
	sessionStop  chan struct{} // closed to signal spawnMpv goroutines to exit
	sessionCmds  []*exec.Cmd  // running mpv processes
	sessionHwnds []uintptr    // desktop windows to destroy on stop
)

// stopSession kills all running mpv processes, signals goroutines to stop, and
// posts WM_CLOSE to every desktop window so their message loops exit.
// Must be called with sessionMu held.
func stopSession() {
	if sessionStop != nil {
		close(sessionStop)
		sessionStop = nil
	}
	for _, c := range sessionCmds {
		if c.Process != nil {
			_ = c.Process.Kill()
		}
	}
	sessionCmds = nil
	for _, h := range sessionHwnds {
		postMessageW.Call(h, wmClose, 0, 0)
	}
	sessionHwnds = nil
}

// ── Public API ────────────────────────────────────────────────────────────────

func RunVideoWallpapers(targets []VideoTarget) error {
	sessionMu.Lock()
	stopSession()
	stop := make(chan struct{})
	sessionStop = stop
	sessionMu.Unlock()

	for _, t := range targets {
		hwnd, _, err := CreateDesktopWindow(t.X, t.Y, t.W, t.H)
		if err != nil {
			return fmt.Errorf("desktop window for %s: %w", t.Path, err)
		}
		sessionMu.Lock()
		sessionHwnds = append(sessionHwnds, hwnd)
		sessionMu.Unlock()

		go fadeInWindow(hwnd)
		go spawnMpv(t.Path, hwnd, stop)
	}

	RunMessageLoop()
	return nil
}

// StopVideoWallpapers tears down the current session (called on app exit or
// when switching away from video mode entirely).
func StopVideoWallpapers() {
	sessionMu.Lock()
	stopSession()
	sessionMu.Unlock()
}

// PreprocessVideo transcodes src to a resolution-matched mp4 in the livepaper
// temp dir. Returns the cached output path (skips ffmpeg if already exists).
func PreprocessVideo(src string, w, h int) (string, error) {
	tmpDir := filepath.Join(os.TempDir(), "livepaper")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	out := filepath.Join(tmpDir, fmt.Sprintf("%s_%dx%d.mp4", base, w, h))

	if _, err := os.Stat(out); err == nil {
		log.Printf("video cache: %s", out)
		return out, nil
	}

	log.Printf("transcoding %s → %dx%d ...", filepath.Base(src), w, h)
	cmd := exec.Command("ffmpeg",
		"-i", src,
		"-vf", fmt.Sprintf(
			"scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
			w, h, w, h,
		),
		"-c:v", "libx264",
		"-crf", "23",
		"-preset", "fast",
		"-movflags", "+faststart",
		"-r", "30",
		"-an",
		"-y", out,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(out)
		return "", fmt.Errorf("ffmpeg encode: %w", err)
	}

	log.Printf("transcoded: %s", out)
	return out, nil
}

// ExtractVideoFrame extracts a single frame from the middle of the video,
// scaled and center-cropped to the given dimensions. Returns image.Image.
func ExtractVideoFrame(path string, w, h int) (image.Image, error) {
	seekSec := VideoMidSec(path)

	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", seekSec),
		"-i", path,
		"-vf", fmt.Sprintf(
			"scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
			w, h, w, h,
		),
		"-frames:v", "1",
		"-f", "image2pipe",
		"-vcodec", "png",
		"-",
	)
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil, fmt.Errorf("ffmpeg frame extract: %w", err)
	}
	img, _, err := image.Decode(bytes.NewReader(out))
	return img, err
}

// VideoMidSec returns the timestamp (seconds) at the midpoint of the video.
func VideoMidSec(path string) float64 {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || f <= 0 {
		return 0
	}
	return f / 2
}

// fadeInWindow animates the desktop window from fully transparent to fully
// opaque over ~600 ms. Only effective when WS_EX_LAYERED is set (Case B).
func fadeInWindow(hwnd uintptr) {
	exStyle, _, _ := getWindowLongPtrW.Call(hwnd, gwlpExStyle)
	if exStyle&wsExLayered == 0 {
		return
	}
	const steps = 60
	const delay = 10 * time.Millisecond // 600 ms total
	for i := 0; i <= steps; i++ {
		setLayeredWindowAttributesW.Call(hwnd, 0, uintptr(255*i/steps), lwaAlpha)
		if i < steps {
			time.Sleep(delay)
		}
	}
}

// ── Internal ──────────────────────────────────────────────────────────────────

// spawnMpv runs mpv in a loop (crash-recovery). Exits cleanly when stop is
// closed (typically from stopSession → cmd.Process.Kill → cmd.Run returns).
func spawnMpv(videoPath string, hwnd uintptr, stop <-chan struct{}) {
	wid := strconv.FormatUint(uint64(hwnd), 10)
	for {
		select {
		case <-stop:
			return
		default:
		}

		cmd := exec.Command("mpv",
			"--wid="+wid,
			"--loop=inf",
			"--no-border",
			"--no-osc",
			"--no-terminal",
			"--keepaspect=no",
			"--hwdec=auto",
			videoPath,
		)

		sessionMu.Lock()
		sessionCmds = append(sessionCmds, cmd)
		sessionMu.Unlock()

		if err := cmd.Run(); err != nil {
			log.Printf("mpv exited: %v", err)
		}

		// Remove from tracking list after exit.
		sessionMu.Lock()
		for i, c := range sessionCmds {
			if c == cmd {
				sessionCmds = append(sessionCmds[:i], sessionCmds[i+1:]...)
				break
			}
		}
		sessionMu.Unlock()
	}
}
