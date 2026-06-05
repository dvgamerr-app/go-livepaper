//go:build windows

package wallpaper

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateBlackCanvas(t *testing.T) {
	c := CreateBlackCanvas(100, 50)
	if c == nil {
		t.Fatal("CreateBlackCanvas returned nil")
	}
	if c.Bounds().Dx() != 100 || c.Bounds().Dy() != 50 {
		t.Errorf("bounds = %v, want 100×50", c.Bounds())
	}
	// Every pixel must be fully-opaque black.
	r, g, b, a := c.At(50, 25).RGBA()
	if r != 0 || g != 0 || b != 0 || a != 0xffff {
		t.Errorf("expected black pixel, got (%d,%d,%d,%d)", r, g, b, a)
	}
}

func TestGetTempDir(t *testing.T) {
	dir, err := GetTempDir()
	if err != nil {
		t.Fatalf("GetTempDir() error = %v", err)
	}
	if dir == "" {
		t.Error("GetTempDir() returned empty string")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("GetTempDir() dir does not exist: %v", err)
	}
	// Calling it again must be idempotent (directory already exists).
	dir2, err := GetTempDir()
	if err != nil {
		t.Fatalf("GetTempDir() second call error = %v", err)
	}
	if dir2 != dir {
		t.Errorf("GetTempDir() second call = %q, want %q", dir2, dir)
	}
}

func TestCleanTempDir(t *testing.T) {
	dir, err := GetTempDir()
	if err != nil {
		t.Fatal(err)
	}

	// Seed the directory with a test file.
	f, err := os.CreateTemp(dir, "cleantest*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	testFile := f.Name()
	f.Close()

	if err := CleanTempDir(); err != nil {
		t.Errorf("CleanTempDir() error = %v", err)
	}

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Errorf("test file still exists after CleanTempDir: %s", testFile)
	}
}

func TestSaveImageAs(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	path, err := SaveImageAs(img, 80)
	if err != nil {
		t.Fatalf("SaveImageAs() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("SaveImageAs() output file missing: %v", err)
	}
	t.Cleanup(func() { os.Remove(path) })
}

// newTestJPEG writes a w×h JPEG to a temp file and returns the path.
func newTestJPEG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	f, err := os.CreateTemp(t.TempDir(), "testimg*.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestLoadAndResizeImage_ScaleByHeight(t *testing.T) {
	// Wide image (100×50): when target is 60×60, scaleH (1.2) > scaleW (0.6).
	path := newTestJPEG(t, 100, 50)
	img, err := LoadAndResizeImage(path, 60, 60)
	if err != nil {
		t.Fatalf("LoadAndResizeImage() error = %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 60 || b.Dy() != 60 {
		t.Errorf("size = %dx%d, want 60×60", b.Dx(), b.Dy())
	}
}

func TestLoadAndResizeImage_ScaleByWidth(t *testing.T) {
	// Tall image (50×100): when target is 60×60, scaleW (1.2) > scaleH (0.6).
	path := newTestJPEG(t, 50, 100)
	img, err := LoadAndResizeImage(path, 60, 60)
	if err != nil {
		t.Fatalf("LoadAndResizeImage() error = %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 60 || b.Dy() != 60 {
		t.Errorf("size = %dx%d, want 60×60", b.Dx(), b.Dy())
	}
}

func TestLoadAndResizeImage_ExactSize(t *testing.T) {
	// Image matches target dimensions exactly — no actual crop needed.
	path := newTestJPEG(t, 80, 80)
	img, err := LoadAndResizeImage(path, 80, 80)
	if err != nil {
		t.Fatalf("LoadAndResizeImage() error = %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 80 || b.Dy() != 80 {
		t.Errorf("size = %dx%d, want 80×80", b.Dx(), b.Dy())
	}
}

func TestLoadAndResizeImage_FileNotFound(t *testing.T) {
	_, err := LoadAndResizeImage(filepath.Join(t.TempDir(), "nonexistent.jpg"), 100, 100)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadAndResizeImage_InvalidData(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "bad*.jpg")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("not an image")
	f.Close()

	_, err = LoadAndResizeImage(f.Name(), 100, 100)
	if err == nil {
		t.Error("expected error for invalid image data")
	}
}
