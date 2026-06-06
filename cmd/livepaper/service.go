package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
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
		// Use raw byte values — the "MiB" IEC suffix is not recognised by all
		// mpv versions and causes an immediate exit-status-2 argument error.
		args = append(args, fmt.Sprintf("--demuxer-max-bytes=%d", int64(s.VRAMCapMB)*1024*1024))
		args = append(args, fmt.Sprintf("--demuxer-max-back-bytes=%d", int64(maxInt(s.VRAMCapMB/2, 32))*1024*1024))
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

func (s *AppService) WindowToggleMaximise() {
	if s.window != nil {
		s.window.ToggleMaximise()
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

// DownloadToTemp fetches a community wallpaper (premium-gated) into the app's
// install directory under a "data" sub-folder. The file is saved without
// an extension to discourage direct use outside the app.
// Returns "premium_required" when the server rejects with 402.
func (s *AppService) DownloadToTemp(url, token, id string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(filepath.Dir(exe), "data")
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

	// No extension — hide file type from the filesystem
	out := filepath.Join(dir, sanitizeName(id))
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

func (s *AppService) BrowseFiles() string {
	results, err := s.app.Dialog.OpenFile().
		SetTitle("Select Wallpapers or Videos").
		AddFilter("All Media", "*.jpg;*.jpeg;*.png;*.bmp;*.webp;*.gif;*.mp4;*.mkv;*.avi;*.mov;*.webm;*.m4v").
		AddFilter("Images", "*.jpg;*.jpeg;*.png;*.bmp;*.webp;*.gif").
		AddFilter("Videos", "*.mp4;*.mkv;*.avi;*.mov;*.webm;*.m4v").
		PromptForMultipleSelection()
	if err != nil || len(results) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(results)
	return string(b)
}

func (s *AppService) IsVideoFile(filePath string) bool {
	return wp.IsVideoFile(filePath)
}

// thumbMaxDim caps the longest side of preview thumbnails. Preview blocks are
// small, so this keeps the encoded data URL light and — combined with the disk
// cache below — lets restore/re-render skip decoding the full-resolution source.
const thumbMaxDim = 512

// GetThumbnail returns a generic preview thumbnail (16:9 bound) for a file.
// Used by gallery/discover cards where no specific monitor is targeted.
func (s *AppService) GetThumbnail(filePath string) string {
	return generateThumbnail(filePath, thumbMaxDim, thumbMaxDim*9/16)
}

// GetMonitorThumbnail returns a preview thumbnail bounded to the target
// monitor's aspect ratio, capped at thumbMaxDim on the longest side. Smaller
// monitors get smaller thumbnails, and every result is cached to disk keyed by
// the source file, its mtime, and the bounding box — so repeated previews
// (restore, window resize, re-render) read a small cached JPEG instead of
// re-decoding and re-scaling the original.
func (s *AppService) GetMonitorThumbnail(filePath string, w, h int) string {
	boxW, boxH := thumbBox(w, h)
	return generateThumbnail(filePath, boxW, boxH)
}

// thumbBox scales a monitor's (w,h) so the longest side equals thumbMaxDim,
// preserving aspect. Falls back to a 16:9 box for invalid input.
func thumbBox(w, h int) (int, int) {
	if w <= 0 || h <= 0 {
		return thumbMaxDim, thumbMaxDim * 9 / 16
	}
	if w >= h {
		return thumbMaxDim, int(math.Round(float64(thumbMaxDim) * float64(h) / float64(w)))
	}
	return int(math.Round(float64(thumbMaxDim) * float64(w) / float64(h))), thumbMaxDim
}

func generateThumbnail(filePath string, boxW, boxH int) string {
	// Pre-generate the animated GIF preview for videos in the background so it
	// is ready by the time the user hovers the card — never generated on hover.
	warmAnimatedThumbnail(filePath)

	cachePath := thumbCachePath(filePath, boxW, boxH)
	if data := readThumbCache(cachePath, filePath); data != nil {
		return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)
	}

	var data []byte
	ext := strings.ToLower(filepath.Ext(filePath))
	if wp.IsVideoFile(filePath) && ext != ".gif" {
		data = videoThumbnailBytes(filePath, boxW, boxH)
	} else {
		data = imageThumbnailBytes(filePath, boxW, boxH)
	}
	if len(data) == 0 {
		return ""
	}
	writeThumbCache(cachePath, data)
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)
}

// cacheKey hashes a source path (and optional discriminators) into a collision-
// free cache filename. Delegates to wp.CacheKey so the tray and CLI encode paths
// produce identical cache filenames and share results.
func cacheKey(parts ...string) string {
	return wp.CacheKey(parts...)
}

// thumbCachePath derives the on-disk cache location for a thumbnail. The key
// folds in the source path and bounding box; mtime is validated on read so a
// re-encoded source invalidates the cache.
func thumbCachePath(filePath string, boxW, boxH int) string {
	dir := filepath.Join(os.TempDir(), "livepaper", "thumbs")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, cacheKey(filePath, fmt.Sprintf("%dx%d", boxW, boxH))+".jpg")
}

