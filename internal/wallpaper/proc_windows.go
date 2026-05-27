//go:build windows

package wallpaper

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// ConfigureBackgroundCommand hides the console window for child processes that
// the tray app launches in the background, such as ffmpeg, ffprobe, and mpv.
func ConfigureBackgroundCommand(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
