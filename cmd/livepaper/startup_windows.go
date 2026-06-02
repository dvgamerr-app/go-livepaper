//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "LivePaper"
)

// applyLaunchAtLogin adds or removes the HKCU Run registry value that makes
// Windows start LivePaper at user login.
func applyLaunchAtLogin(enable bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if enable {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		return k.SetStringValue(runValueName, `"`+exe+`"`)
	}

	// Best-effort delete — a missing value is not an error for us.
	_ = k.DeleteValue(runValueName)
	return nil
}

// isLaunchAtLogin reports whether the Run registry value is currently present.
func isLaunchAtLogin() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runValueName)
	return err == nil
}

// syncLaunchAtLogin reconciles the registry with the saved setting at startup.
func syncLaunchAtLogin(want bool) {
	if isLaunchAtLogin() != want {
		_ = applyLaunchAtLogin(want)
	}
}
