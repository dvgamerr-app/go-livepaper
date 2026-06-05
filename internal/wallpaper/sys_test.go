//go:build windows

package wallpaper

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestSetWallpaper_FileNotFound(t *testing.T) {
	err := SetWallpaper(filepath.Join(t.TempDir(), "nonexistent.jpg"))
	if err == nil {
		t.Error("SetWallpaper() expected error for nonexistent file")
	}
}

func TestSetWallpaper_ValidFile(t *testing.T) {
	// Create a real (empty) file so the existence check passes, then call
	// SetWallpaper. On a headless CI box the SystemParametersInfo call may
	// fail, but it must not panic and the error (if any) is non-nil.
	f, err := os.CreateTemp(t.TempDir(), "wallpaper*.jpg")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// We do not assert success because this requires an interactive desktop
	// session. We only assert that the function returns without panicking.
	_ = SetWallpaper(f.Name())
}

func TestSetWallpaperStyle_Span(t *testing.T) {
	if err := SetWallpaperStyle(STYLE_SPAN); err != nil {
		t.Errorf("SetWallpaperStyle(STYLE_SPAN) error = %v", err)
	}
	// Verify registry value was written.
	k, err := registry.OpenKey(registry.CURRENT_USER, `Control Panel\Desktop`, registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("could not open registry key: %v", err)
	}
	defer k.Close()

	style, _, err := k.GetStringValue("WallpaperStyle")
	if err != nil {
		t.Fatalf("could not read WallpaperStyle: %v", err)
	}
	if style != "22" {
		t.Errorf("WallpaperStyle = %q, want \"22\"", style)
	}

	tile, _, err := k.GetStringValue("TileWallpaper")
	if err != nil {
		t.Fatalf("could not read TileWallpaper: %v", err)
	}
	if tile != "0" {
		t.Errorf("TileWallpaper = %q, want \"0\"", tile)
	}
}

func TestSetWallpaperStyle_Fill(t *testing.T) {
	if err := SetWallpaperStyle(STYLE_FILL); err != nil {
		t.Errorf("SetWallpaperStyle(STYLE_FILL) error = %v", err)
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, `Control Panel\Desktop`, registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("could not open registry key: %v", err)
	}
	defer k.Close()

	style, _, err := k.GetStringValue("WallpaperStyle")
	if err != nil {
		t.Fatalf("could not read WallpaperStyle: %v", err)
	}
	if style != "10" {
		t.Errorf("WallpaperStyle = %q, want \"10\"", style)
	}
}
