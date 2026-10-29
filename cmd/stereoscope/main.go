package main

import (
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"stereoscope/frontend"
	"stereoscope/internal/audio"
	"stereoscope/internal/host"
	"stereoscope/internal/service"
)

func init() {
	application.RegisterEvent[[]audio.PairLevels]("levels")
	application.RegisterEvent[[]audio.Source]("sources")
	application.RegisterEvent[[]audio.Device]("devices")
}

func main() {
	backend, err := host.Open()
	if err != nil {
		log.Fatal(err)
	}
	svc := service.New(backend)
	settings := svc.Settings()

	app := application.New(application.Options{
		Name:        "StereoScope",
		Description: "See which stereo channels are playing, and which apps are playing them.",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(frontend.Dist()),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "StereoScope",
		Width:            960,
		Height:           720,
		MinWidth:         720,
		MinHeight:        560,
		BackgroundColour: application.NewRGB(17, 19, 24),
		URL:              "/",
		Hidden:           settings.StartMinimised,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
	})
	svc.AttachWindow(window)
	setupTray(app, window)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
