package wallpaper

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
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
	sessionCmds  []*exec.Cmd   // running mpv processes
	sessionHwnds []uintptr     // desktop windows to destroy on stop
	sessionPipes []string      // mpv IPC pipe names for playback control
	extraMpvArgs []string      // extra mpv flags from settings (GPU adapter, cache cap)
)

// SetExtraMpvArgs sets additional mpv command-line flags applied to every video
// wallpaper spawned afterwards (e.g. GPU adapter selection, demuxer cache cap).
func SetExtraMpvArgs(args []string) {
	sessionMu.Lock()
	extraMpvArgs = args
	sessionMu.Unlock()
}

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
	sessionPipes = nil
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
		if IsPlayableVideo(out) {
			log.Printf("video cache: %s", out)
			return out, nil
		}
		// Stale cache: an earlier encode was interrupted/killed and left a
		// truncated (no moov atom) or zero-byte file. Reusing it makes both
		// ffmpeg frame extraction and mpv playback fail. Discard and re-encode.
		log.Printf("video cache invalid, re-encoding: %s", out)
		os.Remove(out)
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
	ConfigureBackgroundCommand(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(out)
		return "", fmt.Errorf("ffmpeg encode: %w", err)
	}

	log.Printf("transcoded: %s", out)
	return out, nil
}

// IsPlayableVideo reports whether path is a decodable video file: it must exist,
// be non-empty, and expose a positive container duration via ffprobe. A
// truncated or interrupted encode (missing moov atom) or a zero-byte file fails
// this check, letting callers re-encode instead of handing a corrupt file to
// mpv/ffmpeg (which would otherwise fail with ffmpeg exit 0xfffffffe / mpv exit 2).
//
// If ffprobe itself cannot be found, the probe is skipped and the file is assumed
// playable (only the size guard applies) so a tool-less setup does not wrongly
// reject otherwise-valid cached files.
func IsPlayableVideo(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() == 0 {
		return false
	}
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	ConfigureBackgroundCommand(cmd)
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return true // ffprobe missing — can't prove it's bad, don't block
		}
		return false // non-zero exit = invalid/corrupt input
	}
	d, perr := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	return perr == nil && d > 0
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
	ConfigureBackgroundCommand(cmd)
	// Capture stderr so a failure carries ffmpeg's reason (e.g. "moov atom not
	// found") instead of a bare exit status that hides the root cause.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil && len(out) == 0 {
		err = errors.New("no frame produced")
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if i := strings.LastIndexByte(msg, '\n'); i >= 0 {
			msg = strings.TrimSpace(msg[i+1:]) // keep only ffmpeg's final line
		}
		if msg != "" {
			return nil, fmt.Errorf("ffmpeg frame extract: %w (%s)", err, msg)
		}
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
	ConfigureBackgroundCommand(cmd)
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

// classifyMpvExit interprets an mpv process exit error against mpv's documented
// exit codes. `recoverable` reports whether restarting mpv with the same file
// and arguments could plausibly succeed.
//
//	1 = mpv failed to initialize (invalid option / no usable video output)
//	2 = the file could not be played (corrupt, unsupported, or missing)
//
// Both are permanent for an unchanged file+args, so neither is recoverable —
// retrying would just spin. (The previous code mislabeled 2 as "bad arguments".)
func classifyMpvExit(err error) (code int, msg string, recoverable bool) {
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return -1, "mpv did not start (binary missing or spawn failed)", false
	}
	switch ee.ExitCode() {
	case 1:
		return 1, "mpv failed to initialize — invalid option or no usable video output", false
	case 2:
		return 2, "mpv could not play the file — corrupt, unsupported, or missing", false
	default:
		return ee.ExitCode(), "mpv exited unexpectedly", true
	}
}

// spawnMpv runs mpv in a loop (crash-recovery). Exits cleanly when stop is
// closed (typically from stopSession → cmd.Process.Kill → cmd.Run returns).
func spawnMpv(videoPath string, hwnd uintptr, stop <-chan struct{}) {
	wid := strconv.FormatUint(uint64(hwnd), 10)
	pipe := mpvPipeName(hwnd)

	sessionMu.Lock()
	sessionPipes = append(sessionPipes, pipe)
	extra := append([]string(nil), extraMpvArgs...)
	sessionMu.Unlock()

	// mpv runs with --no-terminal and therefore prints nothing to stderr, so a
	// bad file would otherwise loop forever exiting with status 2 and no reason.
	// Validate once up front and bail with a clear, actionable message instead.
	if !IsPlayableVideo(videoPath) {
		log.Printf("mpv: skipping %q — not a playable video (corrupt, zero-byte, or missing); re-encode required", videoPath)
		return
	}

	for {
		select {
		case <-stop:
			return
		default:
		}

		mpvArgs := []string{
			"--wid=" + wid,
			"--loop=inf",
			"--no-border",
			"--no-osc",
			"--no-terminal",
			"--keepaspect=no",
			"--hwdec=auto",
			"--input-ipc-server=" + pipe,
		}
		mpvArgs = append(mpvArgs, extra...)
		mpvArgs = append(mpvArgs, videoPath)
		cmd := exec.Command("mpv", mpvArgs...)
		ConfigureBackgroundCommand(cmd)

		// Capture mpv stderr so we can log the reason for any unexpected exit.
		var stderrBuf bytes.Buffer
		cmd.Stderr = &stderrBuf

		sessionMu.Lock()
		sessionCmds = append(sessionCmds, cmd)
		sessionMu.Unlock()

		runErr := cmd.Run()

		// Remove from tracking list as soon as the process has exited.
		sessionMu.Lock()
		for i, c := range sessionCmds {
			if c == cmd {
				sessionCmds = append(sessionCmds[:i], sessionCmds[i+1:]...)
				break
			}
		}
		sessionMu.Unlock()

		// A stop request kills mpv (exit status 1 on Windows). Check stop before
		// classifying so a normal teardown does not log a spurious failure.
		select {
		case <-stop:
			return
		default:
		}

		if runErr == nil {
			return // clean exit (e.g. mpv quit) — nothing to recover
		}

		code, msg, recoverable := classifyMpvExit(runErr)
		if stderr := strings.TrimSpace(stderrBuf.String()); stderr != "" {
			log.Printf("mpv exited (code %d): %s\n%s", code, msg, stderr)
		} else {
			log.Printf("mpv exited (code %d): %s", code, msg)
		}
		if !recoverable {
			return
		}

		// Brief pause before restarting to avoid a tight CPU-spinning loop on
		// repeated transient crashes (e.g. renderer hiccup).
		select {
		case <-stop:
			return
		case <-time.After(time.Second):
		}
	}
}
