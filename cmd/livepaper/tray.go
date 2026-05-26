package main

import (
	"fmt"
	"log"
	"time"

	wp "github.com/dvgamerr/go-livepaper/internal/wallpaper"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func runTrayApp() {
	svc := &AppService{}

	app := application.New(application.Options{
		Name:        "livepaper",
		Description: "Live wallpaper",
		Assets: application.AssetOptions{
			Handler: getAssetHandler(),
		},
		Services: []application.Service{
			application.NewService(svc),
		},
	})

	svc.app = app

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:         "LivePaper",
		Width:         740,
		Height:        480,
		DisableResize: true,
		Frameless:     true,
		Hidden:        true,
		HideOnEscape:  true,
	})
	svc.window = window

	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		window.Hide()
	})

	tray := app.SystemTray.New()
	tray.SetIcon(makeTrayIcon())
	tray.SetTooltip("livepaper")
	tray.AttachWindow(window).
		WindowOffset(5).
		WindowDebounce(300 * time.Millisecond)
	tray.SetMenu(buildTrayMenu(app))

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func buildTrayMenu(app *application.App) *application.Menu {
	menu := app.NewMenu()
	menu.Add(fmt.Sprintf("livepaper %s", VERSION)).SetEnabled(false)
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(ctx *application.Context) {
		wp.StopVideoWallpapers()
		app.Quit()
	})
	return menu
}
