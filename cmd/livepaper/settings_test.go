//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// resetSettings restores the global currentSettings to defaults and is called
// as a t.Cleanup so each test starts with a known state.
func resetSettings(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		settingsMu.Lock()
		currentSettings = defaultSettings()
		settingsMu.Unlock()
	})
}

// ---------- defaultSettings ----------

func TestDefaultSettings(t *testing.T) {
	s := defaultSettings()

	if !s.GPUAcceleration {
		t.Error("GPUAcceleration should be true")
	}
	if s.VRAMCapMB != 256 {
		t.Errorf("VRAMCapMB = %d, want 256", s.VRAMCapMB)
	}
	if s.WindowTheme != "mica" {
		t.Errorf("WindowTheme = %q, want \"mica\"", s.WindowTheme)
	}
	if !s.PauseOnGame {
		t.Error("PauseOnGame should be true")
	}
	if !s.PauseOnBattery {
		t.Error("PauseOnBattery should be true")
	}
	if s.LaunchAtLogin {
		t.Error("LaunchAtLogin should be false")
	}
	if !s.StartMinimized {
		t.Error("StartMinimized should be true")
	}
	if !s.RestoreLastPlaylist {
		t.Error("RestoreLastPlaylist should be true")
	}
	for _, key := range []string{HotkeyNext, HotkeyPrev, HotkeyPlayPause, HotkeyOpen} {
		if s.Hotkeys[key] == "" {
			t.Errorf("default hotkey %q is empty", key)
		}
	}
}

// ---------- normalize ----------

func TestNormalize_ValidWindowThemes(t *testing.T) {
	for _, theme := range []string{"mica", "acrylic", "solid"} {
		s := defaultSettings()
		s.WindowTheme = theme
		s.normalize()
		if s.WindowTheme != theme {
			t.Errorf("normalize: valid theme %q changed to %q", theme, s.WindowTheme)
		}
	}
}

func TestNormalize_InvalidWindowTheme(t *testing.T) {
	s := defaultSettings()
	s.WindowTheme = "glass"
	s.normalize()
	if s.WindowTheme != "mica" {
		t.Errorf("normalize: invalid theme → %q, want \"mica\"", s.WindowTheme)
	}
}

func TestNormalize_VRAMCapMBFloor(t *testing.T) {
	s := defaultSettings()
	s.VRAMCapMB = 10
	s.normalize()
	if s.VRAMCapMB != 64 {
		t.Errorf("normalize: VRAMCapMB below floor → %d, want 64", s.VRAMCapMB)
	}
}

func TestNormalize_VRAMCapMBCeiling(t *testing.T) {
	s := defaultSettings()
	s.VRAMCapMB = 9999
	s.normalize()
	if s.VRAMCapMB != 1024 {
		t.Errorf("normalize: VRAMCapMB above ceiling → %d, want 1024", s.VRAMCapMB)
	}
}

func TestNormalize_VRAMCapMBInRange(t *testing.T) {
	s := defaultSettings()
	s.VRAMCapMB = 512
	s.normalize()
	if s.VRAMCapMB != 512 {
		t.Errorf("normalize: VRAMCapMB in range → %d, want 512", s.VRAMCapMB)
	}
}

func TestNormalize_NilHotkeys(t *testing.T) {
	s := defaultSettings()
	s.Hotkeys = nil
	s.normalize()
	for _, key := range []string{HotkeyNext, HotkeyPrev, HotkeyPlayPause, HotkeyOpen} {
		if s.Hotkeys[key] == "" {
			t.Errorf("normalize: nil hotkeys → key %q still empty", key)
		}
	}
}

func TestNormalize_MissingHotkey(t *testing.T) {
	s := defaultSettings()
	delete(s.Hotkeys, HotkeyNext)
	s.normalize()
	if s.Hotkeys[HotkeyNext] == "" {
		t.Errorf("normalize: missing hotkey %q not restored", HotkeyNext)
	}
}

