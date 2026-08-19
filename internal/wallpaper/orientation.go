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
	switch orientation {
	case 1:
		return img
	case 2:
		return mirrorHorizontal(img)
	case 3:
		return rotate180(img)
	case 4:
		return mirrorVertical(img)
	case 5:
		return transformOrientation5(img)
	case 6:
		return transformOrientation6(img)
	case 7:
		return transformOrientation7(img)
	case 8:
		return transformOrientation8(img)
	default:
		return img
	}
}

type pixelMapper func(x, y, width, height int) (dstX, dstY int)

// transformPixels applies an orientation mapping using coordinates local to
// each image's bounds. Decoded images usually start at (0,0), but sub-images do
// not, so source and destination offsets must be handled explicitly.
func transformPixels(img image.Image, dstBounds image.Rectangle, mapper pixelMapper) image.Image {
	srcBounds := img.Bounds()
	width, height := srcBounds.Dx(), srcBounds.Dy()
	dst := image.NewRGBA(dstBounds)

	for y := range height {
		for x := range width {
			dstX, dstY := mapper(x, y, width, height)
			dst.Set(
				dstBounds.Min.X+dstX,
				dstBounds.Min.Y+dstY,
				img.At(srcBounds.Min.X+x, srcBounds.Min.Y+y),
			)
		}
	}
	return dst
}

func mirrorHorizontal(img image.Image) image.Image {
	return transformPixels(img, img.Bounds(), func(x, y, width, _ int) (int, int) {
		return width - x - 1, y
	})
}

func mirrorVertical(img image.Image) image.Image {
	return transformPixels(img, img.Bounds(), func(x, y, _, height int) (int, int) {
		return x, height - y - 1
	})
}

func rotate180(img image.Image) image.Image {
	return transformPixels(img, img.Bounds(), func(x, y, width, height int) (int, int) {
		return width - x - 1, height - y - 1
	})
}

func swappedBounds(img image.Image) image.Rectangle {
	bounds := img.Bounds()
	return image.Rect(0, 0, bounds.Dy(), bounds.Dx())
}

func transformOrientation5(img image.Image) image.Image {
	return transformPixels(img, swappedBounds(img), func(x, y, width, _ int) (int, int) {
		return y, width - x - 1
	})
}

func transformOrientation6(img image.Image) image.Image {
	return transformPixels(img, swappedBounds(img), func(x, y, _, height int) (int, int) {
		return height - y - 1, x
	})
}

func transformOrientation7(img image.Image) image.Image {
	return transformPixels(img, swappedBounds(img), func(x, y, _, _ int) (int, int) {
		return y, x
	})
}

func transformOrientation8(img image.Image) image.Image {
	return transformPixels(img, swappedBounds(img), func(x, y, width, height int) (int, int) {
		return height - y - 1, width - x - 1
	})
}
