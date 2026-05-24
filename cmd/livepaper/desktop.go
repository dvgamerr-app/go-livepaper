package main

import (
	"fmt"
	"log"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wsVisible      uintptr = 0x10000000
	wsPopup        uintptr = 0x80000000
	wsChild        uintptr = 0x40000000
	wsExToolwindow          uintptr = 0x00000080 // hide from taskbar and Alt+Tab
	wsExNoActivate          uintptr = 0x08000000 // prevent focus steal
	wsExLayered             uintptr = 0x00080000 // DWM compositing (required on 24H2)
	wsExNoRedirectionBitmap uintptr = 0x00200000 // set on Progman on Windows 11 24H2
	lwaAlpha                uintptr = 0x2
	wmSysCommand                    = 0x0112
	scMinimize              uintptr = 0xF020
	hwndBottom     uintptr = 1
	swShow                 = 5
	csHredraw              = 0x0002
	csVredraw              = 0x0001
	idcArrow               = 32512
	wmDestroy              = 0x0002
	wmPaint                = 0x000F
	smtoNormal             = 0x0000
	swpFrameChanged uintptr = 0x0020 // notify Windows of GWL_STYLE change
	swpNoMove       uintptr = 0x0002
	swpNoSize       uintptr = 0x0001
	swpNoZOrder     uintptr = 0x0004
	swpNoActivate   uintptr = 0x0010

	// GetWindowLongPtr / SetWindowLongPtr indices (signed, passed as uintptr).
	gwlpStyle   uintptr = 0xFFFFFFFFFFFFFFF0 // GWL_STYLE   = -16
	gwlpExStyle uintptr = 0xFFFFFFFFFFFFFFEC // GWL_EXSTYLE = -20
)

var (
	findWindowW         = user32.NewProc("FindWindowW")
	sendMessageTimeoutW = user32.NewProc("SendMessageTimeoutW")
	findWindowExW       = user32.NewProc("FindWindowExW")
	setParentW          = user32.NewProc("SetParent")
	setWindowPosW       = user32.NewProc("SetWindowPos")
	createWindowExW     = user32.NewProc("CreateWindowExW")
	registerClassExW    = user32.NewProc("RegisterClassExW")
	defWindowProcW      = user32.NewProc("DefWindowProcW")
	postQuitMessageW    = user32.NewProc("PostQuitMessage")
	loadCursorW         = user32.NewProc("LoadCursorW")
	getMessageW         = user32.NewProc("GetMessageW")
	translateMessageW   = user32.NewProc("TranslateMessage")
	dispatchMessageW    = user32.NewProc("DispatchMessageW")
	showWindowW   = user32.NewProc("ShowWindow")
	validateRectW = user32.NewProc("ValidateRect")
	getWindowLongPtrW          = user32.NewProc("GetWindowLongPtrW")
	setWindowLongPtrW          = user32.NewProc("SetWindowLongPtrW")
	getWindowRectW             = user32.NewProc("GetWindowRect")
	setLayeredWindowAttributesW = user32.NewProc("SetLayeredWindowAttributes")

	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	getModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

// wndClassEx mirrors WNDCLASSEXW (80 bytes on amd64).
type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

// winMsg mirrors MSG (44 usable bytes; C sizeof = 48 with trailing padding).
type winMsg struct {
	hwnd    uintptr
	message uint32
	_pad    uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	ptX     int32
	ptY     int32
}

// winRect mirrors RECT for GetWindowRect.
type winRect struct{ left, top, right, bottom int32 }

// findBgWorkerW looks for a background WorkerW that is large enough to be a
// genuine desktop background layer (not a tiny system utility window).
// minW/minH are the minimum acceptable dimensions.
//
// Returns 0 when no valid WorkerW exists — caller should fall back to the
// top-level-below-Progman strategy (Case B).
func findBgWorkerW(minW, minH int) uintptr {
	shellViewClass, _ := windows.UTF16PtrFromString("SHELLDLL_DefView")
	workerWClass, _ := windows.UTF16PtrFromString("WorkerW")

	var iconWW uintptr
	ww, _, _ := findWindowExW.Call(0, 0, uintptr(unsafe.Pointer(workerWClass)), 0)
	for ww != 0 {
		child, _, _ := findWindowExW.Call(ww, 0, uintptr(unsafe.Pointer(shellViewClass)), 0)
		if child != 0 {
			iconWW = ww
			break
		}
		ww, _, _ = findWindowExW.Call(0, ww, uintptr(unsafe.Pointer(workerWClass)), 0)
	}

	// Case A: two-layer WorkerW structure — return the layer AFTER the icon host.
	if iconWW != 0 {
		bgWW, _, _ := findWindowExW.Call(0, iconWW, uintptr(unsafe.Pointer(workerWClass)), 0)
		if bgWW != 0 {
			var r winRect
			getWindowRectW.Call(bgWW, uintptr(unsafe.Pointer(&r)))
			if int(r.right-r.left) >= minW && int(r.bottom-r.top) >= minH {
				log.Printf("WorkerW: icon=0x%X bg=0x%X size=%dx%d [Case A]\n",
					iconWW, bgWW, r.right-r.left, r.bottom-r.top)
				return bgWW
			}
			log.Printf("WorkerW bg=0x%X too small (%dx%d), skip\n",
				bgWW, r.right-r.left, r.bottom-r.top)
		}
	}

	// Windows 11: SHELLDLL_DefView lives in Progman directly (no iconWW).
	// Scan every WorkerW for a large-enough background layer created by 0x052C.
	ww, _, _ = findWindowExW.Call(0, 0, uintptr(unsafe.Pointer(workerWClass)), 0)
	for ww != 0 {
		var r winRect
		getWindowRectW.Call(ww, uintptr(unsafe.Pointer(&r)))
		if int(r.right-r.left) >= minW && int(r.bottom-r.top) >= minH {
			log.Printf("WorkerW: found large 0x%X size=%dx%d [Win11 fallback]\n",
				ww, r.right-r.left, r.bottom-r.top)
			return ww
		}
		log.Printf("WorkerW 0x%X too small (%dx%d), skip\n", ww, r.right-r.left, r.bottom-r.top)
		ww, _, _ = findWindowExW.Call(0, ww, uintptr(unsafe.Pointer(workerWClass)), 0)
	}

	log.Printf("WorkerW: no valid background layer found — using top-level strategy\n")
	return 0
}

// findProgman returns the Progman window handle and triggers the WorkerW
// split. (0xD, 0x1) is required on Windows 11 to create a full-size WorkerW;
// the second call with (0, 0) keeps compatibility with Windows 10.
func findProgman() uintptr {
	progmanClass, _ := windows.UTF16PtrFromString("Progman")
	hProgman, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(progmanClass)), 0)
	if hProgman != 0 {
		sendMessageTimeoutW.Call(hProgman, 0x052C, 0, 0, smtoNormal, 1000, 0)
		sendMessageTimeoutW.Call(hProgman, 0x052C, 0, 0, smtoNormal, 1000, 0)
	}
	return hProgman
}

