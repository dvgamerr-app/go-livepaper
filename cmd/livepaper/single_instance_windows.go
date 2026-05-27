//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	kernel32    = syscall.NewLazyDLL("kernel32.dll")
	user32      = syscall.NewLazyDLL("user32.dll")
	createMutex = kernel32.NewProc("CreateMutexW")
	messageBoxW = user32.NewProc("MessageBoxW")
)

const mutexName = "Global\\livepaper-single-instance"

// checkSingleInstance returns true if this is the only running instance.
// It creates a named mutex; if one already exists, shows an alert and returns false.
func checkSingleInstance() bool {
	name, _ := syscall.UTF16PtrFromString(mutexName)
	handle, _, err := createMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return true
	}
	if err == syscall.ERROR_ALREADY_EXISTS {
		title, _ := syscall.UTF16PtrFromString("Live Paper")
		msg, _ := syscall.UTF16PtrFromString("Live Paper is already running.\nCheck the system tray icon.")
		// MB_OK | MB_ICONINFORMATION | MB_SETFOREGROUND
		messageBoxW.Call(0, uintptr(unsafe.Pointer(msg)), uintptr(unsafe.Pointer(title)), 0x00040040)
		return false
	}
	// Intentionally keep handle open so the mutex stays alive for the process lifetime
	return true
}
