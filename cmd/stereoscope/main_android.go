//go:build android

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func init() {
	// c-shared libraries do not call main. The overlay Wails generates lands
	// this registration at the repo root, which is not this package.
	application.RegisterAndroidMain(main)
}
