package main

import (
	"fmt"
	"image"
	"image/draw"
	"log"
	"os"

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
	Monitor   []string `arg:"-m,--monitor" help:"Target monitors to set wallpaper (e.g. -m 1 -m 2)"`
	Clean     bool     `arg:"-c,--clean" help:"Clean all temporary files"`
	Wallpaper []string `arg:"positional" help:"Wallpaper is a list of file paths to wallpaper images"`
}

var args Args

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

	if err := setWallpaperStyle(STYLE_SPAN); err != nil {
		log.Printf("Error setting wallpaper style: %v\n", err)
	}

	canvas := createBlackCanvas(canvasWidth, canvasHeight)

	for i, monitor := range monitors {
		primaryStatus := " "
		if monitor.primary {
			primaryStatus = "*"
		}
		log.Printf("Monitor %d%s: %+v\n", monitor.index+1, primaryStatus, monitor.resolution)

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
