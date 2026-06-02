//go:build windows

package main

import (
	"sync"
	"syscall"
	"time"
	"unsafe"

	wp "github.com/dvgamerr/go-livepaper/internal/wallpaper"
)

var (
	kernel32win              = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemPowerStatus = kernel32win.NewProc("GetSystemPowerStatus")

	procGetForegroundWindow = user32win.NewProc("GetForegroundWindow")
	procGetWindowRect       = user32win.NewProc("GetWindowRect")
	procMonitorFromWindow   = user32win.NewProc("MonitorFromWindow")
	procGetMonitorInfoW     = user32win.NewProc("GetMonitorInfoW")
	procGetClassNameW       = user32win.NewProc("GetClassNameW")
)

const monitorDefaultToNearest = 2

type rect struct{ left, top, right, bottom int32 }

type monitorInfo struct {
	cbSize    uint32
	rcMonitor rect
	rcWork    rect
	dwFlags   uint32
}

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

// shellClasses are foreground window classes that represent the desktop / shell
// rather than a real foreground application.
var shellClasses = map[string]bool{
	"Progman":                      true,
	"WorkerW":                      true,
	"Shell_TrayWnd":                true,
	"Windows.UI.Core.CoreWindow":   true,
	"XamlExplorerHostIslandWindow": true,
}

func foregroundClass() (uintptr, string) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return 0, ""
	}
	buf := make([]uint16, 256)
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return hwnd, syscall.UTF16ToString(buf[:n])
}

// foregroundHasApp reports whether a real application (not the shell, not our
// own window) currently holds the foreground.
func foregroundHasApp() bool {
	hwnd, class := foregroundClass()
	if hwnd == 0 || shellClasses[class] {
		return false
	}
	return hwnd != findMainWindow()
}

// foregroundFullscreen reports whether the foreground window covers an entire
// monitor — a strong signal that a fullscreen game or video is running.
func foregroundFullscreen() bool {
	hwnd, class := foregroundClass()
	if hwnd == 0 || shellClasses[class] || hwnd == findMainWindow() {
		return false
	}

	var wr rect
	if r, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr))); r == 0 {
		return false
	}

	mon, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if mon == 0 {
		return false
	}
	mi := monitorInfo{cbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	if r, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); r == 0 {
		return false
	}

	m := mi.rcMonitor
	return wr.left <= m.left && wr.top <= m.top && wr.right >= m.right && wr.bottom >= m.bottom
}

func batterySaverOn() bool {
	var st systemPowerStatus
	if r, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&st))); r == 0 {
		return false
	}
	// SystemStatusFlag bit 0 set == "battery saver" / power-saving mode engaged.
	return st.SystemStatusFlag&1 == 1
}

// ── Combined power / focus state ────────────────────────────────────────────

var (
	powerMu      sync.Mutex
	manualPause  bool
	appliedPause bool
	appliedSpeed = 1.0
	stateApplied bool
)

// toggleManualPause flips the manual (hotkey-driven) pause flag and returns the
// new value.
func toggleManualPause() bool {
	powerMu.Lock()
	manualPause = !manualPause
	v := manualPause
	powerMu.Unlock()
	applyPowerState()
	return v
}

// applyPowerState computes the desired playback state from the current settings
// plus live system signals and pushes it to the running video wallpapers,
// sending IPC only when the state actually changes.
func applyPowerState() {
	if !wp.HasActiveVideoWallpapers() {
		return
	}
	s := getSettings()

	powerMu.Lock()
	pause := manualPause
	powerMu.Unlock()

	if s.PauseOnGame && foregroundFullscreen() {
		pause = true
	}
	if s.PauseOnBattery && batterySaverOn() {
		pause = true
	}

	speed := 1.0
	if !pause && s.ReduceMotionOnFocus && foregroundHasApp() {
		speed = 0.6
	}

	powerMu.Lock()
	defer powerMu.Unlock()
	if !stateApplied || pause != appliedPause {
		wp.PauseVideoWallpapers(pause)
		appliedPause = pause
		appliedSpeed = -1 // force speed re-apply after a pause transition
	}
	if !pause && speed != appliedSpeed {
		wp.SetVideoSpeed(speed)
		appliedSpeed = speed
	}
	stateApplied = true
}

// startPowerFocusWatcher polls system state once per second and keeps the video
// wallpapers' playback state in sync with the game / battery / focus settings.
func startPowerFocusWatcher() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			applyPowerState()
		}
	}()
}
