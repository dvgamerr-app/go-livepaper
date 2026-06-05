//go:build windows

package wallpaper

import (
	"testing"
)

func TestGetCanvas_SingleMonitor(t *testing.T) {
	monitors := []MonitorInfo{
		{
			Index:   0,
			Primary: true,
			Resolution: Resolution{
				Width: 1920, Height: 1080,
				X: 0, Y: 0,
			},
		},
	}

	w, h, minX, minY := getCanvas(monitors)

	if w != 1920 {
		t.Errorf("totalWidth = %d, want 1920", w)
	}
	if h != 1080 {
		t.Errorf("totalHeight = %d, want 1080", h)
	}
	if minX != 0 {
		t.Errorf("minX = %d, want 0", minX)
	}
	if minY != 0 {
		t.Errorf("minY = %d, want 0", minY)
	}
	// Coordinates should remain normalised to 0.
	if monitors[0].Resolution.X != 0 || monitors[0].Resolution.Y != 0 {
		t.Errorf("monitor[0] origin = (%d,%d), want (0,0)",
			monitors[0].Resolution.X, monitors[0].Resolution.Y)
	}
}

func TestGetCanvas_TwoMonitorsSideBySide(t *testing.T) {
	monitors := []MonitorInfo{
		{Index: 0, Primary: true, Resolution: Resolution{Width: 1920, Height: 1080, X: 0, Y: 0}},
		{Index: 1, Resolution: Resolution{Width: 1920, Height: 1080, X: 1920, Y: 0}},
	}

	w, h, minX, minY := getCanvas(monitors)

	if w != 3840 {
		t.Errorf("totalWidth = %d, want 3840", w)
	}
	if h != 1080 {
		t.Errorf("totalHeight = %d, want 1080", h)
	}
	if minX != 0 || minY != 0 {
		t.Errorf("min origin = (%d,%d), want (0,0)", minX, minY)
	}
	// Second monitor X should remain 1920 after normalisation.
	if monitors[1].Resolution.X != 1920 {
		t.Errorf("monitor[1].X = %d, want 1920", monitors[1].Resolution.X)
	}
}

func TestGetCanvas_NegativeOrigin(t *testing.T) {
	// Primary monitor at (-1920,0) and secondary at (0,0) — common for left-of-primary layout.
	monitors := []MonitorInfo{
		{Index: 0, Resolution: Resolution{Width: 1920, Height: 1080, X: -1920, Y: 0}},
		{Index: 1, Primary: true, Resolution: Resolution{Width: 2560, Height: 1440, X: 0, Y: 0}},
	}

	w, h, minX, minY := getCanvas(monitors)

	if w != 1920+2560 {
		t.Errorf("totalWidth = %d, want %d", w, 1920+2560)
	}
	if h != 1440 {
		t.Errorf("totalHeight = %d, want 1440", h)
	}
	if minX != -1920 {
		t.Errorf("minX = %d, want -1920", minX)
	}
	if minY != 0 {
		t.Errorf("minY = %d, want 0", minY)
	}
	// After normalisation, monitor[0].X should be 0 (was -1920, shifted by minX=-1920).
	if monitors[0].Resolution.X != 0 {
		t.Errorf("monitor[0].X after normalise = %d, want 0", monitors[0].Resolution.X)
	}
	// monitor[1].X should become 1920 (was 0, shifted by -(-1920)=+1920).
	if monitors[1].Resolution.X != 1920 {
		t.Errorf("monitor[1].X after normalise = %d, want 1920", monitors[1].Resolution.X)
	}
}

func TestGetCanvas_VerticalStack(t *testing.T) {
	monitors := []MonitorInfo{
		{Index: 0, Primary: true, Resolution: Resolution{Width: 1920, Height: 1080, X: 0, Y: 0}},
		{Index: 1, Resolution: Resolution{Width: 1920, Height: 1080, X: 0, Y: 1080}},
	}

	w, h, minX, minY := getCanvas(monitors)

	if w != 1920 {
		t.Errorf("totalWidth = %d, want 1920", w)
	}
	if h != 2160 {
		t.Errorf("totalHeight = %d, want 2160", h)
	}
	if minX != 0 || minY != 0 {
		t.Errorf("min origin = (%d,%d), want (0,0)", minX, minY)
	}
}

func TestGetCanvas_ThreeMonitors(t *testing.T) {
	monitors := []MonitorInfo{
		{Index: 0, Resolution: Resolution{Width: 1920, Height: 1080, X: 0, Y: 0}},
		{Index: 1, Resolution: Resolution{Width: 1920, Height: 1080, X: 1920, Y: 0}},
		{Index: 2, Resolution: Resolution{Width: 1920, Height: 1080, X: 3840, Y: 0}},
	}

	w, h, minX, minY := getCanvas(monitors)

	if w != 5760 {
		t.Errorf("totalWidth = %d, want 5760", w)
	}
	if h != 1080 {
		t.Errorf("totalHeight = %d, want 1080", h)
	}
	if minX != 0 || minY != 0 {
		t.Errorf("min origin = (%d,%d), want (0,0)", minX, minY)
	}
}

// TestGetCanvas_SecondaryLowerXAndY covers the `minX = res.X` and `minY = res.Y`
// update branches inside the loop, which are only reachable when a later
// monitor in the slice has a smaller origin than the first.
func TestGetCanvas_SecondaryLowerXAndY(t *testing.T) {
	// monitor[0] at (1920,100), monitor[1] at (0,0)
	// → loop sees X=0 < minX=1920 and Y=0 < minY=100 → updates both
	monitors := []MonitorInfo{
		{Index: 0, Primary: true, Resolution: Resolution{Width: 1920, Height: 1080, X: 1920, Y: 100}},
		{Index: 1, Resolution: Resolution{Width: 1920, Height: 1080, X: 0, Y: 0}},
	}

	w, h, minX, minY := getCanvas(monitors)

	if w != 3840 {
		t.Errorf("totalWidth = %d, want 3840", w)
	}
	if h != 1180 { // maxY = max(100+1080, 0+1080) = 1180; minY=0 → 1180
		t.Errorf("totalHeight = %d, want 1180", h)
	}
	if minX != 0 {
		t.Errorf("minX = %d, want 0", minX)
	}
	if minY != 0 {
		t.Errorf("minY = %d, want 0", minY)
	}
	// After normalisation: monitor[0].X should be 1920 (was 1920, minX=0 → unchanged).
	if monitors[0].Resolution.X != 1920 {
		t.Errorf("monitor[0].X after normalise = %d, want 1920", monitors[0].Resolution.X)
	}
	// monitor[1].X stays at 0.
	if monitors[1].Resolution.X != 0 {
		t.Errorf("monitor[1].X after normalise = %d, want 0", monitors[1].Resolution.X)
	}
}

func TestGetCanvas_DifferentHeights(t *testing.T) {
	// Monitor with larger vertical extent determines the canvas height.
	monitors := []MonitorInfo{
		{Index: 0, Primary: true, Resolution: Resolution{Width: 1920, Height: 1080, X: 0, Y: 0}},
		{Index: 1, Resolution: Resolution{Width: 2560, Height: 1440, X: 1920, Y: 0}},
	}

	w, h, _, _ := getCanvas(monitors)

	if w != 1920+2560 {
		t.Errorf("totalWidth = %d, want %d", w, 1920+2560)
	}
	if h != 1440 {
		t.Errorf("totalHeight = %d, want 1440", h)
	}
}
