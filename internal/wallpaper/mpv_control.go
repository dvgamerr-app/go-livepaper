//go:build windows

package wallpaper

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Microsoft/go-winio"
)

// mpvPipeName returns the named-pipe path used for an mpv instance bound to a
// given desktop window handle.
func mpvPipeName(hwnd uintptr) string {
	return `\\.\pipe\livepaper-mpv-` + strconv.FormatUint(uint64(hwnd), 10)
}

// sendMpvAll dispatches a single JSON IPC command line to every mpv instance in
// the current session. Failures are ignored — an instance may still be starting
// up or already gone.
func sendMpvAll(command string) {
	sessionMu.Lock()
	pipes := make([]string, len(sessionPipes))
	copy(pipes, sessionPipes)
	sessionMu.Unlock()

	for _, p := range pipes {
		go sendMpvOne(p, command)
	}
}

func sendMpvOne(pipe, command string) {
	timeout := 300 * time.Millisecond
	conn, err := winio.DialPipe(pipe, &timeout)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	_, _ = conn.Write([]byte(command + "\n"))
}

// PauseVideoWallpapers pauses or resumes all running video wallpapers.
func PauseVideoWallpapers(paused bool) {
	sendMpvAll(fmt.Sprintf(`{"command":["set_property","pause",%t]}`, paused))
}

// SetVideoSpeed sets the playback speed multiplier on all running video
// wallpapers (1.0 = normal). Used by the "reduce motion on focus" setting.
func SetVideoSpeed(speed float64) {
	if speed <= 0 {
		speed = 1
	}
	sendMpvAll(fmt.Sprintf(`{"command":["set_property","speed",%s]}`,
		strconv.FormatFloat(speed, 'f', 2, 64)))
}

// HasActiveVideoWallpapers reports whether any video wallpaper is currently
// running, so callers can skip IPC work when nothing is playing.
func HasActiveVideoWallpapers() bool {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	return len(sessionPipes) > 0
}
