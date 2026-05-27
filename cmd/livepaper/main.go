package main

import (
	"fmt"
	"image"
	"image/draw"
	"log"
	"strconv"

	"github.com/alexflint/go-arg"
	wp "github.com/dvgamerr/go-livepaper/internal/wallpaper"
)

var VERSION = "dev"

func (Args) Version() string {
	return fmt.Sprintf("livepaper %s", VERSION)
}

func init() {
	log.SetFlags(log.Lshortfile | log.Ltime)
}

type Args struct {
	Monitor   []string `arg:"-m,--monitor" help:"Target monitor numbers in wallpaper order, 1-based (e.g. -m 1 -m 2)"`
	Clean     bool     `arg:"-c,--clean" help:"Clean all temporary files"`
	Wallpaper []string `arg:"positional" help:"Wallpaper is a list of file paths to wallpaper images"`
}

var args Args

func selectMonitors(monitors []wp.MonitorInfo, selected []string) ([]wp.MonitorInfo, error) {
	if len(selected) == 0 {
		return monitors, nil
	}

	targets := make([]wp.MonitorInfo, 0, len(selected))
	seen := make(map[int]struct{}, len(selected))

	for _, raw := range selected {
		index, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid monitor %q: must be a number", raw)
		}
		if index < 1 || index > len(monitors) {
			return nil, fmt.Errorf("invalid monitor %d: available monitors are 1-%d", index, len(monitors))
		}
		if _, ok := seen[index]; ok {
			return nil, fmt.Errorf("duplicate monitor %d", index)
		}

		seen[index] = struct{}{}
		targets = append(targets, monitors[index-1])
	}

	return targets, nil
}

func main() {
	arg.MustParse(&args)

	if args.Clean {
		if err := wp.CleanTempDir(); err != nil {
			log.Printf("Error cleaning temp directory: %v\n", err)
		} else {
			log.Println("Temporary files cleaned successfully")
			return
		}
	}

	if len(args.Wallpaper) == 0 && len(args.Monitor) == 0 {
		if !checkSingleInstance() {
			return
		}
		runTrayApp()
		return
	}

	if len(args.Wallpaper) == 0 {
		log.Fatalf("No wallpaper specified. Please provide at least one wallpaper image path.")
	}
	if len(args.Wallpaper) != len(args.Monitor) && len(args.Monitor) > 0 {
		log.Fatalf("Invalid arguments: monitors (%d) must match of wallpapers (%d)", len(args.Monitor), len(args.Wallpaper))
	}

	canvasWidth, canvasHeight, monitors, vdMinX, vdMinY := wp.GetMonitors()
	log.Printf("  Monitor: %d\n", len(monitors))
	log.Printf("Wallpaper: %dx%dpx", canvasWidth, canvasHeight)
	log.Printf("VD origin: (%d,%d)\n", vdMinX, vdMinY)

	for _, monitor := range monitors {
		primaryStatus := " "
		if monitor.Primary {
			primaryStatus = "*"
		}
		log.Printf("Monitor %d%s: %+v\n", monitor.Index+1, primaryStatus, monitor.Resolution)
	}

	targets, err := selectMonitors(monitors, args.Monitor)
	if err != nil {
		log.Fatal(err)
	}
	if len(args.Wallpaper) > len(targets) {
		log.Printf("Warning: %d wallpaper(s) ignored — only %d monitor(s) available\n",
			len(args.Wallpaper)-len(targets), len(targets))
	}

	canvas := wp.CreateBlackCanvas(canvasWidth, canvasHeight)
	var vTargets []wp.VideoTarget
	hasImage := false

	for i, wallpaperPath := range args.Wallpaper {
		if i >= len(targets) {
			break
		}
		m := targets[i]

		if wp.IsVideoFile(wallpaperPath) {
			rawX := int(m.Resolution.X) + int(vdMinX)
			rawY := int(m.Resolution.Y) + int(vdMinY)
			log.Printf("Video %d → Monitor %d raw=(%d,%d) size=(%dx%d)",
				i+1, m.Index+1, rawX, rawY, m.Resolution.Width, m.Resolution.Height)
			cached, err := wp.PreprocessVideo(wallpaperPath, int(m.Resolution.Width), int(m.Resolution.Height))
			if err != nil {
				log.Printf("preprocess video: %v", err)
				continue
			}
			vTargets = append(vTargets, wp.VideoTarget{
				Path: cached,
				X:    rawX, Y: rawY,
				W: int(m.Resolution.Width), H: int(m.Resolution.Height),
			})
		} else {
			img, err := wp.LoadAndResizeImage(wallpaperPath, uint(m.Resolution.Width), uint(m.Resolution.Height))
			if err != nil {
				log.Printf("Error loading image for monitor %d: %v\n", m.Index+1, err)
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
			log.Printf("Error setting wallpaper style: %v\n", err)
		}
		outputPath, err := wp.SaveImageAs(canvas, 90)
		if err != nil {
			log.Printf("Error saving canvas: %v\n", err)
		} else {
			log.Printf("saved to: %s\n", outputPath)
			if err := wp.SetWallpaper(outputPath); err != nil {
				log.Printf("Error setting wallpaper: %v\n", err)
			}
		}
	}

	if len(vTargets) > 0 {
		if err := wp.RunVideoWallpapers(vTargets); err != nil {
			log.Fatal(err)
		}
	}
}
