package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/nfnt/resize"
)

func resizeImageToFill(img image.Image, orientation int, width, height uint) (image.Image, image.Point) {
	img = applyOrientation(img, orientation)

	bounds := img.Bounds()
	origWidth := uint(bounds.Dx())
	origHeight := uint(bounds.Dy())

	origRatio := float64(origWidth) / float64(origHeight)
	targetRatio := float64(width) / float64(height)

	var newWidth, newHeight uint
	if origRatio > targetRatio {
		newHeight = height
		newWidth = uint(float64(height) * origRatio)
	} else {
		newWidth = width
		newHeight = uint(float64(width) / origRatio)
	}

	resized := resize.Resize(newWidth, newHeight, img, resize.Lanczos3)

	offsetX := (int(newWidth) - int(width)) / 2
	offsetY := (int(newHeight) - int(height)) / 2

	return resized, image.Point{X: offsetX, Y: offsetY}
}

func loadAndResizeImage(path string, width, height uint) (image.Image, error) {
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

	orientation := getOrientation(file)
	resized, offset := resizeImageToFill(img, orientation, width, height)
	result := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	draw.Draw(result, result.Bounds(), resized, offset, draw.Src)

	return result, nil
}

func createBlackCanvas(width, height int) *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: image.Black}, image.Point{}, draw.Src)
	return canvas
}

func saveImageAs(img image.Image, quality int) (string, error) {
	filename := fmt.Sprintf("wallpaper_%d.jpg", time.Now().UnixNano())

	if !filepath.IsAbs(filename) {
		tempDir, err := getTempDir()
		if err != nil {
			return "", fmt.Errorf("failed to get temp directory: %w", err)
		}

		dir := filepath.Dir(filepath.Join(tempDir, filename))
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}

		filename = filepath.Join(tempDir, filename)
	}

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

func getTempDir() (string, error) {
	tempDir := filepath.Join(os.TempDir(), "livepaper")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	return tempDir, nil
}

func cleanTempDir() error {
	tempDir, err := getTempDir()
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

	for _, entry := range dirEntries {
		if err := os.RemoveAll(filepath.Join(tempDir, entry.Name())); err != nil {
			return fmt.Errorf("failed to remove %s: %w", entry.Name(), err)
		}
	}

	return nil
}
