//go:build windows

package main

import "testing"

func TestParseCombo_Valid(t *testing.T) {
	tests := []struct {
		name     string
		combo    string
		wantMods uintptr
		wantVK   uint32
	}{
		{name: "default next", combo: "Ctrl + Shift + >", wantMods: modControl | modShift, wantVK: 0xBE},
		{name: "windows key", combo: "Win + W", wantMods: modWin, wantVK: 0x57},
		{name: "aliases", combo: "control + option + comma", wantMods: modControl | modAlt, wantVK: 0xBC},
		{name: "single key", combo: "7", wantVK: 0x37},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mods, vk, ok := parseCombo(tt.combo)
			if !ok {
				t.Fatalf("parseCombo(%q) rejected a valid combination", tt.combo)
			}
			if mods != tt.wantMods || vk != tt.wantVK {
				t.Errorf("parseCombo(%q) = (%#x, %#x), want (%#x, %#x)",
					tt.combo, mods, vk, tt.wantMods, tt.wantVK)
			}
		})
	}
}

func TestParseCombo_Invalid(t *testing.T) {
	tests := []string{
		"",
		"Ctrl + Shift",
		"Ctrl + A + B",
		"Ctrl + F1",
		"Ctrl ++ A",
	}

	for _, combo := range tests {
		t.Run(combo, func(t *testing.T) {
			mods, vk, ok := parseCombo(combo)
			if ok || mods != 0 || vk != 0 {
				t.Errorf("parseCombo(%q) = (%#x, %#x, %t), want rejected zero value", combo, mods, vk, ok)
			}
		})
	}
}
