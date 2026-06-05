//go:build windows

package main

import (
	"testing"
)

func TestApplyLaunchAtLogin_Enable(t *testing.T) {
	// Write the registry value and verify the helper reads it back.
	if err := applyLaunchAtLogin(true); err != nil {
		t.Fatalf("applyLaunchAtLogin(true) error = %v", err)
	}
	if !isLaunchAtLogin() {
		t.Error("isLaunchAtLogin() = false after enabling")
	}
}

func TestApplyLaunchAtLogin_Disable(t *testing.T) {
	// Enable first, then disable.
	_ = applyLaunchAtLogin(true)
	if err := applyLaunchAtLogin(false); err != nil {
		t.Fatalf("applyLaunchAtLogin(false) error = %v", err)
	}
	if isLaunchAtLogin() {
		t.Error("isLaunchAtLogin() = true after disabling")
	}
}

func TestSyncLaunchAtLogin_AlreadyCorrect(t *testing.T) {
	// Get current state and sync to it — nothing should change, no panic.
	current := isLaunchAtLogin()
	syncLaunchAtLogin(current) // no-op: registry already matches
}

func TestSyncLaunchAtLogin_Toggle(t *testing.T) {
	// Deliberately mismatch registry vs desired; syncLaunchAtLogin must fix it.
	_ = applyLaunchAtLogin(false)
	syncLaunchAtLogin(true) // should enable it
	if !isLaunchAtLogin() {
		t.Error("syncLaunchAtLogin(true): isLaunchAtLogin() = false")
	}

	syncLaunchAtLogin(false) // should disable it
	if isLaunchAtLogin() {
		t.Error("syncLaunchAtLogin(false): isLaunchAtLogin() = true")
	}
}

func TestCheckDependencies(t *testing.T) {
	svc := &AppService{}
	deps := svc.CheckDependencies()

	for _, key := range []string{"ffmpeg", "ffprobe", "mpv"} {
		if _, ok := deps[key]; !ok {
			t.Errorf("CheckDependencies() missing key %q", key)
		}
	}
	// The values are bool; we don't assert they are true since CI may not have
	// the tools installed.
}

func TestGetVideoDurationUs_InvalidFile(t *testing.T) {
	// Non-existent file → ffprobe fails → returns 0.
	if got := getVideoDurationUs("/nonexistent/file.mp4"); got != 0 {
		t.Errorf("getVideoDurationUs (bad file) = %d, want 0", got)
	}
}