func TestNormalize_ExistingHotkeyPreserved(t *testing.T) {
	s := defaultSettings()
	custom := "Ctrl + Alt + N"
	s.Hotkeys[HotkeyNext] = custom
	s.normalize()
	if s.Hotkeys[HotkeyNext] != custom {
		t.Errorf("normalize: custom hotkey changed from %q to %q", custom, s.Hotkeys[HotkeyNext])
	}
}

// ---------- settingsPath ----------

func TestSettingsPath_WithAPPDATA(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)

	p := settingsPath()
	want := filepath.Join(dir, "livepaper", "settings.json")
	if p != want {
		t.Errorf("settingsPath() = %q, want %q", p, want)
	}
}

func TestSettingsPath_WithoutAPPDATA(t *testing.T) {
	t.Setenv("APPDATA", "")
	p := settingsPath()
	if p == "" {
		t.Error("settingsPath() returned empty when APPDATA is unset")
	}
	// Should fall back to os.TempDir().
	if filepath.Base(filepath.Dir(p)) != "livepaper" {
		t.Errorf("settingsPath() base dir = %q, want \"livepaper\"", filepath.Dir(p))
	}
}

// ---------- saveSettingsToDisk / loadSettings ----------

func TestSaveAndLoadSettings(t *testing.T) {
	resetSettings(t)
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)

	want := defaultSettings()
	want.GPUAdapter = "adapter-a"
	want.VRAMCapMB = 512
	want.WindowTheme = "acrylic"

	if err := saveSettingsToDisk(want); err != nil {
		t.Fatalf("saveSettingsToDisk() error = %v", err)
	}

	got := loadSettings()
	if got.GPUAdapter != want.GPUAdapter {
		t.Errorf("GPUAdapter = %q, want %q", got.GPUAdapter, want.GPUAdapter)
	}
	if got.VRAMCapMB != want.VRAMCapMB {
		t.Errorf("VRAMCapMB = %d, want %d", got.VRAMCapMB, want.VRAMCapMB)
	}
	if got.WindowTheme != want.WindowTheme {
		t.Errorf("WindowTheme = %q, want %q", got.WindowTheme, want.WindowTheme)
	}

	// Replacing an existing settings file must remain supported by the atomic
	// write path.
	want.GPUAdapter = "adapter-b"
	if err := saveSettingsToDisk(want); err != nil {
		t.Fatalf("saveSettingsToDisk() replacing file error = %v", err)
	}
	if got := loadSettings(); got.GPUAdapter != "adapter-b" {
		t.Errorf("GPUAdapter after replacing file = %q, want %q", got.GPUAdapter, "adapter-b")
	}
}

func TestLoadSettings_MissingFile(t *testing.T) {
	resetSettings(t)
	// Point APPDATA at an empty temp dir so no settings file exists.
	t.Setenv("APPDATA", t.TempDir())

	s := loadSettings()
	// Must return defaults when the file is missing.
	if s.GPUAdapter != "" {
		t.Errorf("loadSettings() missing file: GPUAdapter = %q, want empty", s.GPUAdapter)
	}
}

func TestLoadSettings_CorruptFile(t *testing.T) {
	resetSettings(t)
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)

	// Write invalid JSON.
	p := filepath.Join(dir, "livepaper", "settings.json")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	s := loadSettings()
	// Unmarshal failure → defaults used.
	if s.GPUAdapter != "" {
		t.Errorf("loadSettings() corrupt file: GPUAdapter = %q, want empty", s.GPUAdapter)
	}
}

func TestLoadSettings_PartialJSON(t *testing.T) {
	resetSettings(t)
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)

	// JSON with only some fields — missing fields get defaults.
	p := filepath.Join(dir, "livepaper", "settings.json")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	partial, _ := json.Marshal(map[string]any{"gpuAdapter": "adapter-c"})
	if err := os.WriteFile(p, partial, 0644); err != nil {
		t.Fatal(err)
	}

	s := loadSettings()
	if s.GPUAdapter != "adapter-c" {
		t.Errorf("GPUAdapter = %q, want \"ja-JP\"", s.GPUAdapter)
	}
	// VRAMCapMB was not set in JSON, should get the default 256.
	if s.VRAMCapMB != 256 {
		t.Errorf("VRAMCapMB = %d, want default 256", s.VRAMCapMB)
	}
}

