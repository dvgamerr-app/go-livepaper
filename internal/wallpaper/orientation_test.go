//go:build windows

package wallpaper

import (
	"image"
	"image/color"
	"os"
	"testing"
)

// makeCheckerImage creates a w×h RGBA image with distinct corner colours so
// rotation / mirror tests can verify pixel positions unambiguously.
func makeCheckerImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})              // top-left:     red
	img.Set(w-1, 0, color.RGBA{G: 255, A: 255})            // top-right:    green
	img.Set(0, h-1, color.RGBA{B: 255, A: 255})            // bottom-left:  blue
	img.Set(w-1, h-1, color.RGBA{R: 255, G: 255, A: 255})  // bottom-right: yellow
	return img
}

func TestMirrorHorizontal(t *testing.T) {
	img := makeCheckerImage(4, 3)
	r := mirrorHorizontal(img, 4, 3)

	// Original red (0,0) moves to (3,0); original green (3,0) moves to (0,0).
	if r.At(3, 0) != (color.RGBA{R: 255, A: 255}) {
		t.Error("mirrorHorizontal: expected red at (3,0)")
	}
	if r.At(0, 0) != (color.RGBA{G: 255, A: 255}) {
		t.Error("mirrorHorizontal: expected green at (0,0)")
	}
}

func TestMirrorVertical(t *testing.T) {
	img := makeCheckerImage(4, 3)
	r := mirrorVertical(img, 4, 3)

	// Original red (0,0) moves to (0,2); original blue (0,2) moves to (0,0).
	if r.At(0, 2) != (color.RGBA{R: 255, A: 255}) {
		t.Error("mirrorVertical: expected red at (0,2)")
	}
	if r.At(0, 0) != (color.RGBA{B: 255, A: 255}) {
		t.Error("mirrorVertical: expected blue at (0,0)")
	}
}

func TestRotate180(t *testing.T) {
	img := makeCheckerImage(4, 3)
	r := rotate180(img, 4, 3)

	// Original red (0,0) moves to (3,2); original yellow (3,2) moves to (0,0).
	if r.At(3, 2) != (color.RGBA{R: 255, A: 255}) {
		t.Error("rotate180: expected red at (3,2)")
	}
	if r.At(0, 0) != (color.RGBA{R: 255, G: 255, A: 255}) {
		t.Error("rotate180: expected yellow at (0,0)")
	}
}

func TestTransformOrientation5(t *testing.T) {
	img := makeCheckerImage(4, 3)
	r := transformOrientation5(img, 4, 3)

	if got := r.Bounds(); got != image.Rect(0, 0, 3, 4) {
		t.Errorf("transformOrientation5 bounds = %v, want (0,0)-(3,4)", got)
	}
	// (x=0,y=0) → dst(y, width-x-1) = dst(0, 3)
	if r.At(0, 3) != (color.RGBA{R: 255, A: 255}) {
		t.Error("transformOrientation5: expected red at (0,3)")
	}
}

func TestTransformOrientation6(t *testing.T) {
	img := makeCheckerImage(4, 3)
	r := transformOrientation6(img, 4, 3)

	if got := r.Bounds(); got != image.Rect(0, 0, 3, 4) {
		t.Errorf("transformOrientation6 bounds = %v, want (0,0)-(3,4)", got)
	}
	// (x=0,y=0) → dst(height-y-1, x) = dst(2, 0)
	if r.At(2, 0) != (color.RGBA{R: 255, A: 255}) {
		t.Error("transformOrientation6: expected red at (2,0)")
	}
}

func TestTransformOrientation7(t *testing.T) {
	img := makeCheckerImage(4, 3)
	r := transformOrientation7(img, 4, 3)

	if got := r.Bounds(); got != image.Rect(0, 0, 3, 4) {
		t.Errorf("transformOrientation7 bounds = %v, want (0,0)-(3,4)", got)
	}
	// (x=0,y=0) → dst(y, x) = dst(0, 0)
	if r.At(0, 0) != (color.RGBA{R: 255, A: 255}) {
		t.Error("transformOrientation7: expected red at (0,0)")
	}
}

