package main

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
}

func isVideoFile(path string) bool {
	return videoExts[strings.ToLower(filepath.Ext(path))]
}

type videoTarget struct {
	path    string
	x, y   int
	w, h   int
}

// runVideoWallpapers pre-processes each video, embeds one mpv per monitor,
// then blocks on a single Win32 message loop shared by all windows.
func runVideoWallpapers(targets []videoTarget) error {
	for _, t := range targets {
		optimized, err := preprocessVideo(t.path, t.w, t.h)
		if err != nil {
			return fmt.Errorf("preprocess %s: %w", t.path, err)
		}
		hwnd, _, err := createDesktopWindow(t.x, t.y, t.w, t.h)
		if err != nil {
			return fmt.Errorf("desktop window for %s: %w", t.path, err)
		}
		go spawnMpv(optimized, hwnd)
	}
	runMessageLoop()
	return nil
}

// preprocessVideo transcodes src to an h264 MP4 scaled and cropped to w×h,
// capped at 30 fps, with audio stripped. The result is cached in
// %TEMP%\livepaper\ by {name}_{w}x{h}.mp4 — re-runs skip the encode.
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
		"-crf", "23",       // quality: lower = bigger/better (18–28 range)
		"-preset", "fast",  // encode speed vs compression tradeoff
		"-movflags", "+faststart", // moov atom at front for instant seek
		"-r", "30",         // cap frame rate
		"-an",              // no audio needed for wallpaper
		"-y", out,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(out) // remove partial output on failure
		return "", fmt.Errorf("ffmpeg encode: %w", err)
	}

	log.Printf("transcoded: %s", out)
	return out, nil
}

// spawnMpv launches mpv with --wid so it renders directly into hwnd using
// hardware-accelerated decoding. Restarts automatically if mpv exits.
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
