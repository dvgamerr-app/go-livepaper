package wallpaper

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

func RunVideoWallpapers(targets []VideoTarget) error {
	for _, t := range targets {
		optimized, err := preprocessVideo(t.Path, t.W, t.H)
		if err != nil {
			return fmt.Errorf("preprocess %s: %w", t.Path, err)
		}
		hwnd, _, err := CreateDesktopWindow(t.X, t.Y, t.W, t.H)
		if err != nil {
			return fmt.Errorf("desktop window for %s: %w", t.Path, err)
		}
		go spawnMpv(optimized, hwnd)
	}
	RunMessageLoop()
	return nil
}

func preprocessVideo(src string, w, h int) (string, error) {
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

func spawnMpv(videoPath string, hwnd uintptr) {
	wid := strconv.FormatUint(uint64(hwnd), 10)
	for {
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
		if err := cmd.Run(); err != nil {
			log.Printf("mpv exited: %v — restarting", err)
		}
	}
}
