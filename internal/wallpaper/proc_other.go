//go:build !windows

package wallpaper

import "os/exec"

func ConfigureBackgroundCommand(cmd *exec.Cmd) {}
