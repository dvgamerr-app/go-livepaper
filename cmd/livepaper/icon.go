package main

// makeTrayIcon returns PNG bytes for the system tray (32×32).
func makeTrayIcon() []byte {
	return readIconFile("icon-32.png")
}

// appIcon returns PNG bytes for the Wails app-level icon.
// Must be PNG — CreateIconFromResourceEx cannot parse a full ICO file header.
func appIcon() []byte {
	return readIconFile("icon-256.png")
}
