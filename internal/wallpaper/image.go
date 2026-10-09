package wallpaper

import (
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	_ "golang.org/x/image/webp"
	"os"
	"path/filepath"

	"github.com/nfnt/resize"
)

func LoadAndResizeImage(path string, width, height uint) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	if _, err = file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	img = applyOrientation(img, getOrientation(file))

	bounds := img.Bounds()
	origW := float64(bounds.Dx())
	origH := float64(bounds.Dy())

	// cover: scale so the image fills the monitor entirely, then crop to center
	scaleW := float64(width) / origW
	scaleH := float64(height) / origH
	scale := scaleW
	if scaleH > scaleW {
		scale = scaleH
	}
	newW := uint(origW * scale)
	newH := uint(origH * scale)

	resized := resize.Resize(newW, newH, img, resize.Lanczos3)

	result := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	srcX := (int(newW) - int(width)) / 2
	srcY := (int(newH) - int(height)) / 2
	draw.Draw(result, result.Bounds(), resized, image.Point{X: srcX, Y: srcY}, draw.Src)

	return result, nil
}

func CreateBlackCanvas(width, height int) *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: image.Black}, image.Point{}, draw.Src)
	return canvas
}

func SaveImageAs(img image.Image, quality int) (string, error) {
	tempDir, err := GetTempDir()
	if err != nil {
		return "", fmt.Errorf("failed to get temp directory: %w", err)
	}
	filename := filepath.Join(tempDir, "background.jpg")

	file, err := os.Create(filename)
	if err != nil {
		return filename, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	err = jpeg.Encode(file, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return filename, fmt.Errorf("failed to encode image: %w", err)
	}

	return filename, nil
}

// CacheDir returns the directory for generated files (background.jpg, encoded
// videos). It lives beside the executable so everything stays inside the
// install directory instead of %TEMP%, where it can be cleaned out from under
// the running wallpaper. Falls back to %TEMP% if the executable path is unknown.
func CacheDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "data")
	}
	return filepath.Join(os.TempDir(), "livepaper")
}

func GetTempDir() (string, error) {
	tempDir := CacheDir()
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}
	return tempDir, nil
}

// CleanTempDir empties the data directory. Files named in keep (full paths or
// base names) are preserved, e.g. encoded videos still assigned to a monitor.
func CleanTempDir(keep ...string) error {
	tempDir, err := GetTempDir()
	if err != nil {
		return err
	}

	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		return nil
	}

	dirEntries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("failed to read temp directory: %w", err)
	}

	keepSet := map[string]bool{"background.jpg": true, "thumbnail": true}
	for _, k := range keep {
		keepSet[filepath.Base(k)] = true
	}
	for _, entry := range dirEntries {
		// background.jpg is the applied wallpaper, thumbnail is the persistent
		// preview cache, and keep holds files still in use; none are disposable.
		if keepSet[entry.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(tempDir, entry.Name())); err != nil {
			// On Windows a file held open by the browser or mpv cannot be
			// removed until the handle is released. Skip and continue so the
			// rest of the directory is still cleaned up; the file will be
			// removed on the next cleanup call or after the process exits.
			fmt.Printf("Warning: skipping temp file %s (in use): %v\n", entry.Name(), err)
		}
	}

	return nil
}
