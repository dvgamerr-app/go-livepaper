package wallpaper

import (
	"image"
	"os"

	"github.com/rwcarlsen/goexif/exif"
)

// getOrientation reads the EXIF orientation tag, returning 1 (no rotation) when
// the image has no EXIF block or the tag is absent/unreadable. This is a
// best-effort, non-fatal read: most images (PNG, WebP, screenshots, generated
// JPEGs) legitimately carry no EXIF, so failures are expected and stay silent.
func getOrientation(file *os.File) int {
	exifData, err := exif.Decode(file)
	if err != nil {
		return 1
	}
	if data, err := exifData.Get(exif.Orientation); err == nil {
		if val, err := data.Int(0); err == nil {
			return val
		}
	}
	return 1
}

func applyOrientation(img image.Image, orientation int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	switch orientation {
	case 1:
		return img
	case 2:
		return mirrorHorizontal(img, width, height)
	case 3:
		return rotate180(img, width, height)
	case 4:
		return mirrorVertical(img, width, height)
	case 5:
		return transformOrientation5(img, width, height)
	case 6:
		return transformOrientation6(img, width, height)
	case 7:
		return transformOrientation7(img, width, height)
	case 8:
		return transformOrientation8(img, width, height)
	default:
		return img
	}
}

func mirrorHorizontal(img image.Image, width, height int) image.Image {
	dst := image.NewRGBA(img.Bounds())
	for y := range height {
		for x := range width {
			dst.Set(width-x-1, y, img.At(x, y))
		}
	}
	return dst
}

func mirrorVertical(img image.Image, width, height int) image.Image {
	dst := image.NewRGBA(img.Bounds())
	for y := range height {
		for x := range width {
			dst.Set(x, height-y-1, img.At(x, y))
		}
	}
	return dst
}

func rotate180(img image.Image, width, height int) image.Image {
	dst := image.NewRGBA(img.Bounds())
	for y := range height {
		for x := range width {
			dst.Set(width-x-1, height-y-1, img.At(x, y))
		}
	}
	return dst
}

func transformOrientation5(img image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, height, width))
	for y := range height {
		for x := range width {
			dst.Set(y, width-x-1, img.At(x, y))
		}
	}
	return dst
}

func transformOrientation6(img image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, height, width))
	for y := range height {
		for x := range width {
			dst.Set(height-y-1, x, img.At(x, y))
		}
	}
	return dst
}

func transformOrientation7(img image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, height, width))
	for y := range height {
		for x := range width {
			dst.Set(y, x, img.At(x, y))
		}
	}
	return dst
}

func transformOrientation8(img image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, height, width))
	for y := range height {
		for x := range width {
			dst.Set(height-y-1, width-x-1, img.At(x, y))
		}
	}
	return dst
}
