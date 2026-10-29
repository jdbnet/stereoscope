package service

import (
	"context"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"stereoscope/internal/audio"
	"stereoscope/internal/version"
)

// Service is the Wails-bound audio monitor.
type Service struct {
	backend     audio.Backend
	cancel      context.CancelFunc
	window      *application.WebviewWindow
	captureText string
}

// New builds the service. The window may be nil until attached.
func New(backend audio.Backend) *Service {
	return &Service{backend: backend}
}

// AttachWindow lets the tray show the main window.
func (s *Service) AttachWindow(window *application.WebviewWindow) {
	s.window = window
}

// Window returns the main window, if one was attached.
func (s *Service) Window() *application.WebviewWindow { return s.window }

// ServiceStartup starts polling once the application is running.
func (s *Service) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	settings := loadSettings()
	if settings.DeviceID != "" {
		_ = s.backend.Select(settings.DeviceID)
	}
	ctx, s.cancel = context.WithCancel(ctx)
	go s.loop(ctx)
	return nil
}

// ServiceShutdown stops polling and releases the backend.
func (s *Service) ServiceShutdown() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.backend != nil {
		return s.backend.Close()
	}
	return nil
}

func (s *Service) loop(ctx context.Context) {
	levels := time.NewTicker(30 * time.Millisecond)
	sources := time.NewTicker(500 * time.Millisecond)
	defer levels.Stop()
	defer sources.Stop()
	s.emitDevices()
	var changes <-chan struct{}
	if s.backend != nil {
		changes = s.backend.Changed()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-levels.C:
			if lv, err := s.backend.Levels(); err == nil {
				application.Get().Event.Emit("levels", lv)
			}
		case <-sources.C:
			if src, err := s.backend.Sources(); err == nil {
				application.Get().Event.Emit("sources", src)
			}
			if state := captureState(); state != "" && state != s.captureText {
				s.captureText = state
				application.Get().Event.Emit("capture", state)
			}
		case <-changes:
			s.emitDevices()
		}
	}
}

func (s *Service) emitDevices() {
	devices, err := s.backend.Devices()
	if err != nil {
		return
	}
	application.Get().Event.Emit("devices", devices)
}

// Devices returns the current playback devices.
func (s *Service) Devices() ([]audio.Device, error) {
	return s.backend.Devices()
}

// Select monitors the device with the given id and remembers the choice.
func (s *Service) Select(id string) error {
	if err := s.backend.Select(id); err != nil {
		return err
	}
	settings := loadSettings()
	settings.DeviceID = id
	_ = saveSettings(settings)
	s.emitDevices()
	return nil
}

// PlayTest plays a short tone on one side of a stereo pair.
func (s *Service) PlayTest(pairID, side string) error {
	return s.backend.PlayTest(pairID, side)
}

// Settings returns the saved preferences.
func (s *Service) Settings() Settings { return loadSettings() }

// SaveSettings stores preferences.
func (s *Service) SaveSettings(settings Settings) error { return saveSettings(settings) }

// Version returns the build version.
func (s *Service) Version() string { return version.Version }

// Platform returns the operating system name.
func (s *Service) Platform() string { return runtime.GOOS }
