package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Settings mirrors every control exposed in the frontend Settings panel.
// It is persisted as JSON in %APPDATA%\livepaper\settings.json and read back
// by the Go side (encoder, window theme, hotkeys, power/focus watchers).
type Settings struct {
	// General
	Language          string `json:"language"`
	ShowNotifications bool   `json:"showNotifications"`
	Telemetry         bool   `json:"telemetry"`

	// Performance
	GPUAcceleration     bool   `json:"gpuAcceleration"`
	VRAMCapMB           int    `json:"vramCapMB"`
	GPUAdapter          string `json:"gpuAdapter"`
	PauseOnGame         bool   `json:"pauseOnGame"`
	PauseOnBattery      bool   `json:"pauseOnBattery"`
	ReduceMotionOnFocus bool   `json:"reduceMotionOnFocus"`
	WindowTheme         string `json:"windowTheme"` // mica | acrylic | solid

	// Startup
	LaunchAtLogin       bool `json:"launchAtLogin"`
	StartMinimized      bool `json:"startMinimized"`
	RestoreLastPlaylist bool `json:"restoreLastPlaylist"`

	// Hotkeys: action -> human-readable combo (e.g. "Ctrl + Shift + >")
	Hotkeys map[string]string `json:"hotkeys"`
}

// hotkey action keys
const (
	HotkeyNext      = "next"
	HotkeyPrev      = "prev"
	HotkeyPlayPause = "playpause"
	HotkeyOpen      = "open"
)

func defaultSettings() Settings {
	return Settings{
		Language:            "en-US",
		ShowNotifications:   true,
		Telemetry:           false,
		GPUAcceleration:     true,
		VRAMCapMB:           256,
		GPUAdapter:          "",
		PauseOnGame:         true,
		PauseOnBattery:      true,
		ReduceMotionOnFocus: false,
		WindowTheme:         "mica",
		LaunchAtLogin:       false,
		StartMinimized:      true,
		RestoreLastPlaylist: true,
		Hotkeys: map[string]string{
			HotkeyNext:      "Ctrl + Shift + >",
			HotkeyPrev:      "Ctrl + Shift + <",
			HotkeyPlayPause: "Ctrl + Shift + /",
			HotkeyOpen:      "Win + W",
		},
	}
}

var (
	settingsMu      sync.RWMutex
	currentSettings = defaultSettings()
)

func settingsPath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.TempDir()
	}
	return filepath.Join(appData, "livepaper", "settings.json")
}

// loadSettings reads settings from disk, filling any missing fields with
// defaults. Always returns a usable value.
func loadSettings() Settings {
	s := defaultSettings()
	data, err := os.ReadFile(settingsPath())
	if err == nil {
		// Unmarshal over the defaults so new fields keep their default value.
		_ = json.Unmarshal(data, &s)
	}
	s.normalize()
	settingsMu.Lock()
	currentSettings = s
	settingsMu.Unlock()
	return s
}

// normalize repairs out-of-range or empty values so the rest of the code can
// trust the settings without re-validating everywhere.
func (s *Settings) normalize() {
	if s.Language == "" {
		s.Language = "en-US"
	}
	switch s.WindowTheme {
	case "mica", "acrylic", "solid":
	default:
		s.WindowTheme = "mica"
	}
	if s.VRAMCapMB < 64 {
		s.VRAMCapMB = 64
	}
	if s.VRAMCapMB > 1024 {
		s.VRAMCapMB = 1024
	}
	def := defaultSettings().Hotkeys
	if s.Hotkeys == nil {
		s.Hotkeys = map[string]string{}
	}
	for k, v := range def {
		if s.Hotkeys[k] == "" {
			s.Hotkeys[k] = v
		}
	}
}

func getSettings() Settings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return currentSettings
}

func saveSettingsToDisk(s Settings) error {
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ── Service methods exposed to the frontend ─────────────────────────────────

// GetSettings returns the current persisted settings.
func (s *AppService) GetSettings() Settings {
	return getSettings()
}

// SaveSettings persists the given settings and applies any OS-level side
// effects (launch-at-login, window backdrop, global hotkeys).
func (s *AppService) SaveSettings(next Settings) error {
	next.normalize()

	settingsMu.Lock()
	prev := currentSettings
	currentSettings = next
	settingsMu.Unlock()

	if err := saveSettingsToDisk(next); err != nil {
		return err
	}

	// Launch at login — only touch the registry when it actually changed.
	if next.LaunchAtLogin != prev.LaunchAtLogin {
		_ = applyLaunchAtLogin(next.LaunchAtLogin)
	}

	// Window backdrop material.
	if next.WindowTheme != prev.WindowTheme {
		applyWindowTheme(next.WindowTheme)
	}

	// Re-register global hotkeys if any binding changed.
	if hotkeysChanged(prev.Hotkeys, next.Hotkeys) {
		reloadHotkeys()
	}

	return nil
}

// GetGPUAdapters lists the display adapters the user can pick for rendering.
func (s *AppService) GetGPUAdapters() []string {
	return enumerateGPUAdapters()
}

func hotkeysChanged(a, b map[string]string) bool {
	if len(a) != len(b) {
		return true
	}
	for k, v := range a {
		if b[k] != v {
			return true
		}
	}
	return false
}
