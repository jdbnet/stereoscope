//go:build windows

package main

import (
	_ "embed"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"

	"stereoscope/internal/audio"
)

//go:embed icon.png
var trayIcon []byte

func setupTray(app *application.App, window *application.WebviewWindow) {
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("StereoScope")

	menu := app.Menu.New()
	meter := menu.Add("Silent")
	meter.SetEnabled(false)
	menu.AddSeparator()
	menu.Add("Show StereoScope").OnClick(func(*application.Context) {
		window.Show()
		window.Focus()
	})
	menu.Add("Quit").OnClick(func(*application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)
	tray.OnClick(func() {
		window.Show()
		window.Focus()
	})

	app.Event.On("levels", func(e *application.CustomEvent) {
		levels, ok := e.Data.([]audio.PairLevels)
		if !ok || len(levels) == 0 {
			return
		}
		front := levels[0]
		text := fmt.Sprintf("L %.0f%%   R %.0f%%", front.Left*100, front.Right*100)
		meter.SetLabel(text)
		tray.SetTooltip("StereoScope  " + text)
	})
}
