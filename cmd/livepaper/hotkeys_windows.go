//go:build windows

package main

import (
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	procRegisterHotKey     = user32win.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32win.NewProc("UnregisterHotKey")
	procGetMessageW        = user32win.NewProc("GetMessageW")
	procPostThreadMessageW = user32win.NewProc("PostThreadMessageW")
	procGetCurrentThreadID = kernel32win.NewProc("GetCurrentThreadId")
)

const (
	modAlt      uintptr = 0x0001
	modControl  uintptr = 0x0002
	modShift    uintptr = 0x0004
	modWin      uintptr = 0x0008
	modNoRepeat uintptr = 0x4000

	wmHotkey    = 0x0312
	wmAppReload = 0x8000 // custom: re-read settings and re-register
)

// hotkey ids — order matters; index maps to the action list below.
var hotkeyActions = []string{HotkeyNext, HotkeyPrev, HotkeyPlayPause, HotkeyOpen}

type win32Msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

var (
	hotkeyMu       sync.Mutex
	hotkeyThreadID uintptr
	hotkeyApp      *application.App
	hotkeyWindow   *application.WebviewWindow
)

// startHotkeyManager runs a dedicated message loop on a locked OS thread that
// owns the global hotkeys and translates WM_HOTKEY into Wails events.
func startHotkeyManager(app *application.App, window *application.WebviewWindow) {
	hotkeyApp = app
	hotkeyWindow = window

	go func() {
		runtime.LockOSThread()
		tid, _, _ := procGetCurrentThreadID.Call()
		hotkeyMu.Lock()
		hotkeyThreadID = tid
		hotkeyMu.Unlock()

		registerAllHotkeys()

		var msg win32Msg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			switch int32(r) {
			case -1:
				continue
			case 0:
				return // WM_QUIT
			}
			switch msg.message {
			case wmHotkey:
				fireHotkey(int(msg.wParam))
			case wmAppReload:
				registerAllHotkeys()
			}
		}
	}()
}

// reloadHotkeys asks the hotkey thread to unregister and re-register from the
// latest settings.
func reloadHotkeys() {
	hotkeyMu.Lock()
	tid := hotkeyThreadID
	hotkeyMu.Unlock()
	if tid != 0 {
		procPostThreadMessageW.Call(tid, wmAppReload, 0, 0)
	}
}

func registerAllHotkeys() {
	s := getSettings()
	for i := range hotkeyActions {
		id := i + 1
		procUnregisterHotKey.Call(0, uintptr(id))
		combo := s.Hotkeys[hotkeyActions[i]]
		mods, vk, ok := parseCombo(combo)
		if !ok {
			continue
		}
		procRegisterHotKey.Call(0, uintptr(id), mods|modNoRepeat, uintptr(vk))
	}
}

func fireHotkey(id int) {
	if id < 1 || id > len(hotkeyActions) {
		return
	}
	action := hotkeyActions[id-1]

	switch action {
	case HotkeyOpen:
		if hotkeyWindow != nil {
			hotkeyWindow.Show()
			hotkeyWindow.Focus()
		}
	case HotkeyPlayPause:
		paused := toggleManualPause()
		if hotkeyApp != nil {
			hotkeyApp.Event.Emit("video:paused", map[string]bool{"paused": paused})
		}
	}

	if hotkeyApp != nil {
		hotkeyApp.Event.Emit("hotkey:"+action, nil)
	}
}

// parseCombo converts a human-readable combo like "Ctrl + Shift + >" into Win32
// modifier flags and a virtual-key code.
func parseCombo(combo string) (mods uintptr, vk uint32, ok bool) {
	if combo == "" {
		return 0, 0, false
	}
	for _, raw := range strings.Split(combo, "+") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		switch strings.ToLower(tok) {
		case "ctrl", "control":
			mods |= modControl
		case "shift", "⇧":
			mods |= modShift
		case "alt", "option":
			mods |= modAlt
		case "win", "cmd", "meta", "⊞", "super":
			mods |= modWin
		default:
			if v, found := keyToVK(tok); found {
				vk = v
			}
		}
	}
	return mods, vk, vk != 0
}

// keyToVK maps a single key token to its virtual-key code.
func keyToVK(tok string) (uint32, bool) {
	switch strings.ToLower(tok) {
	case ">", ".", "period":
		return 0xBE, true // VK_OEM_PERIOD
	case "<", ",", "comma":
		return 0xBC, true // VK_OEM_COMMA
	case "/", "?", "slash":
		return 0xBF, true // VK_OEM_2
	case "space", "spacebar":
		return 0x20, true
	case "up":
		return 0x26, true
	case "down":
		return 0x28, true
	case "left":
		return 0x25, true
	case "right":
		return 0x27, true
	case "enter", "return":
		return 0x0D, true
	}
	if len(tok) == 1 {
		c := tok[0]
		switch {
		case c >= 'a' && c <= 'z':
			return uint32(c-'a') + 0x41, true
		case c >= 'A' && c <= 'Z':
			return uint32(c-'A') + 0x41, true
		case c >= '0' && c <= '9':
			return uint32(c-'0') + 0x30, true
		}
	}
	return 0, false
}