func TestTransformOrientation8(t *testing.T) {
	img := makeCheckerImage(4, 3)
	r := transformOrientation8(img, 4, 3)

	if got := r.Bounds(); got != image.Rect(0, 0, 3, 4) {
		t.Errorf("transformOrientation8 bounds = %v, want (0,0)-(3,4)", got)
	}
	// (x=0,y=0) → dst(height-y-1, width-x-1) = dst(2, 3)
	if r.At(2, 3) != (color.RGBA{R: 255, A: 255}) {
		t.Error("transformOrientation8: expected red at (2,3)")
	}
}

func TestApplyOrientation_AllCases(t *testing.T) {
	img := makeCheckerImage(4, 3)

	tests := []struct {
		orient    int
		wantW     int
		wantH     int
		sameImage bool
	}{
		{1, 4, 3, true},  // no-op
		{2, 4, 3, false}, // mirrorH
		{3, 4, 3, false}, // rotate180
		{4, 4, 3, false}, // mirrorV
		{5, 3, 4, false}, // orientation5
		{6, 3, 4, false}, // orientation6
		{7, 3, 4, false}, // orientation7
		{8, 3, 4, false}, // orientation8
		{0, 4, 3, true},  // default: returns original
		{9, 4, 3, true},  // default: returns original
	}
	for _, tt := range tests {
		result := applyOrientation(img, tt.orient)
		b := result.Bounds()
		if b.Dx() != tt.wantW || b.Dy() != tt.wantH {
			t.Errorf("applyOrientation(%d): size=%dx%d, want %dx%d",
				tt.orient, b.Dx(), b.Dy(), tt.wantW, tt.wantH)
		}
		if tt.sameImage {
			// Orientation 1 and default must return the exact same interface value.
			if result != image.Image(img) {
				t.Errorf("applyOrientation(%d): expected original image returned", tt.orient)
			}
		}
	}
}

// minimalJPEGWithOrientation6 is a 40-byte JPEG containing only a SOI marker,
// an APP1 segment with EXIF orientation=6, and an EOI marker.
var minimalJPEGWithOrientation6 = []byte{
	0xFF, 0xD8, // SOI
	0xFF, 0xE1, 0x00, 0x22, // APP1 marker + length=34 (includes length field)
	0x45, 0x78, 0x69, 0x66, 0x00, 0x00, // "Exif\0\0"
	// TIFF little-endian header
	0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00, // "II", magic=42, IFD offset=8
	// IFD: 1 entry
	0x01, 0x00, // count=1
	// Entry: tag=0x0112 Orientation, type=SHORT, count=1, value=6
	0x12, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00,
	0x06, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, // next IFD=0
	0xFF, 0xD9, // EOI
}

// minimalJPEGNoOrientationTag has valid EXIF but stores ImageWidth (0x0100)
// instead of Orientation, so exifData.Get(Orientation) returns an error.
var minimalJPEGNoOrientationTag = []byte{
	0xFF, 0xD8,
	0xFF, 0xE1, 0x00, 0x22,
	0x45, 0x78, 0x69, 0x66, 0x00, 0x00,
	0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00,
	0x01, 0x00,
	// tag=0x0100 ImageWidth, type=SHORT, count=1, value=100
	0x00, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00,
	0x64, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0xFF, 0xD9,
}

func TestGetOrientation_NoExif(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "noexif*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, _ = f.WriteString("this is not a jpeg file")
	_, _ = f.Seek(0, 0)

	if got := getOrientation(f); got != 1 {
		t.Errorf("getOrientation (no EXIF) = %d, want 1", got)
	}
}

func TestGetOrientation_WithOrientation6(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "exif_ori6*.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, _ = f.Write(minimalJPEGWithOrientation6)
	_, _ = f.Seek(0, 0)

	if got := getOrientation(f); got != 6 {
		t.Errorf("getOrientation (EXIF orientation=6) = %d, want 6", got)
	}
}

func TestGetOrientation_NoOrientationTag(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "exif_noori*.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, _ = f.Write(minimalJPEGNoOrientationTag)
	_, _ = f.Seek(0, 0)

	if got := getOrientation(f); got != 1 {
		t.Errorf("getOrientation (no Orientation tag) = %d, want 1", got)
	}
}
