//go:build windows

package windows

import (
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"

	"stereoscope/internal/audio"
)

func (b *backend) openLoopback(dev *wca.IMMDevice) {
	b.closeLoopback()
	var client *wca.IAudioClient
	if err := dev.Activate(wca.IID_IAudioClient, wca.CLSCTX_ALL, nil, &client); err != nil {
		return
	}
	var wfx *wca.WAVEFORMATEX
	if err := client.GetMixFormat(&wfx); err != nil || wfx == nil {
		client.Release()
		return
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(wfx)))
	if wfx.WBitsPerSample != 32 || wfx.NChannels == 0 || wfx.NBlockAlign != wfx.NChannels*4 {
		client.Release()
		return
	}
	// One second, in 100-nanosecond units. Shared loopback has to use the mix format.
	const buffer = wca.REFERENCE_TIME(10_000_000)
	if err := client.Initialize(wca.AUDCLNT_SHAREMODE_SHARED, wca.AUDCLNT_STREAMFLAGS_LOOPBACK, buffer, 0, wfx, nil); err != nil {
		client.Release()
		return
	}
	var capture *wca.IAudioCaptureClient
	if err := client.GetService(wca.IID_IAudioCaptureClient, &capture); err != nil {
		client.Release()
		return
	}
	if err := client.Start(); err != nil {
		capture.Release()
		client.Release()
		return
	}
	b.captureClient = client
	b.capture = capture
	b.captureCh = int(wfx.NChannels)
	b.captureMask = channelMask(wfx)
}

func (b *backend) closeLoopback() {
	if b.captureClient != nil {
		_ = b.captureClient.Stop()
	}
	if b.capture != nil {
		b.capture.Release()
		b.capture = nil
	}
	if b.captureClient != nil {
		b.captureClient.Release()
		b.captureClient = nil
	}
	b.captureCh = 0
}

// loopbackPeaks returns the peak absolute sample per channel since the last
// call. A nil slice means no new packets arrived.
func (b *backend) loopbackPeaks() []float32 {
	if b.capture == nil || b.captureCh < 1 {
		return nil
	}
	peaks := make([]float32, b.captureCh)
	got := false
	for {
		var packet uint32
		if err := b.capture.GetNextPacketSize(&packet); err != nil || packet == 0 {
			break
		}
		var data *byte
		var frames, flags uint32
		var devicePos, qpc uint64
		if err := b.capture.GetBuffer(&data, &frames, &flags, &devicePos, &qpc); err != nil {
			break
		}
		if flags&wca.AUDCLNT_BUFFERFLAGS_SILENT == 0 && data != nil && frames > 0 {
			samples := unsafe.Slice((*float32)(unsafe.Pointer(data)), int(frames)*b.captureCh)
			for i, sample := range samples {
				if sample < 0 {
					sample = -sample
				}
				ch := i % b.captureCh
				if sample > peaks[ch] {
					peaks[ch] = sample
				}
			}
		}
		_ = b.capture.ReleaseBuffer(frames)
		got = true
	}
	if !got {
		return nil
	}
	return peaks
}

func levelsFromChannels(mask uint32, peaks []float32) []audio.PairLevels {
	grouped := audio.GroupPeaks(mask, peaks)
	if len(grouped) > 0 {
		return grouped
	}
	if len(peaks) >= 2 {
		return []audio.PairLevels{{
			Pair:  audio.Pair{ID: "front", Label: "Front"},
			Left:  peaks[0],
			Right: peaks[1],
		}}
	}
	if len(peaks) == 1 {
		return []audio.PairLevels{{
			Pair:  audio.Pair{ID: "front", Label: "Front"},
			Left:  peaks[0],
			Right: peaks[0],
		}}
	}
	return nil
}
