//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func setupTray(*application.App, *application.WebviewWindow) {}
