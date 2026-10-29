//go:build !windows && !android

package host

import "stereoscope/internal/audio"

type unsupported struct{}

func (unsupported) Devices() ([]audio.Device, error) { return nil, nil }
func (unsupported) Select(string) error             { return nil }
func (unsupported) Levels() ([]audio.PairLevels, error) {
	return nil, nil
}
func (unsupported) Sources() ([]audio.Source, error) { return nil, nil }
func (unsupported) PlayTest(string, string) error {
	return errTone
}
func (unsupported) Close() error             { return nil }
func (unsupported) Changed() <-chan struct{} { return nil }

var errTone = errString("test tones are available on Windows")

type errString string

func (e errString) Error() string { return string(e) }

// Open returns the platform audio backend.
func Open() (audio.Backend, error) { return unsupported{}, nil }
