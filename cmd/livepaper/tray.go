package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	wp "github.com/dvgamerr/go-livepaper/internal/wallpaper"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type WindowState struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func windowStatePath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.TempDir()
	}
	return filepath.Join(appData, "livepaper", "window-state.json")
}

func loadWindowState() *WindowState {
	data, err := os.ReadFile(windowStatePath())
	if err != nil {
		return nil
	}
	var state WindowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	if state.Width < 900 || state.Height < 580 {
		return nil
	}
	return &state
}

func saveWindowState(state WindowState) {
	path := windowStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0644)
}

func isDev() bool {
	return VERSION == "dev"
}

func runTrayApp() {
	svc := &AppService{}

	app := application.New(application.Options{
		Name:        "livepaper",
		Description: "Live wallpaper",
		Icon:        appIcon(),
		Assets: application.AssetOptions{
			Handler: getAssetHandler(),
		},
		Services: []application.Service{
			application.NewService(svc),
		},
	})

	svc.app = app

	savedState := loadWindowState()

	opts := application.WebviewWindowOptions{
		Title:        "LivePaper",
		Width:        900,
		Height:       580,
		MinWidth:     900,
		MinHeight:    580,
		Frameless:    true,
		Hidden:       !isDev(),
		HideOnEscape: !isDev(),
	}

	if savedState != nil {
		opts.Width = savedState.Width
		opts.Height = savedState.Height
		opts.X = savedState.X
		opts.Y = savedState.Y
		opts.InitialPosition = application.WindowXY
	}

	window := app.Window.NewWithOptions(opts)
	svc.window = window

	// Track last known state in memory for tray-click restore
	var stateMu sync.RWMutex
	lastState := WindowState{X: opts.X, Y: opts.Y, Width: opts.Width, Height: opts.Height}
	if savedState != nil {
		lastState = *savedState
	}

	captureState := func() WindowState {
		x, y := window.Position()
		w, h := window.Size()
		return WindowState{X: x, Y: y, Width: w, Height: h}
	}

	persistState := func() {
		cur := captureState()
		if cur.Width < 900 || cur.Height < 580 {
			return
		}
		stateMu.Lock()
		lastState = cur
		stateMu.Unlock()
		saveWindowState(cur)
	}

	window.RegisterHook(events.Common.WindowDidMove, func(e *application.WindowEvent) {
		persistState()
	})
	window.RegisterHook(events.Common.WindowDidResize, func(e *application.WindowEvent) {
		persistState()
	})

	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		window.Hide()
	})

	// Periodic save as fallback for frameless-window drags that may not fire events
	go func() {
		var last WindowState
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			cur := captureState()
			if cur != last && cur.Width >= 900 && cur.Height >= 580 {
				stateMu.Lock()
				lastState = cur
				stateMu.Unlock()
				saveWindowState(cur)
				last = cur
			}
		}
	}()

	tray := app.SystemTray.New()
	tray.SetIcon(makeTrayIcon())
	tray.SetTooltip("livepaper")
	// AttachWindow for debounce only; click position is handled by OnClick below
	tray.AttachWindow(window).
		WindowOffset(5).
		WindowDebounce(300 * time.Millisecond)
	tray.SetMenu(buildTrayMenu(app))

	// Override default ToggleWindow so the window restores to saved position
	// instead of snapping near the tray icon
	tray.OnClick(func() {
		if window.IsVisible() {
			window.Hide()
			return
		}
		stateMu.RLock()
		s := lastState
		stateMu.RUnlock()
		window.SetPosition(s.X, s.Y)
		window.Show()
		window.Focus()
	})

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
