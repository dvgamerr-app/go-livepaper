package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true,
	".mov": true, ".webm": true, ".m4v": true, ".flv": true,
}

func isVideoFile(path string) bool {
	return videoExts[strings.ToLower(filepath.Ext(path))]
}

// frameBuf is a mutex-guarded single buffer. The renderer holds the lock
// only during the GDI blit (~<1 ms for 1080p), so the decoder is rarely blocked.
type frameBuf struct {
	mu          sync.Mutex
	data        []byte
	width       int
	height      int
	ready       bool
}

func newFrameBuf(w, h int) *frameBuf {
	return &frameBuf{
		data:   make([]byte, w*h*4),
		width:  w,
		height: h,
	}
}

func (f *frameBuf) write(src []byte) {
	f.mu.Lock()
	copy(f.data, src)
	f.ready = true
	f.mu.Unlock()
}

// renderWith calls fn with the current frame data while holding the lock.
// fn must not retain the slice after returning.
func (f *frameBuf) renderWith(fn func(data []byte, w, h int)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ready {
		fn(f.data, f.width, f.height)
	}
}

// runVideoWallpaper starts video decoding and rendering on the monitor at
// screen offset (x, y), then blocks on the Win32 message loop.
func runVideoWallpaper(videoPath string, x, y, w, h int) error {
	hwnd, progmanRef, err := createDesktopWindow(x, y, w, h)
	if err != nil {
		return fmt.Errorf("desktop window: %w", err)
	}

	buf := newFrameBuf(w, h)
	go decodeVideoFrames(videoPath, w, h, buf)
	go renderFrames(hwnd, buf, progmanRef)

	runMessageLoop()
	return nil
}

// decodeVideoFrames runs ffmpeg with hardware-accelerated decode (-hwaccel auto
// tries DXVA2/D3D11VA/NVDEC/QuickSync before falling back to software) and
// writes raw BGRA frames into buf. Loops the video indefinitely via -stream_loop.
func decodeVideoFrames(videoPath string, w, h int, buf *frameBuf) {
	frameSize := w * h * 4
	frame := make([]byte, frameSize)

	for {
		// -stream_loop -1  : demuxer-level infinite loop (cheaper than process restart)
		// -hwaccel auto    : GPU decode; CPU only touches pipe I/O and GDI blit
		// scale+crop filter: fill-scale then centre-crop to exact canvas size
		cmd := exec.Command("ffmpeg",
			"-stream_loop", "-1",
			"-hwaccel", "auto",
			"-i", videoPath,
			"-vf", fmt.Sprintf(
				"scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
				w, h, w, h,
			),
			"-f", "rawvideo",
			"-pix_fmt", "bgra",
			"-r", "30",
			"-an",
			"pipe:1",
		)
		cmd.Stderr = io.Discard // suppress ffmpeg banner / progress

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Printf("ffmpeg pipe: %v", err)
			return
		}
		if err := cmd.Start(); err != nil {
			log.Printf("ffmpeg not found — install ffmpeg and ensure it is in PATH: %v", err)
			return
		}

		r := bufio.NewReaderSize(stdout, frameSize*2)
		for {
			if _, err := io.ReadFull(r, frame); err != nil {
				break
			}
			buf.write(frame)
		}
		cmd.Wait()
		// Process-level restart as fallback (e.g. if -stream_loop isn't supported).
	}
}

// renderFrames blits the latest decoded frame to the desktop window at 30 fps.
// GetDC/StretchDIBits/ReleaseDC are safe to call from any thread.
// progmanRef is non-zero for Case B: every 300 frames we re-assert z-order
// below Progman in case the shell refreshes and hoists the window.
func renderFrames(hwnd uintptr, buf *frameBuf, progmanRef uintptr) {
	ticker := time.NewTicker(time.Second / 30)
	defer ticker.Stop()

	bmi := bitmapInfoHeader{
		biSize:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		biPlanes:   1,
		biBitCount: 32,
		biWidth:    int32(buf.width),
		biHeight:   int32(-buf.height), // negative = top-down scanlines
	}

	var frame int
	for range ticker.C {
		frame++
		if progmanRef != 0 && frame%300 == 0 {
			// Re-assert bottom of z-order every ~10s in case the shell resets it.
			setWindowPosW.Call(hwnd, hwndBottom,
				0, 0, 0, 0,
				swpNoMove|swpNoSize|swpNoActivate)
		}
		buf.renderWith(func(data []byte, w, h int) {
			hdc, _, _ := getDCW.Call(hwnd)
			if hdc == 0 {
				return
			}
			stretchDIBitsW.Call(
				hdc,
				0, 0, uintptr(w), uintptr(h),
				0, 0, uintptr(w), uintptr(h),
				uintptr(unsafe.Pointer(&data[0])),
				uintptr(unsafe.Pointer(&bmi)),
				dibRgbColors,
				srccopy,
			)
			releaseDCW.Call(hwnd, hdc)
		})
	}
}