// readThumbCache returns cached thumbnail bytes when the cache file exists and
// is at least as new as the source. Returns nil to signal a cache miss.
func readThumbCache(cachePath, srcPath string) []byte {
	ci, err := os.Stat(cachePath)
	if err != nil {
		return nil
	}
	if si, err := os.Stat(srcPath); err == nil && si.ModTime().After(ci.ModTime()) {
		return nil // source changed since the cache was written
	}
	data, err := os.ReadFile(cachePath)
	if err != nil || len(data) == 0 {
		return nil
	}
	return data
}

func writeThumbCache(cachePath string, data []byte) {
	_ = os.WriteFile(cachePath, data, 0644)
}

func imageThumbnailBytes(filePath string, boxW, boxH int) []byte {
	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	thumb := resize.Thumbnail(uint(boxW), uint(boxH), img, resize.Lanczos3)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 88}); err != nil {
		return nil
	}
	return buf.Bytes()
}

func videoThumbnailBytes(filePath string, boxW, boxH int) []byte {
	seekSec := wp.VideoMidSec(filePath)
	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", seekSec),
		"-i", filePath,
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", boxW, boxH),
		"-frames:v", "1",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-q:v", "2",
		"-",
	)
	wp.ConfigureBackgroundCommand(cmd)
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	return out
}

// gifLocks serializes GIF generation per output file so an eager background
// warm and an on-hover request never run ffmpeg twice for the same video.
var (
	gifLocksMu sync.Mutex
	gifLocks   = map[string]*sync.Mutex{}
)

func gifLock(key string) *sync.Mutex {
	gifLocksMu.Lock()
	defer gifLocksMu.Unlock()
	m, ok := gifLocks[key]
	if !ok {
		m = &sync.Mutex{}
		gifLocks[key] = m
	}
	return m
}

// warmAnimatedThumbnail kicks off GIF-preview generation in the background for
// video files, so the preview is cached before the user hovers the card.
func warmAnimatedThumbnail(filePath string) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if !wp.IsVideoFile(filePath) || ext == ".gif" {
		return
	}
	go buildAnimatedThumbnail(filePath)
}

// buildAnimatedThumbnail returns the cached GIF-preview bytes for a video,
// generating and caching it on first call. Concurrent callers for the same file
// are serialized so ffmpeg runs at most once.
func buildAnimatedThumbnail(filePath string) []byte {
	ext := strings.ToLower(filepath.Ext(filePath))
	if !wp.IsVideoFile(filePath) || ext == ".gif" {
		return nil
	}

	tmpDir := filepath.Join(os.TempDir(), "livepaper")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil
	}
	outGif := filepath.Join(tmpDir, cacheKey(filePath)+"_preview.gif")

	lock := gifLock(outGif)
	lock.Lock()
	defer lock.Unlock()

	// Return cached file if it already exists
	if data, err := os.ReadFile(outGif); err == nil && len(data) > 0 {
		return data
	}

	seekSec := wp.VideoMidSec(filePath)
	// Single-pass palette GIF: split → palettegen + paletteuse
	filter := "fps=10,scale=320:-2:force_original_aspect_ratio=decrease,split[s0][s1];[s0]palettegen[p];[s1][p]paletteuse"
	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", seekSec),
		"-i", filePath,
		"-t", "2.5",
		"-vf", filter,
		"-loop", "0",
		"-y", outGif,
	)
	wp.ConfigureBackgroundCommand(cmd)
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil
	}
	data, err := os.ReadFile(outGif)
	if err != nil || len(data) == 0 {
		return nil
	}
	return data
}

