//go:build windows

package wallpaper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// hasTool reports whether a CLI tool is on PATH.
func hasTool(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// makeTestVideo encodes a tiny valid mp4 at path using ffmpeg's lavfi source.
func makeTestVideo(t *testing.T, path string, w, h int) {
	t.Helper()
	cmd := exec.Command("ffmpeg",
		"-v", "error",
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc=duration=1:size=%dx%d:rate=10", w, h),
		"-c:v", "libx264", "-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		"-y", path,
	)
	ConfigureBackgroundCommand(cmd)
	if err := cmd.Run(); err != nil {
		t.Skipf("could not create test video with ffmpeg: %v", err)
	}
}

func TestIsPlayableVideo_BadFiles(t *testing.T) {
	if !hasTool("ffprobe") {
		t.Skip("ffprobe not available")
	}
	dir := t.TempDir()

	if IsPlayableVideo(filepath.Join(dir, "missing.mp4")) {
		t.Error("IsPlayableVideo(missing) = true, want false")
	}

	empty := filepath.Join(dir, "empty.mp4")
	if err := os.WriteFile(empty, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if IsPlayableVideo(empty) {
		t.Error("IsPlayableVideo(empty) = true, want false")
	}

	// Truncated/corrupt mp4 — no moov atom. This is exactly the stale-cache file
	// that previously caused ffmpeg exit 0xfffffffe and mpv exit 2.
	corrupt := filepath.Join(dir, "corrupt.mp4")
	if err := os.WriteFile(corrupt, []byte("not a real moov mp4 payload"), 0644); err != nil {
		t.Fatal(err)
	}
	if IsPlayableVideo(corrupt) {
		t.Error("IsPlayableVideo(corrupt) = true, want false")
	}
}

func TestIsPlayableVideo_Good(t *testing.T) {
	if !hasTool("ffmpeg") || !hasTool("ffprobe") {
		t.Skip("ffmpeg/ffprobe not available")
	}
	good := filepath.Join(t.TempDir(), "good.mp4")
	makeTestVideo(t, good, 320, 240)
	if !IsPlayableVideo(good) {
		t.Error("IsPlayableVideo(freshly encoded mp4) = false, want true")
	}
}

func TestExtractVideoFrame_ErrorCarriesReason(t *testing.T) {
	if !hasTool("ffmpeg") {
		t.Skip("ffmpeg not available")
	}
	_, err := ExtractVideoFrame(filepath.Join(t.TempDir(), "missing.mp4"), 320, 240)
	if err == nil {
		t.Fatal("ExtractVideoFrame(missing) expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ffmpeg frame extract") {
		t.Errorf("error should mention 'ffmpeg frame extract', got: %v", err)
	}
}

func TestClassifyMpvExit(t *testing.T) {
	// Run real processes that exit with specific codes so we get genuine
	// *exec.ExitError values, then assert classifyMpvExit's interpretation.
	run := func(code int) error {
		cmd := exec.Command("cmd", "/c", "exit", strconv.Itoa(code))
		ConfigureBackgroundCommand(cmd)
		return cmd.Run()
	}

	// Exit 1 = init/bad-option, exit 2 = unplayable file: both non-recoverable.
	for _, code := range []int{1, 2} {
		gotCode, _, recoverable := classifyMpvExit(run(code))
		if gotCode != code {
			t.Errorf("classifyMpvExit(exit %d): code = %d, want %d", code, gotCode, code)
		}
		if recoverable {
			t.Errorf("classifyMpvExit(exit %d): recoverable = true, want false", code)
		}
	}

	// An unexpected code (e.g. 3) is treated as a transient crash → recoverable.
	if _, _, recoverable := classifyMpvExit(run(3)); !recoverable {
		t.Error("classifyMpvExit(exit 3): recoverable = false, want true (retry transient crash)")
	}

	// A non-ExitError (spawn failure) is not recoverable.
	spawnErr := exec.Command("definitely-not-a-real-binary-xyz").Run()
	if _, _, recoverable := classifyMpvExit(spawnErr); recoverable {
		t.Error("classifyMpvExit(spawn failure): recoverable = true, want false")
	}
}

func TestPreprocessVideo_ValidCacheHit(t *testing.T) {
	if !hasTool("ffmpeg") || !hasTool("ffprobe") {
		t.Skip("ffmpeg/ffprobe not available")
	}
	tmpDir := filepath.Join(os.TempDir(), "livepaper")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatal(err)
	}

	const w, h, base = 320, 240, "cachetest_valid"
	// src does NOT exist: a cache miss would force an encode that fails, so a
	// successful no-error return proves the valid cache was reused.
	srcPath := filepath.Join(t.TempDir(), base+".mp4")
	cachedPath := filepath.Join(tmpDir, fmt.Sprintf("%s_%dx%d.mp4", CacheKey(srcPath), w, h))
	makeTestVideo(t, cachedPath, w, h)
	t.Cleanup(func() { os.Remove(cachedPath) })

	out, err := PreprocessVideo(srcPath, w, h)
	if err != nil {
		t.Fatalf("PreprocessVideo() error = %v", err)
	}
	if out != cachedPath {
		t.Errorf("PreprocessVideo() = %q, want %q", out, cachedPath)
	}
}

func TestPreprocessVideo_StaleCacheRejected(t *testing.T) {
	if !hasTool("ffprobe") {
		t.Skip("ffprobe not available (cannot validate cache)")
	}
	tmpDir := filepath.Join(os.TempDir(), "livepaper")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatal(err)
	}

	const w, h, base = 320, 240, "cachetest_stale"
	// src missing → a re-encode (which a stale cache must trigger) fails.
	srcPath := filepath.Join(t.TempDir(), base+".mp4")
	cachedPath := filepath.Join(tmpDir, fmt.Sprintf("%s_%dx%d.mp4", CacheKey(srcPath), w, h))
	// Corrupt cache (no moov atom), as left by an interrupted encode.
	if err := os.WriteFile(cachedPath, []byte("corrupt not a video"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(cachedPath) })

	_, err := PreprocessVideo(srcPath, w, h)
	if err == nil {
		t.Fatal("PreprocessVideo() reused stale cache, expected re-encode error")
	}
	if !strings.Contains(err.Error(), "ffmpeg") {
		t.Errorf("error should mention ffmpeg (re-encode attempt), got: %v", err)
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
