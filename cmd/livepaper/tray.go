package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	wp "github.com/dvgamerr/go-livepaper/internal/wallpaper"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed frontend
var frontendFS embed.FS

func runTrayApp() {
	svc := &AppService{}

	assetsFS, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:        "livepaper",
		Description: "Live wallpaper",
		Assets: application.AssetOptions{
			Handler: http.FileServer(http.FS(assetsFS)),
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
		Hidden:        true,
		HideOnEscape:  true,
	})

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
