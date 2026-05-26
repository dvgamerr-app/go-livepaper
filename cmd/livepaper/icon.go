package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// makeTrayIcon generates a simple 32×32 PNG icon for the system tray.
func makeTrayIcon() []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	bg := color.RGBA{30, 30, 30, 255}
	accent := color.RGBA{100, 180, 255, 255}

	for y := range size {
		for x := range size {
			img.Set(x, y, bg)
		}
	}

	// Draw a simple "W" shape in accent color
	for y := 6; y < 26; y++ {
		t := float64(y-6) / 20.0
		// left stroke
		img.Set(4, y, accent)
		img.Set(5, y, accent)
		// right stroke
		img.Set(26, y, accent)
		img.Set(27, y, accent)
		// inner V strokes
		if y >= 14 {
			mid := 14 + int(t*8)
			img.Set(mid-1, y, accent)
			img.Set(mid, y, accent)
			img.Set(30-mid, y, accent)
			img.Set(31-mid, y, accent)
		}
		_ = t
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
