# StereoScope

Go 1.27.3 and Wails v3.0.0. The module line is `go 1.27.3`. The app entrypoint is `cmd/stereoscope`. Frontend assets are embedded from `frontend/dist` by `frontend/embed.go`.

There is no git repository yet. Do not create one, commit, or push unless asked.

## Layout

- `cmd/stereoscope` starts the Wails app, the window, and the Windows tray.
- `internal/audio` holds `Device`, `Pair`, `PairLevels`, `Source`, and `Backend`.
- `internal/audio/windows` is the WASAPI backend (`//go:build windows`), using `github.com/moutend/go-wca`.
- `internal/audio/android` is the playback-capture backend (`//go:build android`).
- `internal/host` chooses the backend. `internal/service` polls it and emits Wails events.
- `frontend` is Vite and TypeScript. Accent colour `#1ebe8a`, dark theme, UK English.
- `build/android/.../AudioBridge.java` is the JNI helper. `AudioCaptureService` records the mix. `PlaybackNotificationListener` supplies an app name only when notification access is enabled.

## Windows

All COM calls run on one goroutine that has called `runtime.LockOSThread` and `CoInitializeEx`. The service talks to that goroutine with channels and reads snapshots. `IMMNotificationClient` callbacks only signal the goroutine.

`GetChannelsPeakValues` feeds `GroupPeaks`. The channel mask order matches BASS pairs: Front (FL/FR), Centre/LFE, Rear (RL/RR), Side (SL/SR). A stereo device produces one pair.

A session is playing when it is active and its peak is at least 0.005. PID 0 and the system-sounds session are shown as "System sounds".

Test tones use a short shared-mode WASAPI render stream. The status label uses a threshold of about -50 dBFS and 150ms hysteresis (`internal/audio/status.go`).

## Android

Wails v3 Android support is experimental. The same `main.go` is built as `libwails.so`. There is no high-level native-call API, so Go calls the static methods on `com.wails.app.AudioBridge`.

Android routes playback to one active output. The device list reports that route. Levels need Android 10 or later, a media-projection consent prompt, and a foreground service. Apps that opt out of capture do not move the meters. Sources are usage labels such as "Media playback" or "Game". A package name is shown only when a media notification is actually visible. Test tones are unavailable. Tray calls are Windows-only.

`android` implies the `linux` build tag. Desktop Linux files must use `linux && !android`.

## Release

GoReleaser v2 builds the Windows binary and publishes the GitHub release. `.goreleaser.yaml` runs `scripts/prepare` first (bindings, frontend, and `wails3 task android:package` when `ANDROID_HOME` is set) and `scripts/package` afterwards for the NSIS installer. The Release workflow is `workflow_dispatch` only. It tags `svu next` from the conventional commits, builds the APK on Ubuntu, then runs `goreleaser release --clean` on Windows. The Android task refuses a Windows host (`Unsupported host OS`) because it only selects the Linux or macOS NDK toolchain. GoReleaser writes the release notes and stamps `-X stereoscope/internal/version.Version`. A plain `go build` reports `dev`.
