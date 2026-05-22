package main

import (
	"fmt"
	"image"
	"image/draw"
	"log"
	"os"
	"strconv"

	"github.com/alexflint/go-arg"
)

// VERSION is set during build using ldflags.
var VERSION = "dev"

func (Args) Version() string {
	return fmt.Sprintf("livepaper %s", VERSION)
}

func init() {
	log.SetFlags(log.Lshortfile | log.Ltime)
}

// Args defines the command line arguments.
type Args struct {
	Monitor   []string `arg:"-m,--monitor" help:"Target monitor numbers in wallpaper order, 1-based (e.g. -m 1 -m 2)"`
	Clean     bool     `arg:"-c,--clean" help:"Clean all temporary files"`
	Wallpaper []string `arg:"positional" help:"Wallpaper is a list of file paths to wallpaper images"`
}

var args Args

func selectMonitors(monitors []MonitorInfo, selected []string) ([]MonitorInfo, error) {
	if len(selected) == 0 {
		return monitors, nil
	}

	targets := make([]MonitorInfo, 0, len(selected))
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
		if err := cleanTempDir(); err != nil {
			log.Printf("Error cleaning temp directory: %v\n", err)
		} else {
			log.Println("Temporary files cleaned successfully")
			return
		}
	}

	if len(args.Wallpaper) == 0 {
		log.Fatalf("No wallpaper specified. Please provide at least one wallpaper image path.")
	}
	if len(args.Wallpaper) != len(args.Monitor) && len(args.Monitor) > 0 {
		log.Fatalf("Invalid arguments: monitors (%d) must match of wallpapers (%d)", len(args.Monitor), len(args.Wallpaper))
	}

	canvasWidth, canvasHeight, monitors := getMonitors()
	log.Printf("  Monitor: %d\n", len(monitors))
	log.Printf("Wallpaper: %dx%dpx", canvasWidth, canvasHeight)

	for _, monitor := range monitors {
		primaryStatus := " "
		if monitor.primary {
			primaryStatus = "*"
		}
		log.Printf("Monitor %d%s: %+v\n", monitor.index+1, primaryStatus, monitor.resolution)
	}

	targets, err := selectMonitors(monitors, args.Monitor)
	if err != nil {
		log.Fatal(err)
	}
	if len(args.Monitor) == 0 && len(args.Wallpaper) > len(monitors) {
		log.Printf("Warning: %d wallpaper(s) ignored because only %d monitor(s) were detected\n", len(args.Wallpaper)-len(monitors), len(monitors))
	}

	if err := setWallpaperStyle(STYLE_SPAN); err != nil {
		log.Printf("Error setting wallpaper style: %v\n", err)
	}

	canvas := createBlackCanvas(canvasWidth, canvasHeight)

	for i, monitor := range targets {
		if i > len(args.Wallpaper)-1 {
			continue
		}
		img1, err := loadAndResizeImage(args.Wallpaper[i], uint(monitor.resolution.width), uint(monitor.resolution.height))
		if err != nil {
			log.Printf("Error loadAndResizeImage: %v\n", err)
		}
		draw.Draw(canvas, img1.Bounds().Add(image.Pt(int(monitor.resolution.x), int(monitor.resolution.y))), img1, image.Point{}, draw.Over)
	}

	outputPath, err := saveImageAs(canvas, 90)
	if err != nil {
		log.Printf("Error saving image: %v\n", err)
	}

	log.Printf("saved to: %s\n", outputPath)

	if err := setWallpaper(outputPath); err != nil {
		log.Printf("Error setting wallpaper: %v\n", err)
		os.Exit(1)
	}
}
