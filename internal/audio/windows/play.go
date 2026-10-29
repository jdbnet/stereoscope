//go:build windows

package windows

import (
	"fmt"
	"math"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"

	"stereoscope/internal/audio"
)

func playTone(dev *wca.IMMDevice, mask uint32, channels int, pairID, side string) error {
	var client *wca.IAudioClient
	if err := dev.Activate(wca.IID_IAudioClient, wca.CLSCTX_ALL, nil, &client); err != nil {
		return err
	}
	defer client.Release()

	var wfx *wca.WAVEFORMATEX
	if err := client.GetMixFormat(&wfx); err != nil || wfx == nil {
		return fmt.Errorf("mix format: %w", err)
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(wfx)))
	if mask == 0 {
		mask = channelMask(wfx)
	}
	if channels == 0 {
		channels = int(wfx.NChannels)
	}
	if wfx.WBitsPerSample != 32 || wfx.NBlockAlign != wfx.NChannels*4 {
		return fmt.Errorf("test tone needs a 32-bit float mix format")
	}

	const duration = 400 * time.Millisecond
	period := wca.REFERENCE_TIME(duration / 100) // 100-nanosecond units
	if err := client.Initialize(wca.AUDCLNT_SHAREMODE_SHARED, 0, period, 0, wfx, nil); err != nil {
		return err
	}
	var render *wca.IAudioRenderClient
	if err := client.GetService(wca.IID_IAudioRenderClient, &render); err != nil {
		return err
	}
	defer render.Release()

	var frames uint32
	if err := client.GetBufferSize(&frames); err != nil {
		return err
	}
	var data *byte
	if err := render.GetBuffer(frames, &data); err != nil {
		return err
	}
	samples := unsafe.Slice((*float32)(unsafe.Pointer(data)), int(frames)*channels)
	fillTone(samples, channels, int(wfx.NSamplesPerSec), mask, pairID, side)
	if err := render.ReleaseBuffer(frames, 0); err != nil {
		return err
	}
	if err := client.Start(); err != nil {
		return err
	}
	time.Sleep(duration)
	return client.Stop()
}

func fillTone(samples []float32, channels, rate int, mask uint32, pairID, side string) {
	leftOn, rightOn := audio.Sides(pairID, side)
	left := audio.ChannelIndex(mask, channels, pairID, "left")
	right := audio.ChannelIndex(mask, channels, pairID, "right")
	frames := len(samples) / channels
	for i := 0; i < frames; i++ {
		env := 0.2
		if i < rate/100 {
			env *= float64(i) / float64(rate/100)
		}
		if frames-i < rate/100 {
			env *= float64(frames-i) / float64(rate/100)
		}
		s := float32(env * math.Sin(2*math.Pi*440*float64(i)/float64(rate)))
		base := i * channels
		for c := 0; c < channels; c++ {
			samples[base+c] = 0
		}
		if leftOn && left >= 0 {
			samples[base+left] = s
		}
		if rightOn && right >= 0 {
			samples[base+right] = s
		}
	}
}
