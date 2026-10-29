# StereoScope

StereoScope shows whether sound is coming out of the left channel, the right channel, or both, and which programs are playing to the output you pick.

It is a [Wails](https://v3.wails.io/) v3 application written in Go 1.27.3. The interface is a dark theme with accent colour `#1ebe8a`.

## Windows

The Windows build reads peak meters from WASAPI. It does not capture the mix, so idle monitoring stays light. For a 7.1 device it shows four pairs: Front, Centre / LFE, Rear, and Side. Each pair is labelled Left only, Right only, Both, or Silent. The test buttons play a short tone on the left, the right, or both of that pair.

A program appears under Now playing when its session is active and actually producing sound. System sounds are labelled "System sounds". The last device is stored in the user config directory under `stereoscope/settings.json`.

The system tray shows a compact left/right percentage and can open or quit the app.

## Android

Android support uses the experimental Wails v3 Android target. Android usually plays everything through one active output, so choosing a device monitors that route rather than a separate mix.

Levels require Android 10 or later. The app asks for media-projection consent and keeps a foreground notification while it is listening. It measures the left and right peaks of the captured stereo mix. Many streaming and DRM apps opt out of capture, so their audio will not move the meters. The interface says: this app doesn't allow audio capture.

Now playing lists what Android reports, for example "Media playback", "Game", or "Notification". An app name is shown only when a media notification is visible and notification access has been enabled for StereoScope. Test tones are not available.

Build an APK with the Android SDK, NDK 26.3, and API 35 installed:

```bash
wails3 task android:package
```

## Build

Install Go 1.27.3, Node, and Wails v3.0.0 (`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`).

```bash
wails3 task dev          # current desktop
goreleaser build --snapshot --clean
```

Push to `main`, then run the Release workflow from the Actions tab. It has no inputs. The workflow tags the next semantic version from the conventional commits. The APK is built on Ubuntu, because the Wails Android task only supports a Linux or macOS NDK. GoReleaser then runs on Windows, writes the release notes, and publishes that version with `stereoscope.exe` (Windows amd64) and the APK. Nothing is code-signed.

The version shown in About comes from the git tag via GoReleaser. A normal `go build` shows `dev`.

## Checks

- Play a left-only and a right-only file and confirm the label and meters.
- Switch devices while audio is playing. Unplug a USB or Bluetooth device and confirm the list recovers.
- Two apps playing at once both appear. A paused app disappears within about a second.
- A 7.1 device shows four pairs. A BASS program playing to one pair's left lights only that meter.
- Idle monitoring should stay under about 2% CPU.
- On Android, walk through consent, the foreground notification, an opted-out app, and rotation.

## Limits

- Wails Android support is experimental and can change.
- Capture cannot see apps that opt out, and it cannot name other apps unless notification access is granted.
- The Windows COM code is not exercised by `go test` on Linux. `go test ./internal/audio` covers channel grouping, the silence threshold, and the 150ms label hold.
