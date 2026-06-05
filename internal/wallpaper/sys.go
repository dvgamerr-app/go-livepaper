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
	spifSendChange      = 0x02
	retSuccess          = "The operation completed successfully."
)

func broadcastSettingChange(wallpaperPtr uintptr, flags uintptr) error {
	ret, _, err := systemParametersInfo.Call(spiSetDeskWallpaper, 0, wallpaperPtr, flags)
	if ret == 0 {
		if err != nil && err.Error() != retSuccess {
			return fmt.Errorf("SystemParametersInfo call failed: %w", err)
		}
	} else {
		if err != nil && err.Error() != retSuccess {
			fmt.Printf("SystemParametersInfo returned non-zero (%d) but reported an error: %v\n", ret, err)
		}
	}
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

	if err := broadcastSettingChange(uintptr(unsafe.Pointer(imagePathPtr)), spifUpdateINIFile|spifSendChange); err != nil {
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
