//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	dwmapi                    = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")

	user32win       = syscall.NewLazyDLL("user32.dll")
	procFindWindowW = user32win.NewProc("FindWindowW")
)

const (
	dwmwaUseImmersiveDarkMode = 20
	dwmwaSystemBackdropType   = 38

	// DWM_SYSTEMBACKDROP_TYPE values
	dwmsbtNone       int32 = 1
	dwmsbtMainWindow int32 = 2 // Mica
	dwmsbtTransient  int32 = 3 // Acrylic
)

// findMainWindow returns the HWND of the LivePaper webview window, located by
// its window title.
func findMainWindow() uintptr {
	title, _ := syscall.UTF16PtrFromString("Live Paper")
	h, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	return h
}

// applyWindowTheme switches the DWM system backdrop material of the main window
// at runtime (Windows 11 22621+). On older builds the call is a harmless no-op;
// the frontend always mirrors the material in CSS so the change is still
// visible either way.
func applyWindowTheme(theme string) {
	hwnd := findMainWindow()
	if hwnd == 0 {
		return
	}

	var backdrop int32
	switch theme {
	case "acrylic":
		backdrop = dwmsbtTransient
	case "solid":
		backdrop = dwmsbtNone
	default: // mica
		backdrop = dwmsbtMainWindow
	}

	dark := int32(1)
	procDwmSetWindowAttribute.Call(hwnd, uintptr(dwmwaUseImmersiveDarkMode),
		uintptr(unsafe.Pointer(&dark)), 4)
	procDwmSetWindowAttribute.Call(hwnd, uintptr(dwmwaSystemBackdropType),
		uintptr(unsafe.Pointer(&backdrop)), 4)
}
