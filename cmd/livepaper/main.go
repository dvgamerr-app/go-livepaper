package main

import (
	"fmt"
	"image"
	"image/draw"
	"log"
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

	canvasWidth, canvasHeight, monitors, vdMinX, vdMinY := getMonitors()
	log.Printf("  Monitor: %d\n", len(monitors))
	log.Printf("Wallpaper: %dx%dpx", canvasWidth, canvasHeight)
	log.Printf("VD origin: (%d,%d)\n", vdMinX, vdMinY)

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
	if len(args.Wallpaper) > len(targets) {
		log.Printf("Warning: %d wallpaper(s) ignored — only %d monitor(s) available\n",
			len(args.Wallpaper)-len(targets), len(targets))
	}

	canvas := createBlackCanvas(canvasWidth, canvasHeight)
	var vTargets []videoTarget
	hasImage := false

	for i, wp := range args.Wallpaper {
		if i >= len(targets) {
			break
		}
		m := targets[i]

		if isVideoFile(wp) {
			rawX := int(m.resolution.x) + int(vdMinX)
			rawY := int(m.resolution.y) + int(vdMinY)
			log.Printf("Video %d → Monitor %d raw=(%d,%d) size=(%dx%d)",
				i+1, m.index+1, rawX, rawY, m.resolution.width, m.resolution.height)
			vTargets = append(vTargets, videoTarget{
				path: wp,
				x:    rawX, y: rawY,
				w:    int(m.resolution.width), h: int(m.resolution.height),
			})
		} else {
			img, err := loadAndResizeImage(wp, uint(m.resolution.width), uint(m.resolution.height))
			if err != nil {
				log.Printf("Error loading image for monitor %d: %v\n", m.index+1, err)
				continue
			}
			draw.Draw(canvas,
				img.Bounds().Add(image.Pt(int(m.resolution.x), int(m.resolution.y))),
				img, image.Point{}, draw.Over)
			hasImage = true
		}
	}

	// Set static wallpaper whenever any image is present, or as black backdrop for videos.
	if hasImage || len(vTargets) > 0 {
		if err := setWallpaperStyle(STYLE_SPAN); err != nil {
			log.Printf("Error setting wallpaper style: %v\n", err)
		}
		outputPath, err := saveImageAs(canvas, 90)
		if err != nil {
			log.Printf("Error saving canvas: %v\n", err)
		} else {
			log.Printf("saved to: %s\n", outputPath)
			if err := setWallpaper(outputPath); err != nil {
				log.Printf("Error setting wallpaper: %v\n", err)
			}
		}
	}

	if len(vTargets) > 0 {
		if err := runVideoWallpapers(vTargets); err != nil {
			log.Fatal(err)
		}
	}
}
