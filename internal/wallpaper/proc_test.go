//go:build windows

package wallpaper

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestConfigureBackgroundCommand_Nil(t *testing.T) {
	// Must not panic on nil input.
	ConfigureBackgroundCommand(nil)
}

func TestConfigureBackgroundCommand_NewCommand(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "echo", "hello")
	if cmd.SysProcAttr != nil {
		t.Fatal("precondition: SysProcAttr should be nil before configure")
	}

	ConfigureBackgroundCommand(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("ConfigureBackgroundCommand: SysProcAttr is still nil after call")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Error("ConfigureBackgroundCommand: HideWindow should be true")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Error("ConfigureBackgroundCommand: CREATE_NO_WINDOW flag not set")
	}
}

func TestConfigureBackgroundCommand_ExistingAttr(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "echo", "hello")
	existing := &syscall.SysProcAttr{}
	cmd.SysProcAttr = existing

	ConfigureBackgroundCommand(cmd)

	if cmd.SysProcAttr != existing {
		t.Error("ConfigureBackgroundCommand: should reuse existing SysProcAttr, not replace it")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Error("ConfigureBackgroundCommand: HideWindow should be true on reused attr")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Error("ConfigureBackgroundCommand: CREATE_NO_WINDOW flag not set on reused attr")
	}
}
