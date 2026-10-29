//go:build windows

package host

import (
	"stereoscope/internal/audio"
	"stereoscope/internal/audio/windows"
)

// Open returns the platform audio backend.
func Open() (audio.Backend, error) { return windows.Open() }
