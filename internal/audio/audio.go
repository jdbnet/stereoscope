// Package audio is the platform-neutral monitoring API.
package audio

import "errors"

// ErrUnsupported is returned when a platform cannot perform an operation.
var ErrUnsupported = errors.New("not supported on this platform")

// Device is a render endpoint the user can monitor.
type Device struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Default  bool   `json:"default"`
	Channels int    `json:"channels"`
	Active   bool   `json:"active"`
}

// Pair is one stereo grouping of a device's channels.
type Pair struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// PairLevels holds peak levels in the range 0..1 for one pair.
type PairLevels struct {
	Pair  Pair    `json:"pair"`
	Left  float32 `json:"left"`
	Right float32 `json:"right"`
}

// Source is a process or playback client currently sending audio.
type Source struct {
	PID  int     `json:"pid"`
	Name string  `json:"name"`
	Icon string  `json:"icon"`
	Peak float32 `json:"peak"`
	Note string  `json:"note"`
}

// Backend reads devices, levels and sources for one selected output.
type Backend interface {
	Devices() ([]Device, error)
	Select(id string) error
	Levels() ([]PairLevels, error)
	Sources() ([]Source, error)
	PlayTest(pairID string, side string) error
	Close() error
	// Changed is closed and replaced when the device list should be refreshed.
	Changed() <-chan struct{}
}
