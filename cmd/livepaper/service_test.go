//go:build windows

package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wp "github.com/dvgamerr/go-livepaper/internal/wallpaper"
)

// ---------- extFromContentType ----------

func TestExtFromContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want string
	}{
		{"image/png", ".png"},
		{"image/webp", ".webp"},
		{"image/gif", ".gif"},
		{"video/mp4", ".mp4"},
		{"video/webm", ".webm"},
		{"video/x-matroska", ".mkv"},
		{"video/quicktime", ".mov"},
		{"image/jpeg", ".jpg"},
		{"application/octet-stream", ".jpg"}, // default
		{"", ".jpg"},                         // empty → default
		{"video/mp4; charset=utf-8", ".mp4"}, // with params
		{"video/mkv", ".mkv"},
	}
	for _, tt := range tests {
		if got := extFromContentType(tt.ct); got != tt.want {
			t.Errorf("extFromContentType(%q) = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

// ---------- sanitizeName ----------

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"hello world", "hello_world"},
		{"file-name_123", "file-name_123"},
		{"abc/def", "abc_def"},
		{"", "wallpaper"},        // empty → fallback
		{"   ", "___"},           // spaces
		{"ABC123-_", "ABC123-_"}, // already clean
		{"ภาษาไทย", "_______"},   // 7 non-ASCII runes → 7 underscores
	}
	for _, tt := range tests {
		if got := sanitizeName(tt.input); got != tt.want {
			t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------- maxInt ----------

func TestMaxInt(t *testing.T) {
	tests := []struct{ a, b, want int }{
		{1, 2, 2},
		{5, 3, 5},
		{0, 0, 0},
		{-1, -2, -1},
		{100, 100, 100},
	}
	for _, tt := range tests {
		if got := maxInt(tt.a, tt.b); got != tt.want {
			t.Errorf("maxInt(%d,%d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// ---------- monitorAspectRatio ----------

func TestMonitorAspectRatio(t *testing.T) {
	tests := []struct {
		w, h int32
		want float64
	}{
		{1920, 1080, 1920.0 / 1080.0},
		{2560, 1440, 2560.0 / 1440.0},
		{1920, 0, 0}, // zero height → 0 (avoid division by zero)
		{0, 1080, 0},
	}
	for _, tt := range tests {
		m := wp.MonitorInfo{
			Resolution: wp.Resolution{Width: tt.w, Height: tt.h},
		}
		got := monitorAspectRatio(m)
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("monitorAspectRatio(%dx%d) = %f, want %f", tt.w, tt.h, got, tt.want)
		}
	}
}

// ---------- pickHWEncoder ----------

func TestPickHWEncoder_NvidiaPreference(t *testing.T) {
	encOnce.Do(func() {}) // ensure encSet is initialised (may be empty if ffmpeg absent)

	// Override encSet to simulate available encoders.
	orig := encSet
	t.Cleanup(func() { encSet = orig })

	encSet = map[string]bool{"h264_nvenc": true, "h264_amf": true}

	nvidiaAdapters := []string{
		"NVIDIA GeForce RTX 4090",
		"NVIDIA RTX 3080",
		"GTX 1060",
	}
	for _, a := range nvidiaAdapters {
		if got := pickHWEncoder(a); got != "h264_nvenc" {
			t.Errorf("pickHWEncoder(%q) = %q, want \"h264_nvenc\"", a, got)
		}
	}
}

func TestPickHWEncoder_IntelPreference(t *testing.T) {
	encOnce.Do(func() {})
	orig := encSet
	t.Cleanup(func() { encSet = orig })

	encSet = map[string]bool{"h264_qsv": true}

	if got := pickHWEncoder("Intel UHD Graphics 770"); got != "h264_qsv" {
		t.Errorf("pickHWEncoder(Intel) = %q, want \"h264_qsv\"", got)
	}
}

func TestPickHWEncoder_AMDPreference(t *testing.T) {
	encOnce.Do(func() {})
	orig := encSet
	t.Cleanup(func() { encSet = orig })

	encSet = map[string]bool{"h264_amf": true}

	for _, a := range []string{"AMD Radeon RX 7900", "Radeon Graphics"} {
		if got := pickHWEncoder(a); got != "h264_amf" {
			t.Errorf("pickHWEncoder(%q) = %q, want \"h264_amf\"", a, got)
		}
	}
}

func TestPickHWEncoder_DefaultOrder(t *testing.T) {
	encOnce.Do(func() {})
	orig := encSet
	t.Cleanup(func() { encSet = orig })

	encSet = map[string]bool{"h264_amf": true}

	// Unknown adapter → default pref list is [nvenc, qsv, amf]; amf is last.
	if got := pickHWEncoder("Unknown GPU"); got != "h264_amf" {
		t.Errorf("pickHWEncoder(unknown) = %q, want \"h264_amf\"", got)
	}
}

func TestPickHWEncoder_NoneAvailable(t *testing.T) {
	encOnce.Do(func() {})
	orig := encSet
	t.Cleanup(func() { encSet = orig })

	encSet = map[string]bool{}

	if got := pickHWEncoder("NVIDIA RTX 4090"); got != "" {
		t.Errorf("pickHWEncoder (none available) = %q, want \"\"", got)
	}
}

// ---------- encoderArgs ----------

func TestEncoderArgs_GPUAccelerationOff(t *testing.T) {
	settingsMu.Lock()
	currentSettings = defaultSettings()
	currentSettings.GPUAcceleration = false
	settingsMu.Unlock()
	t.Cleanup(func() {
		settingsMu.Lock()
		currentSettings = defaultSettings()
		settingsMu.Unlock()
	})

	args := encoderArgs()
	if len(args) == 0 {
		t.Fatal("encoderArgs() returned empty slice")
	}
	// Software fallback must use libx264.
	found := false
	for _, a := range args {
		if a == "libx264" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("encoderArgs() with GPUAcceleration=false: libx264 not in %v", args)
	}
}

func TestEncoderArgs_NvidiaEncoder(t *testing.T) {
	encOnce.Do(func() {})
	orig := encSet
	t.Cleanup(func() { encSet = orig })
	encSet = map[string]bool{"h264_nvenc": true}

	settingsMu.Lock()
	currentSettings = defaultSettings()
	currentSettings.GPUAcceleration = true
	currentSettings.GPUAdapter = "NVIDIA GeForce RTX 4090"
	settingsMu.Unlock()
	t.Cleanup(func() {
		settingsMu.Lock()
		currentSettings = defaultSettings()
		settingsMu.Unlock()
	})

	args := encoderArgs()
	found := false
	for _, a := range args {
		if a == "h264_nvenc" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("encoderArgs() with NVIDIA+h264_nvenc: encoder not in %v", args)
	}
}

func TestEncoderArgs_QSVEncoder(t *testing.T) {
	encOnce.Do(func() {})
	orig := encSet
	t.Cleanup(func() { encSet = orig })
	encSet = map[string]bool{"h264_qsv": true}

	settingsMu.Lock()
	currentSettings = defaultSettings()
	currentSettings.GPUAcceleration = true
	currentSettings.GPUAdapter = "Intel UHD Graphics"
	settingsMu.Unlock()
	t.Cleanup(func() {
		settingsMu.Lock()
		currentSettings = defaultSettings()
		settingsMu.Unlock()
	})

	args := encoderArgs()
	found := false
	for _, a := range args {
		if a == "h264_qsv" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("encoderArgs() with Intel+h264_qsv: encoder not in %v", args)
	}
}

func TestEncoderArgs_AMFEncoder(t *testing.T) {
	encOnce.Do(func() {})
	orig := encSet
	t.Cleanup(func() { encSet = orig })
	encSet = map[string]bool{"h264_amf": true}

	settingsMu.Lock()
	currentSettings = defaultSettings()
	currentSettings.GPUAcceleration = true
	currentSettings.GPUAdapter = "AMD Radeon RX 7900 XT"
	settingsMu.Unlock()
	t.Cleanup(func() {
		settingsMu.Lock()
		currentSettings = defaultSettings()
		settingsMu.Unlock()
	})

	args := encoderArgs()
	found := false
	for _, a := range args {
		if a == "h264_amf" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("encoderArgs() with AMD+h264_amf: encoder not in %v", args)
	}
}

func TestEncoderArgs_GPUOnNoHWEncoder(t *testing.T) {
	encOnce.Do(func() {})
	orig := encSet
	t.Cleanup(func() { encSet = orig })
	encSet = map[string]bool{} // no HW encoders

	settingsMu.Lock()
	currentSettings = defaultSettings()
	currentSettings.GPUAcceleration = true
	currentSettings.GPUAdapter = "NVIDIA GeForce RTX 4090"
	settingsMu.Unlock()
	t.Cleanup(func() {
		settingsMu.Lock()
		currentSettings = defaultSettings()
		settingsMu.Unlock()
	})

	args := encoderArgs()
	// Falls back to libx264 when no hw encoder is available.
	found := false
	for _, a := range args {
		if a == "libx264" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("encoderArgs() GPU on, no HW encoder: expected libx264 fallback in %v", args)
	}
}

// ---------- mpvArgsFromSettings ----------

func TestMpvArgsFromSettings_GPUAccelOn(t *testing.T) {
	settingsMu.Lock()
	currentSettings = defaultSettings()
	currentSettings.GPUAcceleration = true
	currentSettings.GPUAdapter = "NVIDIA GeForce RTX"
	currentSettings.VRAMCapMB = 256
	settingsMu.Unlock()
	t.Cleanup(func() {
		settingsMu.Lock()
		currentSettings = defaultSettings()
		settingsMu.Unlock()
	})

	args := mpvArgsFromSettings()
	hasHwdecAuto, hasAdapter, hasDemuxer := false, false, false
	for _, a := range args {
		if a == "--hwdec=auto" {
			hasHwdecAuto = true
		}
		if a == "--d3d11-adapter=NVIDIA GeForce RTX" {
			hasAdapter = true
		}
		if strings.HasPrefix(a, "--demuxer-max-bytes") {
			hasDemuxer = true
		}
	}
	if !hasHwdecAuto {
		t.Errorf("mpvArgsFromSettings (GPU on): missing --hwdec=auto in %v", args)
	}
	if !hasAdapter {
		t.Errorf("mpvArgsFromSettings (GPU on): missing --d3d11-adapter flag in %v", args)
	}
	if !hasDemuxer {
		t.Errorf("mpvArgsFromSettings (GPU on): missing --demuxer-max-bytes flag in %v", args)
	}
}

func TestMpvArgsFromSettings_GPUAccelOff(t *testing.T) {
	settingsMu.Lock()
	currentSettings = defaultSettings()
	currentSettings.GPUAcceleration = false
	currentSettings.GPUAdapter = ""
	currentSettings.VRAMCapMB = 0
	settingsMu.Unlock()
	t.Cleanup(func() {
		settingsMu.Lock()
		currentSettings = defaultSettings()
		settingsMu.Unlock()
	})

	args := mpvArgsFromSettings()
	hasHwdecNo := false
	for _, a := range args {
		if a == "--hwdec=no" {
			hasHwdecNo = true
		}
	}
	if !hasHwdecNo {
		t.Errorf("mpvArgsFromSettings (GPU off): missing --hwdec=no in %v", args)
	}
}

func TestMpvArgsFromSettings_NoAdapter(t *testing.T) {
	settingsMu.Lock()
	currentSettings = defaultSettings()
	currentSettings.GPUAcceleration = true
	currentSettings.GPUAdapter = ""
	settingsMu.Unlock()
	t.Cleanup(func() {
		settingsMu.Lock()
		currentSettings = defaultSettings()
		settingsMu.Unlock()
	})

	args := mpvArgsFromSettings()
	for _, a := range args {
		if strings.HasPrefix(a, "--d3d11-adapter=") {
			t.Errorf("mpvArgsFromSettings (no adapter): unexpected %q flag", a)
		}
	}
}

// ---------- thumbnail cache ----------

func TestThumbnailCacheDirIsBesideExecutable(t *testing.T) {
	dir, err := thumbnailCacheDir()
	if err != nil {
		t.Fatalf("thumbnailCacheDir() error = %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	want := filepath.Join(filepath.Dir(exe), "data", "thumbnail")
	if dir != want {
		t.Errorf("thumbnailCacheDir() = %q, want %q", dir, want)
	}
	if info, err := os.Stat(dir); err != nil {
		t.Errorf("thumbnail cache directory was not created: %v", err)
	} else if !info.IsDir() {
		t.Errorf("thumbnail cache path is not a directory: %q", dir)
	}
}

// ---------- layoutMonitors ----------

func TestLayoutMonitors(t *testing.T) {
	layout := MonitorLayoutData{
		CanvasWidth:  3840,
		CanvasHeight: 1080,
		Monitors:     make([]MonitorData, 2),
	}

	scale := 0.5
	m := wp.MonitorInfo{
		Index:   0,
		Primary: true,
		Resolution: wp.Resolution{
			Width: 1920, Height: 1080,
			X: 0, Y: 0,
		},
	}
	layout.layoutMonitors(0, m, scale)

	got := layout.Monitors[0]
	if got.Width != 1920 || got.Height != 1080 {
		t.Errorf("Width/Height = %d/%d, want 1920/1080", got.Width, got.Height)
	}
	if got.PreviewWidth != 960 || got.PreviewHeight != 540 {
		t.Errorf("PreviewWidth/Height = %d/%d, want 960/540", got.PreviewWidth, got.PreviewHeight)
	}
	if !got.Primary {
		t.Error("Primary should be true")
	}
}

// ---------- AppService.FileExists ----------

func TestFileExists(t *testing.T) {
	svc := &AppService{}

	f, err := os.CreateTemp(t.TempDir(), "exist*.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	if !svc.FileExists(f.Name()) {
		t.Errorf("FileExists(%q) = false, want true", f.Name())
	}
	if svc.FileExists(f.Name() + "_nope") {
		t.Error("FileExists(nonexistent) = true, want false")
	}
}
