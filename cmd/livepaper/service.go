package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	wp "github.com/dvgamerr/go-livepaper/internal/wallpaper"
	"github.com/nfnt/resize"
	"github.com/pkg/browser"
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
	app      *application.App
	window   *application.WebviewWindow
	encoding sync.Map // key: filePath → *exec.Cmd
}

// ── Video encoder selection ─────────────────────────────────────────────────

var (
	encOnce sync.Once
	encSet  map[string]bool
)

// availableEncoders probes ffmpeg once for the hardware H.264 encoders present
// on this machine and caches the result.
func availableEncoders() map[string]bool {
	encOnce.Do(func() {
		encSet = map[string]bool{}
		cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
		wp.ConfigureBackgroundCommand(cmd)
		out, err := cmd.Output()
		if err != nil {
			return
		}
		text := string(out)
		for _, e := range []string{"h264_nvenc", "h264_qsv", "h264_amf"} {
			if strings.Contains(text, e) {
				encSet[e] = true
			}
		}
	})
	return encSet
}

// pickHWEncoder chooses the best available hardware encoder, preferring the one
// matching the selected GPU adapter's vendor.
func pickHWEncoder(adapter string) string {
	avail := availableEncoders()
	a := strings.ToLower(adapter)
	var pref []string
	switch {
	case strings.Contains(a, "nvidia"), strings.Contains(a, "geforce"),
		strings.Contains(a, "rtx"), strings.Contains(a, "gtx"):
		pref = []string{"h264_nvenc", "h264_qsv", "h264_amf"}
	case strings.Contains(a, "intel"):
		pref = []string{"h264_qsv", "h264_nvenc", "h264_amf"}
	case strings.Contains(a, "amd"), strings.Contains(a, "radeon"):
		pref = []string{"h264_amf", "h264_nvenc", "h264_qsv"}
	default:
		pref = []string{"h264_nvenc", "h264_qsv", "h264_amf"}
	}
	for _, e := range pref {
		if avail[e] {
			return e
		}
	}
	return ""
}

// encoderArgs returns the ffmpeg -c:v arguments honoring the GPU-acceleration
// setting, falling back to CPU libx264 when hardware encode is off or absent.
func encoderArgs() []string {
	s := getSettings()
	if s.GPUAcceleration {
		switch pickHWEncoder(s.GPUAdapter) {
		case "h264_nvenc":
			return []string{"-c:v", "h264_nvenc", "-preset", "p4", "-rc", "vbr", "-cq", "23", "-b:v", "0"}
		case "h264_qsv":
			return []string{"-c:v", "h264_qsv", "-global_quality", "23"}
		case "h264_amf":
			return []string{"-c:v", "h264_amf", "-quality", "balanced", "-rc", "cqp", "-qp_i", "23", "-qp_p", "23"}
		}
	}
	return []string{"-c:v", "libx264", "-crf", "23", "-preset", "fast"}
}

// mpvArgsFromSettings derives mpv flags for GPU adapter selection and a soft
// memory cap from the current settings.
func mpvArgsFromSettings() []string {
	s := getSettings()
	var args []string
	// Hardware decoding flag. Appended after the spawner's default "--hwdec=auto"
	// so this value wins (mpv: last flag takes effect). When off, also switch to
	// lightweight render settings to reduce GPU rendering load noticeably.
	if s.GPUAcceleration {
		args = append(args, "--hwdec=auto")
	} else {
		args = append(args, "--hwdec=no",
			"--scale=bilinear", "--cscale=bilinear", "--dscale=bilinear",
			"--video-sync=audio", "--tone-mapping=clip")
	}
	if s.GPUAdapter != "" {
		args = append(args, "--d3d11-adapter="+s.GPUAdapter)
	}
	if s.VRAMCapMB > 0 {
		args = append(args, fmt.Sprintf("--demuxer-max-bytes=%dMiB", s.VRAMCapMB))
		args = append(args, fmt.Sprintf("--demuxer-max-back-bytes=%dMiB", maxInt(s.VRAMCapMB/2, 32)))
	}
	return args
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ToggleVideoPause flips manual pause on all running video wallpapers and
// returns the new paused state. Used by the play/pause hotkey and UI.
func (s *AppService) ToggleVideoPause() bool {
	return toggleManualPause()
}

func (s *AppService) CancelEncoding(filePath string) {
	if v, ok := s.encoding.Load(filePath); ok {
		if cmd := v.(*exec.Cmd); cmd.Process != nil {
			cmd.Process.Kill()
		}
	}
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

func (s *AppService) WindowShow() {
	if s.window != nil {
		s.window.Show()
		s.window.Focus()
	}
}

func (s *AppService) OpenExternal(url string) error {
	return browser.OpenURL(url)
}

func (s *AppService) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func extFromContentType(ct string) string {
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "mp4"), strings.Contains(ct, "video/mp4"):
		return ".mp4"
	case strings.Contains(ct, "webm"):
		return ".webm"
	case strings.Contains(ct, "x-matroska"), strings.Contains(ct, "mkv"):
		return ".mkv"
	case strings.Contains(ct, "quicktime"):
		return ".mov"
	default:
		return ".jpg"
	}
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "wallpaper"
	}
	return b.String()
}

// DownloadToTemp fetches a community wallpaper from the backend (with the user's
// bearer token) into the temp folder and returns the local path so the normal
// apply flow can use it. Returns "premium_required" when the server gates it.
func (s *AppService) DownloadToTemp(url, token, id string) (string, error) {
	dir := filepath.Join(os.TempDir(), "livepaper", "discover")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusPaymentRequired:
		return "", fmt.Errorf("premium_required")
	case http.StatusUnauthorized:
		return "", fmt.Errorf("unauthorized")
	default:
		return "", fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	out := filepath.Join(dir, sanitizeName(id)+extFromContentType(resp.Header.Get("Content-Type")))
	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(out)
		return "", err
	}
	f.Close()
	return out, nil
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
	wp.ConfigureBackgroundCommand(cmd)
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

	ffArgs := []string{
		"-i", filePath,
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", w, h, w, h),
	}
	ffArgs = append(ffArgs, encoderArgs()...)
	ffArgs = append(ffArgs,
		"-movflags", "+faststart", "-r", "30", "-an",
		"-progress", "pipe:1",
		"-nostats",
		"-y", out,
	)
	cmd := exec.Command("ffmpeg", ffArgs...)
	wp.ConfigureBackgroundCommand(cmd)
	cmd.Stderr = io.Discard
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	s.encoding.Store(filePath, cmd)
	defer s.encoding.Delete(filePath)

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
	ffArgs := []string{
		"-i", filePath,
		"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2",
	}
	ffArgs = append(ffArgs, encoderArgs()...)
	ffArgs = append(ffArgs,
		"-movflags", "+faststart", "-an",
		"-progress", "pipe:1",
		"-nostats",
		"-y", out,
	)
	cmd := exec.Command("ffmpeg", ffArgs...)
	wp.ConfigureBackgroundCommand(cmd)
	cmd.Stderr = io.Discard
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	s.encoding.Store(filePath, cmd)
	defer s.encoding.Delete(filePath)

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
	wp.ConfigureBackgroundCommand(cmd)
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
		"ffmpeg":  check("ffmpeg"),
		"ffprobe": check("ffprobe"),
		"mpv":     check("mpv"),
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
		wp.SetExtraMpvArgs(mpvArgsFromSettings())
		go func() {
			if err := wp.RunVideoWallpapers(vTargets); err != nil {
				log.Printf("video wallpapers: %v", err)
			}
		}()
	}
	return nil
}
