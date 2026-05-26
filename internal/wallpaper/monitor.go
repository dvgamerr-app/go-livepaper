package wallpaper

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Resolution struct {
	Width  int32
	Height int32
	X      int32
	Y      int32
}

type Rectangle struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type monitorInfo struct {
	CbSize    uint32
	RcMonitor Rectangle
	RcWork    Rectangle
	DwFlags   uint32
}

type MonitorInfo struct {
	Index      int
	Resolution Resolution
	Rectangle  Rectangle
	Primary    bool
}

func monitorEnumProc(hMonitor, hdcMonitor, lprcMonitor uintptr, dwData *uintptr) uintptr {
	monitors := (*[]MonitorInfo)(unsafe.Pointer(dwData))
	info, err := getMonitorInfoFromHandle(windows.Handle(hMonitor), len(*monitors))
	if err == nil {
		*monitors = append(*monitors, info)
	}
	return 1
}

func getMonitorInfoFromHandle(hMonitor windows.Handle, index int) (MonitorInfo, error) {
	info := MonitorInfo{Index: index}

	var mi monitorInfo
	mi.CbSize = uint32(unsafe.Sizeof(mi))

	ret, _, _ := getMonitorInfoW.Call(
		uintptr(hMonitor),
		uintptr(unsafe.Pointer(&mi)),
	)

	if ret == 0 {
		return info, fmt.Errorf("GetMonitorInfoW failed")
	}

	width := mi.RcMonitor.Right - mi.RcMonitor.Left
	height := mi.RcMonitor.Bottom - mi.RcMonitor.Top
	info.Primary = (mi.DwFlags & 1) != 0
	info.Rectangle = mi.RcMonitor
	info.Resolution = Resolution{
		Width:  width,
		Height: height,
		X:      mi.RcMonitor.Left,
		Y:      mi.RcMonitor.Top,
	}

	return info, nil
}

func getCanvas(monitors []MonitorInfo) (totalWidth, totalHeight int, minX, minY int32) {
	minX = monitors[0].Resolution.X
	minY = monitors[0].Resolution.Y
	maxX := monitors[0].Resolution.X + monitors[0].Resolution.Width
	maxY := monitors[0].Resolution.Y + monitors[0].Resolution.Height

	for _, m := range monitors[1:] {
		res := &m.Resolution
		if res.X < minX {
			minX = res.X
		}
		if res.Y < minY {
			minY = res.Y
		}
		if res.X+res.Width > maxX {
			maxX = res.X + res.Width
		}
		if res.Y+res.Height > maxY {
			maxY = res.Y + res.Height
		}
	}

	totalWidth = int(maxX - minX)
	totalHeight = int(maxY - minY)

	for i := range monitors {
		monitors[i].Resolution.X -= minX
		monitors[i].Resolution.Y -= minY
	}
	return
}

func GetMonitors() (canvasW, canvasH int, monitors []MonitorInfo, vdMinX, vdMinY int32) {
	enumDisplayMonitors.Call(
		0,
		0,
		windows.NewCallback(monitorEnumProc),
		uintptr(unsafe.Pointer(&monitors)),
	)
	canvasW, canvasH, vdMinX, vdMinY = getCanvas(monitors)
	return
}
