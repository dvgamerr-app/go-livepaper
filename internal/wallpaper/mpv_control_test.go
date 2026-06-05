//go:build windows

package wallpaper

import (
	"fmt"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
)

func TestMpvPipeName(t *testing.T) {
	tests := []struct {
		hwnd uintptr
		want string
	}{
		{0, `\\.\pipe\livepaper-mpv-0`},
		{12345, `\\.\pipe\livepaper-mpv-12345`},
		{0xDEADBEEF, fmt.Sprintf(`\\.\pipe\livepaper-mpv-%d`, uintptr(0xDEADBEEF))},
	}
	for _, tt := range tests {
		if got := mpvPipeName(tt.hwnd); got != tt.want {
			t.Errorf("mpvPipeName(0x%X) = %q, want %q", tt.hwnd, got, tt.want)
		}
	}
}

func TestHasActiveVideoWallpapers(t *testing.T) {
	// Already tested in video_test.go; provide a brief sanity check here too.
	sessionMu.Lock()
	sessionPipes = nil
	sessionMu.Unlock()
	if HasActiveVideoWallpapers() {
		t.Error("HasActiveVideoWallpapers() = true with no pipes")
	}

	sessionMu.Lock()
	sessionPipes = []string{`\\.\pipe\livepaper-mpv-9999`}
	sessionMu.Unlock()
	t.Cleanup(func() {
		sessionMu.Lock()
		sessionPipes = nil
		sessionMu.Unlock()
	})
	if !HasActiveVideoWallpapers() {
		t.Error("HasActiveVideoWallpapers() = false with one pipe")
	}
}

func TestPauseVideoWallpapers_NoPipes(t *testing.T) {
	sessionMu.Lock()
	sessionPipes = nil
	sessionMu.Unlock()

	// With no active pipes, sendMpvAll is a no-op — must not panic.
	PauseVideoWallpapers(true)
	PauseVideoWallpapers(false)
}

func TestSetVideoSpeed_Normal(t *testing.T) {
	sessionMu.Lock()
	sessionPipes = nil
	sessionMu.Unlock()

	// With no active pipes, sendMpvAll is a no-op — must not panic.
	SetVideoSpeed(0.5)
	SetVideoSpeed(1.0)
	SetVideoSpeed(2.0)
}

func TestSetVideoSpeed_ZeroOrNegative(t *testing.T) {
	// speed ≤ 0 should be clamped to 1 before the command is sent.
	// We can't inspect the sent payload directly without a real pipe, so we
	// just verify the call does not panic and modifies no global state.
	sessionMu.Lock()
	sessionPipes = nil
	sessionMu.Unlock()

	SetVideoSpeed(0)
	SetVideoSpeed(-1)
}

// TestSendMpvOne_ConnectsAndWrites spins up a real named pipe server, calls
// sendMpvOne, and checks that the data was received. This exercises the
// successful code path inside sendMpvOne (conn.Write).
func TestSendMpvOne_ConnectsAndWrites(t *testing.T) {
	pipe := `\\.\pipe\livepaper-test-sendone`
	received := make(chan string, 1)

	// Start a pipe server in a goroutine.
	ln, err := winio.ListenPipe(pipe, nil)
	if err != nil {
		t.Skipf("could not create test pipe: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 256)
		n, _ := conn.Read(buf)
		received <- string(buf[:n])
	}()

	sendMpvOne(pipe, `{"command":["get_version"]}`)

	select {
	case msg := <-received:
		if msg != `{"command":["get_version"]}`+"\n" {
			t.Errorf("received %q, want command+newline", msg)
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for pipe message")
	}
}

func TestSendMpvAll_CopiesPipes(t *testing.T) {
	// Verify that sendMpvAll reads a snapshot of sessionPipes under the lock
	// rather than holding the lock during I/O. We inject a pipe that does not
	// exist so sendMpvOne will fail silently — no panic expected.
	sessionMu.Lock()
	sessionPipes = []string{`\\.\pipe\livepaper-test-nonexistent`}
	sessionMu.Unlock()
	t.Cleanup(func() {
		sessionMu.Lock()
		sessionPipes = nil
		sessionMu.Unlock()
	})

	// Fire-and-forget: sendMpvAll spawns goroutines. The goroutines fail
	// connecting to the nonexistent pipe and exit silently. No assertion needed
	// beyond "no panic".
	sendMpvAll(`{"command":["get_version"]}`)
}
