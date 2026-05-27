package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/gif"
	_ "image/png"
	_ "golang.org/x/image/webp"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nfnt/resize"
	wp "github.com/dvgamerr/go-livepaper/internal/wallpaper"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type MonitorData struct {
	Index         int     `json:"index"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	X             int     `json:"x"`
	Y             int     `json:"y"`
	PreviewX      int     `json:"previewX"`
	PreviewY      int     `json:"previewY"`
	PreviewWidth  int     `json:"previewWidth"`
	PreviewHeight int     `json:"previewHeight"`
	AspectRatio   float64 `json:"aspectRatio"`
	Primary       bool    `json:"primary"`
}

type MonitorLayoutData struct {
	CanvasWidth  int           `json:"canvasWidth"`
	CanvasHeight int           `json:"canvasHeight"`
	StageWidth   int           `json:"stageWidth"`
	StageHeight  int           `json:"stageHeight"`
	Scale        float64       `json:"scale"`
	Monitors     []MonitorData `json:"monitors"`
}

type WallpaperAssignment struct {
	MonitorIndex int    `json:"monitorIndex"`
	FilePath     string `json:"filePath"`
}

type ProgressEvent struct {
	File     string `json:"file"`
	Progress int    `json:"progress"`
}

type AppService struct {
	app    *application.App
	window *application.WebviewWindow
}

func (s *AppService) WindowMinimise() {
	if s.window != nil {
		s.window.Minimise()
	}
}

func (s *AppService) WindowHide() {
	if s.window != nil {
		s.window.Hide()
	}
}

func (s *AppService) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (s *AppService) GetVersion() string {
	return VERSION
}

func (s *AppService) GetGPUStats() GPUStats {
	return readGPUStats()
}

func (s *AppService) GetMonitors() []MonitorData {
	_, _, monitors, _, _ := wp.GetMonitors()
	result := make([]MonitorData, len(monitors))
	for i, m := range monitors {
		result[i] = MonitorData{
			Index:       m.Index,
			Width:       int(m.Resolution.Width),
			Height:      int(m.Resolution.Height),
			X:           int(m.Resolution.X),
			Y:           int(m.Resolution.Y),
			AspectRatio: monitorAspectRatio(m),
			Primary:     m.Primary,
		}
	}
	return result
}

func (s *AppService) GetMonitorLayout(availWidth, availHeight, labelHeight int) MonitorLayoutData {
	canvasWidth, canvasHeight, monitors, _, _ := wp.GetMonitors()
	layout := MonitorLayoutData{
		CanvasWidth:  canvasWidth,
		CanvasHeight: canvasHeight,
		StageHeight:  labelHeight,
		Scale:        1,
		Monitors:     make([]MonitorData, len(monitors)),
	}

	if len(monitors) == 0 || canvasWidth <= 0 || canvasHeight <= 0 {
		return layout
	}

	if availWidth < 1 {
		availWidth = 1
	}
	usableHeight := availHeight - labelHeight
	if usableHeight < 1 {
		usableHeight = 1
	}

	scaleW := float64(availWidth) / float64(canvasWidth)
	scaleH := float64(usableHeight) / float64(canvasHeight)
	scale := math.Min(scaleW, scaleH)
	if scale > 1 {
		scale = 1
	}
	if scale <= 0 {
		scale = 1
	}

	layout.Scale = scale
	layout.StageWidth = int(math.Round(float64(canvasWidth) * scale))
	layout.StageHeight = int(math.Round(float64(canvasHeight)*scale)) + labelHeight

	for i, m := range monitors {
		layout.layoutMonitors(i, m, scale)
	}

	return layout
}

func (layout *MonitorLayoutData) layoutMonitors(i int, m wp.MonitorInfo, scale float64) {
	width := int(m.Resolution.Width)
	height := int(m.Resolution.Height)
	layout.Monitors[i] = MonitorData{
		Index:         m.Index,
		Width:         width,
		Height:        height,
		X:             int(m.Resolution.X),
		Y:             int(m.Resolution.Y),
		PreviewX:      int(math.Round(float64(m.Resolution.X) * scale)),
		PreviewY:      int(math.Round(float64(m.Resolution.Y) * scale)),
		PreviewWidth:  int(math.Round(float64(m.Resolution.Width) * scale)),
		PreviewHeight: int(math.Round(float64(m.Resolution.Height) * scale)),
		AspectRatio:   monitorAspectRatio(m),
		Primary:       m.Primary,
	}
}

func monitorAspectRatio(m wp.MonitorInfo) float64 {
	if m.Resolution.Height == 0 {
		return 0
	}
	return float64(m.Resolution.Width) / float64(m.Resolution.Height)
}

func (s *AppService) BrowseFile() string {
	result, err := s.app.Dialog.OpenFile().
		SetTitle("Select Wallpaper or Video").
		AddFilter("All Media", "*.jpg;*.jpeg;*.png;*.bmp;*.webp;*.gif;*.mp4;*.mkv;*.avi;*.mov;*.webm;*.m4v").
		AddFilter("Images", "*.jpg;*.jpeg;*.png;*.bmp;*.webp;*.gif").
		AddFilter("Videos", "*.mp4;*.mkv;*.avi;*.mov;*.webm;*.m4v").
		PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return result
}

func (s *AppService) IsVideoFile(filePath string) bool {
	return wp.IsVideoFile(filePath)
}

func (s *AppService) GetThumbnail(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if wp.IsVideoFile(filePath) && ext != ".gif" {
		return videoThumbnail(filePath)
	}
	return imageThumbnail(filePath)
}

func imageThumbnail(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return ""
	}
	thumb := resize.Thumbnail(800, 450, img, resize.Lanczos3)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 88}); err != nil {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func videoThumbnail(filePath string) string {
	seekSec := wp.VideoMidSec(filePath)
	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", seekSec),
		"-i", filePath,
		"-vf", "scale=800:450:force_original_aspect_ratio=decrease,pad=800:450:(ow-iw)/2:(oh-ih)/2",
		"-frames:v", "1",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-q:v", "1",
		"-",
	)
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(out)
}

func (s *AppService) PreprocessVideo(filePath string, w, h int) (string, error) {
	if strings.ToLower(filepath.Ext(filePath)) == ".gif" {
		return s.preprocessGIF(filePath)
	}

	tmpDir := filepath.Join(os.TempDir(), "livepaper")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	out := filepath.Join(tmpDir, fmt.Sprintf("%s_%dx%d.mp4", base, w, h))

	if _, err := os.Stat(out); err == nil {
		s.app.Event.Emit("video:progress", ProgressEvent{File: filePath, Progress: 100})
		return out, nil
	}

	durationUs := getVideoDurationUs(filePath)

	cmd := exec.Command("ffmpeg",
		"-i", filePath,
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", w, h, w, h),
		"-c:v", "libx264", "-crf", "23", "-preset", "fast",
		"-movflags", "+faststart", "-r", "30", "-an",
		"-progress", "pipe:1",
		"-nostats",
		"-y", out,
	)
	cmd.Stderr = io.Discard
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "out_time_us=") {
			us, _ := strconv.ParseInt(strings.TrimPrefix(line, "out_time_us="), 10, 64)
			if durationUs > 0 && us > 0 {
				pct := int(float64(us) / float64(durationUs) * 100)
				if pct > 99 {
					pct = 99
				}
				s.app.Event.Emit("video:progress", ProgressEvent{File: filePath, Progress: pct})
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		os.Remove(out)
		return "", fmt.Errorf("ffmpeg encode: %w", err)
	}

	s.app.Event.Emit("video:progress", ProgressEvent{File: filePath, Progress: 100})
	return out, nil
}

func (s *AppService) preprocessGIF(filePath string) (string, error) {
	tmpDir := filepath.Join(os.TempDir(), "livepaper")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	cfg, _, err := image.DecodeConfig(f)
	f.Close()
	if err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	out := filepath.Join(tmpDir, fmt.Sprintf("%s_%dx%d_gif.mp4", base, cfg.Width, cfg.Height))

	if _, err := os.Stat(out); err == nil {
		s.app.Event.Emit("video:progress", ProgressEvent{File: filePath, Progress: 100})
		return out, nil
	}

	durationUs := getVideoDurationUs(filePath)

	// Preserve original GIF fps and size. H.264 requires even dimensions,
	// so round each axis down to the nearest even pixel if needed.
	cmd := exec.Command("ffmpeg",
		"-i", filePath,
		"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2",
		"-c:v", "libx264", "-crf", "23", "-preset", "fast",
		"-movflags", "+faststart", "-an",
		"-progress", "pipe:1",
		"-nostats",
		"-y", out,
	)
	cmd.Stderr = io.Discard
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "out_time_us=") {
			us, _ := strconv.ParseInt(strings.TrimPrefix(line, "out_time_us="), 10, 64)
			if durationUs > 0 && us > 0 {
				pct := int(float64(us) / float64(durationUs) * 100)
				if pct > 99 {
					pct = 99
				}
				s.app.Event.Emit("video:progress", ProgressEvent{File: filePath, Progress: pct})
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		os.Remove(out)
		return "", fmt.Errorf("ffmpeg gif encode: %w", err)
	}

	s.app.Event.Emit("video:progress", ProgressEvent{File: filePath, Progress: 100})
	return out, nil
}

func getVideoDurationUs(filePath string) int64 {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return int64(f * 1e6)
}

func (s *AppService) CheckDependencies() map[string]bool {
	check := func(cmd string) bool {
		_, err := exec.LookPath(cmd)
		return err == nil
	}
	return map[string]bool{
		"ffmpeg": check("ffmpeg"),
		"ffprobe": check("ffprobe"),
		"mpv":    check("mpv"),
	}
}

func (s *AppService) CleanTempFiles() error {
	return wp.CleanTempDir()
}

func (s *AppService) ResetWallpapers() {
	wp.StopVideoWallpapers()
}

func (s *AppService) ApplyWallpapers(assignments []WallpaperAssignment) error {
	canvasWidth, canvasHeight, monitors, vdMinX, vdMinY := wp.GetMonitors()

	monitorMap := make(map[int]wp.MonitorInfo)
	for _, m := range monitors {
		monitorMap[m.Index] = m
	}

	canvas := wp.CreateBlackCanvas(canvasWidth, canvasHeight)
	var vTargets []wp.VideoTarget
	hasImage := false

	for _, a := range assignments {
		if a.FilePath == "" {
			continue
		}
		m, ok := monitorMap[a.MonitorIndex]
		if !ok {
			continue
		}

		isVideo := wp.IsVideoFile(a.FilePath)

		if isVideo {
			rawX := int(m.Resolution.X) + int(vdMinX)
			rawY := int(m.Resolution.Y) + int(vdMinY)

			// Draw a mid-frame onto the static wallpaper canvas so the background
			// image matches the video when mpv hasn't started yet or after it exits.
			if frame, err := wp.ExtractVideoFrame(a.FilePath, int(m.Resolution.Width), int(m.Resolution.Height)); err == nil {
				draw.Draw(canvas,
					frame.Bounds().Add(image.Pt(int(m.Resolution.X), int(m.Resolution.Y))),
					frame, image.Point{}, draw.Over)
				hasImage = true
			} else {
				log.Printf("monitor %d: frame extract: %v", m.Index+1, err)
			}

			// filePath is already the encoded cached path (set by JS after PreprocessVideo)
			vTargets = append(vTargets, wp.VideoTarget{
				Path: a.FilePath,
				X:    rawX, Y: rawY,
				W: int(m.Resolution.Width), H: int(m.Resolution.Height),
			})
		} else {
			img, err := wp.LoadAndResizeImage(a.FilePath, uint(m.Resolution.Width), uint(m.Resolution.Height))
			if err != nil {
				log.Printf("monitor %d: %v", m.Index+1, err)
				continue
			}
			draw.Draw(canvas,
				img.Bounds().Add(image.Pt(int(m.Resolution.X), int(m.Resolution.Y))),
				img, image.Point{}, draw.Over)
			hasImage = true
		}
	}

	if hasImage || len(vTargets) > 0 {
		if err := wp.SetWallpaperStyle(wp.STYLE_SPAN); err != nil {
			return fmt.Errorf("set style: %w", err)
		}
		outputPath, err := wp.SaveImageAs(canvas, 90)
		if err != nil {
			return fmt.Errorf("save canvas: %w", err)
		}
		if err := wp.SetWallpaper(outputPath); err != nil {
			return fmt.Errorf("set wallpaper: %w", err)
		}
		log.Printf("wallpaper set: %s", outputPath)
	}

	if len(vTargets) > 0 {
		go func() {
			if err := wp.RunVideoWallpapers(vTargets); err != nil {
				log.Printf("video wallpapers: %v", err)
			}
		}()
	}
	return nil
}
