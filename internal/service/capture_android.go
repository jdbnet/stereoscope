//go:build android

package service

import "stereoscope/internal/audio/android"

// BeginCapture shows the Android media-projection consent prompt.
func (s *Service) BeginCapture() string {
	android.BeginCapture()
	return android.CaptureState()
}

// CaptureState reports whether playback capture is running.
func (s *Service) CaptureState() string { return android.CaptureState() }

func captureState() string { return android.CaptureState() }