func TestLoadSettings_IgnoresRemovedTelemetryKey(t *testing.T) {
	resetSettings(t)
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)

	// Settings written by older versions still carry the removed "telemetry"
	// key; it must be ignored instead of discarding the rest of the file.
	p := filepath.Join(dir, "livepaper", "settings.json")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	legacy, _ := json.Marshal(map[string]any{"gpuAdapter": "adapter-a", "telemetry": true, "vramCapMB": 512})
	if err := os.WriteFile(p, legacy, 0644); err != nil {
		t.Fatal(err)
	}

	s := loadSettings()
	if s.GPUAdapter != "adapter-a" {
		t.Errorf("GPUAdapter = %q, want \"th-TH\"", s.GPUAdapter)
	}
	if s.VRAMCapMB != 512 {
		t.Errorf("VRAMCapMB = %d, want 512", s.VRAMCapMB)
	}

	if err := saveSettingsToDisk(s); err != nil {
		t.Fatalf("saveSettingsToDisk() error = %v", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if _, ok := saved["telemetry"]; ok {
		t.Error("saved settings still contain the removed \"telemetry\" key")
	}
}

// ---------- getSettings ----------

func TestGetSettings(t *testing.T) {
	resetSettings(t)
	settingsMu.Lock()
	currentSettings = defaultSettings()
	currentSettings.GPUAdapter = "adapter-b"
	settingsMu.Unlock()

	got := getSettings()
	if got.GPUAdapter != "adapter-b" {
		t.Errorf("getSettings().GPUAdapter = %q, want \"de-DE\"", got.GPUAdapter)
	}
}

func TestGetSettings_ReturnsIndependentHotkeys(t *testing.T) {
	resetSettings(t)
	settingsMu.Lock()
	currentSettings = defaultSettings()
	settingsMu.Unlock()

	got := getSettings()
	got.Hotkeys[HotkeyNext] = "Alt + N"

	if current := getSettings().Hotkeys[HotkeyNext]; current != "Ctrl + Shift + >" {
		t.Errorf("mutating returned hotkeys changed current settings to %q", current)
	}
}

func TestSaveSettings_FailedWritePreservesCurrentSettings(t *testing.T) {
	resetSettings(t)
	blockedRoot := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockedRoot, []byte("block directory creation"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", blockedRoot)

	next := defaultSettings()
	next.GPUAdapter = "adapter-a"
	if err := (&AppService{}).SaveSettings(next); err == nil {
		t.Fatal("SaveSettings() error = nil, want persistence error")
	}

	if got := getSettings().GPUAdapter; got != "" {
		t.Errorf("current GPUAdapter after failed save = %q, want %q", got, "")
	}
}

func TestSaveSettings_ClonesInputHotkeys(t *testing.T) {
	resetSettings(t)
	t.Setenv("APPDATA", t.TempDir())

	next := defaultSettings()
	next.GPUAdapter = "adapter-a"
	if err := (&AppService{}).SaveSettings(next); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}

	next.Hotkeys[HotkeyNext] = "Alt + N"
	if got := getSettings().Hotkeys[HotkeyNext]; got != "Ctrl + Shift + >" {
		t.Errorf("mutating input hotkeys changed current settings to %q", got)
	}
}

// ---------- hotkeysChanged ----------

func TestHotkeysChanged(t *testing.T) {
	a := map[string]string{"next": "Ctrl+N", "prev": "Ctrl+P"}
	b := map[string]string{"next": "Ctrl+N", "prev": "Ctrl+P"}

	if hotkeysChanged(a, b) {
		t.Error("hotkeysChanged(identical): got true, want false")
	}

	b["next"] = "Alt+N"
	if !hotkeysChanged(a, b) {
		t.Error("hotkeysChanged(value diff): got false, want true")
	}

	c := map[string]string{"next": "Ctrl+N"}
	if !hotkeysChanged(a, c) {
		t.Error("hotkeysChanged(length diff): got false, want true")
	}

	if !hotkeysChanged(a, nil) {
		t.Error("hotkeysChanged(nil b): got false, want true")
	}
}
