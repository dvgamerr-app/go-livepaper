package wallpaper

import "golang.org/x/sys/windows"

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	enumDisplayMonitors  = user32.NewProc("EnumDisplayMonitors")
	getMonitorInfoW      = user32.NewProc("GetMonitorInfoW")
	systemParametersInfo = user32.NewProc("SystemParametersInfoW")

	findWindowW                 = user32.NewProc("FindWindowW")
	sendMessageTimeoutW         = user32.NewProc("SendMessageTimeoutW")
	findWindowExW               = user32.NewProc("FindWindowExW")
	setParentW                  = user32.NewProc("SetParent")
	setWindowPosW               = user32.NewProc("SetWindowPos")
	createWindowExW             = user32.NewProc("CreateWindowExW")
	registerClassExW            = user32.NewProc("RegisterClassExW")
	defWindowProcW              = user32.NewProc("DefWindowProcW")
	postQuitMessageW            = user32.NewProc("PostQuitMessage")
	translateMessageW           = user32.NewProc("TranslateMessage")
	dispatchMessageW            = user32.NewProc("DispatchMessageW")
	getMessageW                 = user32.NewProc("GetMessageW")
	showWindowW                 = user32.NewProc("ShowWindow")
	validateRectW               = user32.NewProc("ValidateRect")
	getWindowLongPtrW           = user32.NewProc("GetWindowLongPtrW")
	setWindowLongPtrW           = user32.NewProc("SetWindowLongPtrW")
	getWindowRectW              = user32.NewProc("GetWindowRect")
	setLayeredWindowAttributesW = user32.NewProc("SetLayeredWindowAttributes")
	loadCursorW                 = user32.NewProc("LoadCursorW")

	getModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)
