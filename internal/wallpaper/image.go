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

func GetTempDir() (string, error) {
	tempDir := filepath.Join(os.TempDir(), "livepaper")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	return tempDir, nil
}

func CleanTempDir() error {
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

	var lastErr error
	for _, entry := range dirEntries {
		if err := os.RemoveAll(filepath.Join(tempDir, entry.Name())); err != nil {
			// On Windows a file open by the browser or mpv cannot be deleted yet.
			// Log and continue so the rest of the directory is still cleaned up.
			fmt.Printf("Warning: could not remove temp file %s (in use?): %v\n", entry.Name(), err)
			lastErr = err
		}
	}

	return lastErr
}
