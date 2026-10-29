package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var raw embed.FS

// Dist is the Vite production build, with paths relative to the dist root.
func Dist() fs.FS {
	sub, err := fs.Sub(raw, "dist")
	if err != nil {
		return raw
	}
	return sub
}
