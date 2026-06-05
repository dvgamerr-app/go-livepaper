//go:build windows

package wallpaper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsVideoFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"video.mp4", true},
		{"video.MP4", true},
		{"video.mkv", true},
		{"video.avi", true},
		{"video.mov", true},
		{"video.webm", true},
		{"video.m4v", true},
		{"video.flv", true},
		{"video.gif", true},
		{"image.jpg", false},
		{"image.png", false},
		{"image.webp", false},
		{"document.pdf", false},
		{"noextension", false},
		{"/path/to/file.MP4", true},
		{"/path/to/file.AVI", true},
	}
	for _, tt := range tests {
		if got := IsVideoFile(tt.path); got != tt.want {
			t.Errorf("IsVideoFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestSetExtraMpvArgs(t *testing.T) {
	// Save and restore original state.
	sessionMu.Lock()
	orig := extraMpvArgs
	sessionMu.Unlock()
	t.Cleanup(func() {
		sessionMu.Lock()
		extraMpvArgs = orig
		sessionMu.Unlock()
	})

	args := []string{"--hwdec=auto", "--d3d11-adapter=NVIDIA"}
	SetExtraMpvArgs(args)

	sessionMu.Lock()
	got := extraMpvArgs
	sessionMu.Unlock()

	if len(got) != len(args) {
		t.Fatalf("SetExtraMpvArgs: len=%d, want %d", len(got), len(args))
	}
	for i, a := range args {
		if got[i] != a {
			t.Errorf("SetExtraMpvArgs[%d] = %q, want %q", i, got[i], a)
		}
	}

	// Nil/empty slice must also be storable.
	SetExtraMpvArgs(nil)
	sessionMu.Lock()
	gotNil := extraMpvArgs
	sessionMu.Unlock()
	if gotNil != nil {
		t.Errorf("SetExtraMpvArgs(nil): got %v, want nil", gotNil)
	}
}

func TestHasActiveVideoWallpapers_Empty(t *testing.T) {
	sessionMu.Lock()
	sessionPipes = nil
	sessionMu.Unlock()

	if HasActiveVideoWallpapers() {
		t.Error("HasActiveVideoWallpapers() = true with empty pipes, want false")
	}
}

func TestHasActiveVideoWallpapers_WithPipes(t *testing.T) {
	sessionMu.Lock()
	sessionPipes = []string{`\\.\pipe\livepaper-mpv-12345`}
	sessionMu.Unlock()
	t.Cleanup(func() {
		sessionMu.Lock()
		sessionPipes = nil
		sessionMu.Unlock()
	})

	if !HasActiveVideoWallpapers() {
		t.Error("HasActiveVideoWallpapers() = false with active pipes, want true")
	}
}

func TestStopVideoWallpapers_Empty(t *testing.T) {
	// Ensure no panic when session is already empty.
	sessionMu.Lock()
	sessionStop = nil
	sessionCmds = nil
	sessionHwnds = nil
	sessionPipes = nil
	sessionMu.Unlock()

	StopVideoWallpapers() // must not panic or deadlock
}

func TestStopVideoWallpapers_ClearsState(t *testing.T) {
	stop := make(chan struct{})
	sessionMu.Lock()
	sessionStop = stop
	sessionPipes = []string{`\\.\pipe\livepaper-mpv-1`}
	sessionHwnds = nil // no actual windows to post WM_CLOSE to
	sessionMu.Unlock()

	StopVideoWallpapers()

	sessionMu.Lock()
	pipes := sessionPipes
	cmds := sessionCmds
	sessionMu.Unlock()

	if len(pipes) != 0 {
		t.Errorf("sessionPipes after Stop = %v, want empty", pipes)
	}
	if len(cmds) != 0 {
		t.Errorf("sessionCmds after Stop = %v, want empty", cmds)
	}
	// The stop channel must have been closed.
	select {
	case <-stop:
	default:
		t.Error("sessionStop channel was not closed by StopVideoWallpapers")
	}
}

func TestStopSession_WithRunningProcess(t *testing.T) {
	// Start a real long-running process so c.Process != nil when stopSession runs.
	cmd := exec.Command("ping", "-n", "30", "127.0.0.1")
	ConfigureBackgroundCommand(cmd)
	if err := cmd.Start(); err != nil {
		t.Skipf("could not start ping process: %v", err)
	}

	stop := make(chan struct{})
	sessionMu.Lock()
	sessionStop = stop
	sessionCmds = []*exec.Cmd{cmd}
	sessionHwnds = nil
	sessionPipes = nil
	sessionMu.Unlock()

	StopVideoWallpapers()

	sessionMu.Lock()
	remaining := len(sessionCmds)
	sessionMu.Unlock()

	if remaining != 0 {
		t.Errorf("sessionCmds after StopVideoWallpapers = %d, want 0", remaining)
	}
	// cmd.Wait cleans up; ignore error since we killed it.
	_ = cmd.Wait()
}

// TestStopSession_WithDummyHwnd exercises the postMessageW loop in stopSession.
// Posting WM_CLOSE to an invalid hwnd (like 1) is a no-op that fails silently;
// it must not panic.
func TestStopSession_WithDummyHwnd(t *testing.T) {
	sessionMu.Lock()
	sessionStop = nil
	sessionCmds = nil
	sessionPipes = nil
	sessionHwnds = []uintptr{1} // invalid hwnd – PostMessage returns FALSE, ignored
	sessionMu.Unlock()

	StopVideoWallpapers() // must not panic
}

func TestVideoMidSec_NoTool(t *testing.T) {
	// Pass a path that is guaranteed not to be a real video file.
	// ffprobe will fail, so VideoMidSec must return 0 gracefully.
	got := VideoMidSec(filepath.Join(t.TempDir(), "nonexistent.mp4"))
	if got != 0 {
		t.Errorf("VideoMidSec (no ffprobe / bad file) = %f, want 0", got)
	}
}

func TestVideoMidSec_ZeroDuration(t *testing.T) {
	// Create a text file (ffprobe will report 0 or fail to parse duration).
	f, err := os.CreateTemp(t.TempDir(), "notavideo*.mp4")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("fake video data")
	f.Close()

	got := VideoMidSec(f.Name())
	if got != 0 {
		t.Errorf("VideoMidSec (invalid media) = %f, want 0", got)
	}
}

func TestPreprocessVideo_CacheHit(t *testing.T) {
	// Pre-create the expected output file so PreprocessVideo skips ffmpeg.
	tmpDir := filepath.Join(os.TempDir(), "livepaper")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatal(err)
	}

	const (
		w    = 1920
		h    = 1080
		base = "cachetest"
	)
	cachedPath := filepath.Join(tmpDir, fmt.Sprintf("%s_%dx%d.mp4", base, w, h))
	f, err := os.Create(cachedPath)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(cachedPath) })

	// src just needs the right base name; the file itself need not exist
	// because PreprocessVideo checks the cache output path first.
	srcPath := filepath.Join(t.TempDir(), base+".mp4")

	out, err := PreprocessVideo(srcPath, w, h)
	if err != nil {
		t.Fatalf("PreprocessVideo() error = %v", err)
	}
	if out != cachedPath {
		t.Errorf("PreprocessVideo() = %q, want %q", out, cachedPath)
	}
}

func TestPreprocessVideo_FFmpegError(t *testing.T) {
	// Source file does not exist → ffmpeg will fail → must return an error.
	// (The cache file also must not exist so we don't hit the cache branch.)
	src := filepath.Join(t.TempDir(), "definitely_missing.mp4")
	_, err := PreprocessVideo(src, 640, 360)
	if err == nil {
		t.Error("PreprocessVideo() expected error for missing source")
	}
	if !strings.Contains(err.Error(), "ffmpeg") {
		t.Errorf("error should mention ffmpeg, got: %v", err)
	}
}
