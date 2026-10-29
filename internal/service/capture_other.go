//go:build !android

package service

// BeginCapture is only used on Android.
func (s *Service) BeginCapture() string { return "unsupported" }

// CaptureState is only used on Android.
func (s *Service) CaptureState() string { return "unsupported" }
