//go:build android

package host

import (
	"stereoscope/internal/audio"
	"stereoscope/internal/audio/android"
)

// Open returns the platform audio backend.
func Open() (audio.Backend, error) { return android.Open() }