// createDesktopWindow embeds a borderless window behind the desktop icons.
//
// Case A — a valid background WorkerW exists (size >= w/2 × h/2):
//   SetParent the window into WorkerW, convert screen→client coords via GetWindowRect.
//
// Case B — icons live directly in Progman, no valid large WorkerW:
//   Create a top-level WS_POPUP|WS_EX_TOOLWINDOW window at (screenX,screenY) and
//   place it just below Progman in z-order. Progman's icons naturally float above.
//
// Returns (hwnd, progmanRef): progmanRef is 0 for Case A, hProgman for Case B.
// progmanRef is used by renderFrames to periodically re-assert z-order.
//
// Must be called from the goroutine that will also run runMessageLoop.
func createDesktopWindow(screenX, screenY, w, h int) (hwnd, progmanRef uintptr, err error) {
	runtime.LockOSThread()

	hProgman := findProgman()
	if hProgman == 0 {
		return 0, 0, fmt.Errorf("Progman window not found")
	}
	log.Printf("Progman: 0x%X\n", hProgman)

	bgWW := findBgWorkerW(w/2, h/2)

	hInstance, _, _ := getModuleHandleW.Call(0)
	className, _ := windows.UTF16PtrFromString("LivepaperWindow")
	windowName, _ := windows.UTF16PtrFromString("livepaper")

	wndProc := windows.NewCallback(func(hwnd, msg, wParam, lParam uintptr) uintptr {
		switch msg {
		case wmDestroy:
			postQuitMessageW.Call(0)
			return 0
		case wmPaint:
			validateRectW.Call(hwnd, 0)
			return 0
		case wmSysCommand:
			if wParam&0xFFF0 == scMinimize {
				return 0 // block Win+D / minimize
			}
		}
		r, _, _ := defWindowProcW.Call(hwnd, msg, wParam, lParam)
		return r
	})

	cursor, _, _ := loadCursorW.Call(0, idcArrow)
	wc := wndClassEx{
		style:         csHredraw | csVredraw,
		lpfnWndProc:   wndProc,
		hInstance:     hInstance,
		hCursor:       cursor,
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	registerClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	if bgWW != 0 {
		// Case A: embed as child of the background WorkerW layer.
		// Convert screen coords to WorkerW client coords via its screen rect.
		var parentRect winRect
		getWindowRectW.Call(bgWW, uintptr(unsafe.Pointer(&parentRect)))
		log.Printf("WorkerW rect: left=%d top=%d right=%d bottom=%d\n",
			parentRect.left, parentRect.top, parentRect.right, parentRect.bottom)

		clientX := screenX - int(parentRect.left)
		clientY := screenY - int(parentRect.top)
		log.Printf("Case A — client pos: (%d,%d) size: %dx%d\n", clientX, clientY, w, h)

		// Create as WS_POPUP first; cross-process SetParent rejects WS_CHILD.
		hwnd, _, lastErr := createWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(className)),
			uintptr(unsafe.Pointer(windowName)),
			wsVisible|wsPopup,
			uintptr(screenX), uintptr(screenY), uintptr(w), uintptr(h),
			0, 0, hInstance, 0,
		)
		if hwnd == 0 {
			return 0, 0, fmt.Errorf("CreateWindowExW: %w", lastErr)
		}

		setParentW.Call(hwnd, bgWW)

		// WS_POPUP → WS_CHILD after cross-process reparent.
		style, _, _ := getWindowLongPtrW.Call(hwnd, gwlpStyle)
		style = (style &^ wsPopup) | wsChild
		setWindowLongPtrW.Call(hwnd, gwlpStyle, style)

		setWindowLongPtrW.Call(hwnd, gwlpExStyle, wsExToolwindow)
		setWindowPosW.Call(hwnd, 0, 0, 0, 0, 0,
			swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged|swpNoActivate)
		setWindowPosW.Call(hwnd, hwndBottom,
			uintptr(clientX), uintptr(clientY), uintptr(w), uintptr(h),
			swpNoActivate)
		showWindowW.Call(hwnd, swShow)
		return hwnd, 0, nil
	}

	// Case B (Windows 11 24H2): no full-size WorkerW exists.
	// Progman has WS_EX_NOREDIRECTIONBITMAP → DWM composites directly.
	// Technique from Lively Wallpaper v2.2.0.0:
	//   • Reparent into Progman as WS_CHILD
	//   • Add WS_EX_LAYERED so DWM includes us in its composition tree
	//   • Insert just below SHELLDLL_DefView (desktop icons) in z-order
	var progmanRect winRect
	getWindowRectW.Call(hProgman, uintptr(unsafe.Pointer(&progmanRect)))

	clientX := screenX - int(progmanRect.left)
	clientY := screenY - int(progmanRect.top)
	log.Printf("Case B — Progman 0x%X rect=(%d,%d,%d,%d) client=(%d,%d) size=%dx%d\n",
		hProgman, progmanRect.left, progmanRect.top, progmanRect.right, progmanRect.bottom,
		clientX, clientY, w, h)

	// Create with WS_EX_LAYERED from the start — setting it after creation via
	// SetWindowLongPtr gets cleared by SWP_FRAMECHANGED on 24H2.
	hwnd, _, lastErr := createWindowExW.Call(
		uintptr(wsExLayered),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		wsVisible|wsPopup,
		uintptr(screenX), uintptr(screenY), uintptr(w), uintptr(h),
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		return 0, 0, fmt.Errorf("CreateWindowExW: %w", lastErr)
	}

	// Reparent into Progman (cross-process SetParent is allowed for popups).
	setParentW.Call(hwnd, hProgman)

	// WS_POPUP → WS_CHILD. SWP_FRAMECHANGED notifies Windows of the style change
	// but also clears WS_EX_LAYERED — re-set it explicitly afterwards.
	style, _, _ := getWindowLongPtrW.Call(hwnd, gwlpStyle)
	style = (style &^ wsPopup) | wsChild
	setWindowLongPtrW.Call(hwnd, gwlpStyle, style)
	setWindowPosW.Call(hwnd, 0, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged|swpNoActivate)

	// SWP_FRAMECHANGED clears WS_EX_LAYERED — re-set it and re-configure.
	setWindowLongPtrW.Call(hwnd, gwlpExStyle, wsExLayered)
	setLayeredWindowAttributesW.Call(hwnd, 0, 255, lwaAlpha)

	// Find SHELLDLL_DefView (Progman's icon child) and insert our window just
	// below it so icons remain visible above the video.
	shellViewClass, _ := windows.UTF16PtrFromString("SHELLDLL_DefView")
	hShellView, _, _ := findWindowExW.Call(hProgman, 0, uintptr(unsafe.Pointer(shellViewClass)), 0)
	insertAfter := hwndBottom
	if hShellView != 0 {
		insertAfter = hShellView
		log.Printf("Case B — SHELLDLL_DefView=0x%X, inserting below it\n", hShellView)
	} else {
		log.Printf("Case B — SHELLDLL_DefView not found, using HWND_BOTTOM\n")
	}

	setWindowPosW.Call(hwnd, insertAfter,
		uintptr(clientX), uintptr(clientY), uintptr(w), uintptr(h),
		swpNoActivate)
	showWindowW.Call(hwnd, swShow)
	return hwnd, 0, nil
}

// runMessageLoop pumps Win32 messages on the locked OS thread until WM_QUIT.
func runMessageLoop() {
	var m winMsg
	for {
		r, _, _ := getMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if r == 0 {
			break
		}
		translateMessageW.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
