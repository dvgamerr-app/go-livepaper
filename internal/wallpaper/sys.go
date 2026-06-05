package wallpaper

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

type WallpaperStyle int

const (
	STYLE_SPAN WallpaperStyle = 22
	STYLE_FILL WallpaperStyle = 10
)

const (
	spiSetDeskWallpaper = 0x0014
	spifUpdateINIFile   = 0x01
	hwndBroadcast       = 0xFFFF // HWND_BROADCAST
	wmSettingChange     = 0x001A // WM_SETTINGCHANGE / WM_WININICHANGE
	retSuccess          = "The operation completed successfully."
)

// applyWallpaper sets the wallpaper via SystemParametersInfo and then
// notifies all top-level windows asynchronously.
//
// We intentionally avoid SPIF_SENDCHANGE (0x02) because it triggers a
// synchronous SendMessage(HWND_BROADCAST, WM_SETTINGCHANGE) that waits up
// to 5 s for every top-level window to respond.  When Explorer is busy
// processing desktop-window changes (e.g. mpv embedding / removal), the
// broadcast times out and the desktop may not refresh.
//
// Instead we use SendNotifyMessageW which is fire-and-forget: it posts the
// notification without blocking, so it never times out.
func applyWallpaper(wallpaperPtr uintptr) error {
	ret, _, err := systemParametersInfo.Call(spiSetDeskWallpaper, 0, wallpaperPtr, spifUpdateINIFile)
	if ret == 0 && err != nil && err.Error() != retSuccess {
		return fmt.Errorf("SystemParametersInfo call failed: %w", err)
	}
	sendNotifyMessageW.Call(hwndBroadcast, wmSettingChange, spiSetDeskWallpaper, 0)
	return nil
}

func SetWallpaper(imagePath string) error {
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return fmt.Errorf("image file not found: %s", imagePath)
	}

	imagePathPtr, err := syscall.UTF16PtrFromString(imagePath)
	if err != nil {
		return fmt.Errorf("failed to convert path to UTF16 pointer: %w", err)
	}

	if err := applyWallpaper(uintptr(unsafe.Pointer(imagePathPtr))); err != nil {
		return fmt.Errorf("failed to set wallpaper: %w", err)
	}

	return nil
}

func SetWallpaperStyle(style WallpaperStyle) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Control Panel\Desktop`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	if err = key.SetStringValue("WallpaperStyle", fmt.Sprintf("%d", style)); err != nil {
		return fmt.Errorf("failed to set WallpaperStyle registry value: %w", err)
	}

	if err = key.SetStringValue("TileWallpaper", "0"); err != nil {
		return fmt.Errorf("failed to set TileWallpaper registry value: %w", err)
	}

	return nil
}
