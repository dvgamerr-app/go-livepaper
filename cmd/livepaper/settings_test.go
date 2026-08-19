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

	if s.Language != "en-US" {
		t.Errorf("Language = %q, want \"en-US\"", s.Language)
	}
	if !s.ShowNotifications {
		t.Error("ShowNotifications should be true")
	}
	if s.Telemetry {
		t.Error("Telemetry should be false")
	}
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

func TestNormalize_EmptyLanguage(t *testing.T) {
	s := defaultSettings()
	s.Language = ""
	s.normalize()
	if s.Language != "en-US" {
		t.Errorf("normalize: Language = %q, want \"en-US\"", s.Language)
	}
}

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
	want.Language = "th-TH"
	want.VRAMCapMB = 512
	want.WindowTheme = "acrylic"

	if err := saveSettingsToDisk(want); err != nil {
		t.Fatalf("saveSettingsToDisk() error = %v", err)
	}

	got := loadSettings()
	if got.Language != want.Language {
		t.Errorf("Language = %q, want %q", got.Language, want.Language)
	}
	if got.VRAMCapMB != want.VRAMCapMB {
		t.Errorf("VRAMCapMB = %d, want %d", got.VRAMCapMB, want.VRAMCapMB)
	}
	if got.WindowTheme != want.WindowTheme {
		t.Errorf("WindowTheme = %q, want %q", got.WindowTheme, want.WindowTheme)
	}

	// Replacing an existing settings file must remain supported by the atomic
	// write path.
	want.Language = "de-DE"
	if err := saveSettingsToDisk(want); err != nil {
		t.Fatalf("saveSettingsToDisk() replacing file error = %v", err)
	}
	if got := loadSettings(); got.Language != "de-DE" {
		t.Errorf("Language after replacing file = %q, want %q", got.Language, "de-DE")
	}
}

func TestLoadSettings_MissingFile(t *testing.T) {
	resetSettings(t)
	// Point APPDATA at an empty temp dir so no settings file exists.
	t.Setenv("APPDATA", t.TempDir())

	s := loadSettings()
	// Must return defaults when the file is missing.
	if s.Language != "en-US" {
		t.Errorf("loadSettings() missing file: Language = %q, want \"en-US\"", s.Language)
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
	if s.Language != "en-US" {
		t.Errorf("loadSettings() corrupt file: Language = %q, want \"en-US\"", s.Language)
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
	partial, _ := json.Marshal(map[string]any{"language": "ja-JP"})
	if err := os.WriteFile(p, partial, 0644); err != nil {
		t.Fatal(err)
	}

	s := loadSettings()
	if s.Language != "ja-JP" {
		t.Errorf("Language = %q, want \"ja-JP\"", s.Language)
	}
	// VRAMCapMB was not set in JSON, should get the default 256.
	if s.VRAMCapMB != 256 {
		t.Errorf("VRAMCapMB = %d, want default 256", s.VRAMCapMB)
	}
}

// ---------- getSettings ----------

func TestGetSettings(t *testing.T) {
	resetSettings(t)
	settingsMu.Lock()
	currentSettings = defaultSettings()
	currentSettings.Language = "de-DE"
	settingsMu.Unlock()

	got := getSettings()
	if got.Language != "de-DE" {
		t.Errorf("getSettings().Language = %q, want \"de-DE\"", got.Language)
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
	next.Language = "th-TH"
	if err := (&AppService{}).SaveSettings(next); err == nil {
		t.Fatal("SaveSettings() error = nil, want persistence error")
	}

	if got := getSettings().Language; got != "en-US" {
		t.Errorf("current Language after failed save = %q, want %q", got, "en-US")
	}
}

func TestSaveSettings_ClonesInputHotkeys(t *testing.T) {
	resetSettings(t)
	t.Setenv("APPDATA", t.TempDir())

	next := defaultSettings()
	next.Language = "th-TH"
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