// GetAnimatedThumbnail returns the animated GIF preview for a video file. The
// GIF is normally already cached from warmAnimatedThumbnail (triggered at
// thumbnail time), so this returns instantly without generating on hover.
func (s *AppService) GetAnimatedThumbnail(filePath string) string {
	data := buildAnimatedThumbnail(filePath)
	if len(data) == 0 {
		return ""
	}
	return "data:image/gif;base64," + base64.StdEncoding.EncodeToString(data)
}

func (s *AppService) PreprocessVideo(filePath string, w, h int) (string, error) {
	if strings.ToLower(filepath.Ext(filePath)) == ".gif" {
		return s.preprocessGIF(filePath)
	}

	tmpDir := filepath.Join(os.TempDir(), "livepaper")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}
	out := filepath.Join(tmpDir, fmt.Sprintf("%s_%dx%d.mp4", cacheKey(filePath), w, h))

	if _, err := os.Stat(out); err == nil {
		if wp.IsPlayableVideo(out) {
			s.app.Event.Emit("video:progress", ProgressEvent{File: filePath, Progress: 100})
			return out, nil
		}
		// Stale/corrupt cache from an interrupted encode — discard and re-encode
		// so we never hand a truncated file to mpv/ffmpeg later.
		os.Remove(out)
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

	out := filepath.Join(tmpDir, fmt.Sprintf("%s_%dx%d_gif.mp4", cacheKey(filePath), cfg.Width, cfg.Height))

	if _, err := os.Stat(out); err == nil {
		if wp.IsPlayableVideo(out) {
			s.app.Event.Emit("video:progress", ProgressEvent{File: filePath, Progress: 100})
			return out, nil
		}
		// Stale/corrupt cache from an interrupted encode — discard and re-encode.
		os.Remove(out)
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

// ── Admin API helpers ────────────────────────────────────────────────────────

const adminAPIBase = "https://sso.dvgamerr.app"

func adminDo(method, url, token, contentType string, body io.Reader, contentLength int64, timeout ...time.Duration) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if contentLength >= 0 {
		req.ContentLength = contentLength
	}
	t := 30 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}
	client := &http.Client{Timeout: t}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

// AdminListWallpapers returns JSON array of all wallpapers from admin API.
func (s *AppService) AdminListWallpapers(token string) string {
	data, err := adminDo("GET", adminAPIBase+"/api/admin/wallpapers", token, "", nil, -1)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(b)
	}
	return string(data)
}

// generateUploadThumbnailBytes creates a thumbnail for upload:
// - JPEG (max 1920×1080) for images
// - animated GIF (480px wide, 3s) for videos
func generateUploadThumbnailBytes(filePath string) ([]byte, string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if wp.IsVideoFile(filePath) && ext != ".gif" {
		data, err := makeUploadGIF(filePath)
		return data, "image/gif", err
	}
	if ext == ".gif" {
		data, err := os.ReadFile(filePath)
		return data, "image/gif", err
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, "", err
	}
	thumb := resize.Thumbnail(1920, 1080, img, resize.Lanczos3)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 88}); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/jpeg", nil
}

func makeUploadGIF(filePath string) ([]byte, error) {
	tmpDir := filepath.Join(os.TempDir(), "livepaper")
	os.MkdirAll(tmpDir, 0755)
	out := filepath.Join(tmpDir, cacheKey(filePath)+"_upload_thumb.gif")
	if data, err := os.ReadFile(out); err == nil && len(data) > 0 {
		return data, nil
	}
	seekSec := wp.VideoMidSec(filePath)
	filter := "fps=10,scale=480:-2:force_original_aspect_ratio=decrease,split[s0][s1];[s0]palettegen[p];[s1][p]paletteuse"
	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", seekSec),
		"-i", filePath,
		"-t", "3",
		"-vf", filter,
		"-loop", "0",
		"-y", out,
	)
	wp.ConfigureBackgroundCommand(cmd)
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(out)
}

