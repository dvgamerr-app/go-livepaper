package main

import (
	"fmt"
	"image"
	"image/draw"
	"log"

	wp "github.com/dvgamerr/go-livepaper/internal/wallpaper"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func runTrayApp() {
	app := application.New(application.Options{
		Name:        "livepaper",
		Description: "Live wallpaper tray",
		Assets:      application.AlphaAssets,
	})

	tray := app.SystemTray.New()
	tray.SetIcon(makeTrayIcon())
	tray.SetTooltip("livepaper — right-click to set wallpaper")
	tray.SetMenu(buildTrayMenu(app))

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func applyWallpaperFromPaths(app *application.App, paths []string, monitorIndexes []int) error {
	canvasWidth, canvasHeight, monitors, vdMinX, vdMinY := wp.GetMonitors()

	var targets []wp.MonitorInfo
	if len(monitorIndexes) == 0 {
		targets = monitors
	} else {
		for _, idx := range monitorIndexes {
			if idx >= 0 && idx < len(monitors) {
				targets = append(targets, monitors[idx])
			}
		}
	}

	canvas := wp.CreateBlackCanvas(canvasWidth, canvasHeight)
	var vTargets []wp.VideoTarget
	hasImage := false

	for i, path := range paths {
		if i >= len(targets) {
			break
		}
		m := targets[i]

		if wp.IsVideoFile(path) {
			rawX := int(m.Resolution.X) + int(vdMinX)
			rawY := int(m.Resolution.Y) + int(vdMinY)
			vTargets = append(vTargets, wp.VideoTarget{
				Path: path,
				X:    rawX, Y: rawY,
				W: int(m.Resolution.Width), H: int(m.Resolution.Height),
			})
		} else {
			img, err := wp.LoadAndResizeImage(path, uint(m.Resolution.Width), uint(m.Resolution.Height))
			if err != nil {
				log.Printf("monitor %d: %v\n", m.Index+1, err)
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
		return wp.RunVideoWallpapers(vTargets)
	}
	return nil
}

func buildTrayMenu(app *application.App) *application.Menu {
	_, _, monitors, _, _ := wp.GetMonitors()

	menu := app.NewMenu()

	menu.Add("Set Wallpaper...").OnClick(func(ctx *application.Context) {
		result, err := app.Dialog.OpenFile().
			SetTitle("Select Wallpaper").
			AddFilter("Images", "*.jpg;*.jpeg;*.png;*.bmp;*.webp").
			PromptForSingleSelection()
		if err != nil || result == "" {
			return
		}
		if err := applyWallpaperFromPaths(app, []string{result}, nil); err != nil {
			log.Printf("applyWallpaper: %v", err)
		}
	})

	if len(monitors) > 1 {
		monitorMenu := menu.AddSubmenu("Set for Monitor")
		for _, m := range monitors {
			m := m
			label := fmt.Sprintf("Monitor %d", m.Index+1)
			if m.Primary {
				label += " (Primary)"
			}
			label += fmt.Sprintf(" %dx%d", m.Resolution.Width, m.Resolution.Height)
			monitorMenu.Add(label).OnClick(func(ctx *application.Context) {
				result, err := app.Dialog.OpenFile().
					SetTitle(fmt.Sprintf("Select Wallpaper for Monitor %d", m.Index+1)).
					AddFilter("Images", "*.jpg;*.jpeg;*.png;*.bmp;*.webp").
					PromptForSingleSelection()
				if err != nil || result == "" {
					return
				}
				if err := applyWallpaperFromPaths(app, []string{result}, []int{m.Index}); err != nil {
					log.Printf("monitor %d: %v", m.Index+1, err)
				}
			})
		}
	}

	menu.Add("Set Video Wallpaper...").OnClick(func(ctx *application.Context) {
		result, err := app.Dialog.OpenFile().
			SetTitle("Select Video Wallpaper").
			AddFilter("Videos", "*.mp4;*.mkv;*.avi;*.mov;*.webm;*.m4v;*.gif").
			PromptForSingleSelection()
		if err != nil || result == "" {
			return
		}
		if err := applyWallpaperFromPaths(app, []string{result}, nil); err != nil {
			log.Printf("applyWallpaper video: %v", err)
		}
	})

	menu.AddSeparator()

	menu.Add("Clean Temp Files").OnClick(func(ctx *application.Context) {
		if err := wp.CleanTempDir(); err != nil {
			log.Printf("cleanTempDir: %v", err)
		} else {
			log.Println("Temp files cleaned")
		}
	})

	menu.AddSeparator()

	menu.Add(fmt.Sprintf("livepaper %s", VERSION)).SetEnabled(false)
	menu.Add("Quit").OnClick(func(ctx *application.Context) {
		app.Quit()
	})

	return menu
}