// AdminUploadWallpaper runs the full 3-step upload: POST metadata → PUT thumbnail → PUT original.
func (s *AppService) AdminUploadWallpaper(token, filePath, title, tier string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	isVid := wp.IsVideoFile(filePath) && ext != ".gif"

	contentTypeMap := map[string]string{
		".mp4": "video/mp4", ".webm": "video/webm", ".mov": "video/quicktime",
		".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
		".webp": "image/webp", ".gif": "image/gif",
	}
	origCT, ok := contentTypeMap[ext]
	if !ok {
		origCT = "application/octet-stream"
	}

	thumbBytes, thumbCT, err := generateUploadThumbnailBytes(filePath)
	if err != nil {
		return "", fmt.Errorf("thumbnail: %w", err)
	}

	// Step 1: create metadata row
	createPayload, _ := json.Marshal(map[string]interface{}{
		"title": title, "tier": tier,
		"contentType": origCT, "thumbnailContentType": thumbCT,
		"isVideo": isVid,
	})
	resp, err := adminDo("POST", adminAPIBase+"/api/admin/wallpapers", token, "application/json", bytes.NewReader(createPayload), int64(len(createPayload)))
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	var meta struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &meta); err != nil || meta.ID == "" {
		return "", fmt.Errorf("create response: %s", string(resp))
	}
	id := meta.ID

	// Step 2: upload thumbnail
	if _, err := adminDo("PUT", fmt.Sprintf("%s/api/admin/wallpapers/%s/upload?type=thumbnail", adminAPIBase, id), token, thumbCT, bytes.NewReader(thumbBytes), int64(len(thumbBytes)), 600*time.Second); err != nil {
		return id, fmt.Errorf("thumbnail upload: %w", err)
	}

	// Step 3: upload original (streamed)
	f, err := os.Open(filePath)
	if err != nil {
		return id, fmt.Errorf("open original: %w", err)
	}
	defer f.Close()
	fi, _ := f.Stat()
	if _, err := adminDo("PUT", fmt.Sprintf("%s/api/admin/wallpapers/%s/upload?type=original", adminAPIBase, id), token, origCT, f, fi.Size(), 600*time.Second); err != nil {
		return id, fmt.Errorf("original upload: %w", err)
	}

	return id, nil
}

// AdminReplaceFile re-uploads either thumbnail or original for an existing wallpaper.
func (s *AppService) AdminReplaceFile(token, id, uploadType, filePath string) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	var data []byte
	var ct string
	var err error

	if uploadType == "thumbnail" {
		data, ct, err = generateUploadThumbnailBytes(filePath)
	} else {
		data, err = os.ReadFile(filePath)
		ctMap := map[string]string{
			".mp4": "video/mp4", ".webm": "video/webm",
			".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
			".webp": "image/webp", ".gif": "image/gif",
		}
		ct = ctMap[ext]
		if ct == "" {
			ct = "application/octet-stream"
		}
	}
	if err != nil {
		return err
	}
	_, err = adminDo("PUT", fmt.Sprintf("%s/api/admin/wallpapers/%s/upload?type=%s", adminAPIBase, id, uploadType), token, ct, bytes.NewReader(data), int64(len(data)), 600*time.Second)
	return err
}

// AdminPatchWallpaper sends a PATCH request with arbitrary JSON body.
func (s *AppService) AdminPatchWallpaper(token, id, body string) error {
	_, err := adminDo("PATCH", fmt.Sprintf("%s/api/admin/wallpapers/%s", adminAPIBase, id), token, "application/json", strings.NewReader(body), int64(len(body)))
	return err
}

// AdminDeleteWallpaper deletes a wallpaper and purges R2 assets.
func (s *AppService) AdminDeleteWallpaper(token, id string) error {
	_, err := adminDo("DELETE", fmt.Sprintf("%s/api/admin/wallpapers/%s?purge=true", adminAPIBase, id), token, "", nil, -1)
	return err
}
