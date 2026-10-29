//go:build windows

package windows

import (
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"

	"stereoscope/internal/audio"
)

const playingThreshold float32 = 0.005

type command struct {
	kind           string
	id, pair, side string
	errc           chan error
}

type backend struct {
	cmds    chan command
	notify  chan struct{}
	done    chan struct{}
	changed chan struct{}

	mu       sync.RWMutex
	devices  []audio.Device
	levels   []audio.PairLevels
	sources  []audio.Source
	readyErr error

	enum       *wca.IMMDeviceEnumerator
	selected   *wca.IMMDevice
	selectedID string
	meter      *wca.IAudioMeterInformation
	mask       uint32
	channels   int
	icons      map[string]string
}

// Open starts the WASAPI backend on a locked OS thread.
func Open() (audio.Backend, error) {
	b := &backend{
		cmds:    make(chan command),
		notify:  make(chan struct{}, 1),
		done:    make(chan struct{}),
		changed: make(chan struct{}, 1),
		icons:   map[string]string{},
	}
	ready := make(chan error, 1)
	go b.loop(ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return b, nil
}

func (b *backend) Devices() ([]audio.Device, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := append([]audio.Device(nil), b.devices...)
	return out, nil
}

func (b *backend) Levels() ([]audio.PairLevels, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]audio.PairLevels(nil), b.levels...), nil
}

func (b *backend) Sources() ([]audio.Source, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]audio.Source(nil), b.sources...), nil
}

func (b *backend) Changed() <-chan struct{} { return b.changed }

func (b *backend) Select(id string) error {
	return b.call(command{kind: "select", id: id})
}

func (b *backend) PlayTest(pairID, side string) error {
	return b.call(command{kind: "play", pair: pairID, side: side})
}

func (b *backend) Close() error {
	select {
	case <-b.done:
		return nil
	default:
		close(b.done)
	}
	return nil
}

func (b *backend) call(c command) error {
	c.errc = make(chan error, 1)
	select {
	case <-b.done:
		return fmt.Errorf("audio backend stopped")
	case b.cmds <- c:
	}
	return <-c.errc
}

func (b *backend) signal() {
	select {
	case b.notify <- struct{}{}:
	default:
	}
}

func (b *backend) loop(ready chan error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		ready <- err
		return
	}
	defer ole.CoUninitialize()

	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &b.enum); err != nil {
		ready <- err
		return
	}
	defer b.enum.Release()

	client := wca.NewIMMNotificationClient(wca.IMMNotificationClientCallback{
		OnDefaultDeviceChanged: func(wca.EDataFlow, wca.ERole, string) error { b.signal(); return nil },
		OnDeviceAdded:          func(string) error { b.signal(); return nil },
		OnDeviceRemoved:        func(string) error { b.signal(); return nil },
		OnDeviceStateChanged:   func(string, uint64) error { b.signal(); return nil },
		OnPropertyValueChanged: func(string, uint64) error { return nil },
	})
	_ = b.enum.RegisterEndpointNotificationCallback(client)
	defer unregisterNotification(b.enum, client)

	if err := b.refreshDevices(); err != nil {
		ready <- err
		return
	}
	if err := b.selectDefault(); err != nil {
		ready <- err
		return
	}
	b.readLevels()
	b.readSources()
	ready <- nil

	levels := time.NewTicker(30 * time.Millisecond)
	sources := time.NewTicker(500 * time.Millisecond)
	defer levels.Stop()
	defer sources.Stop()

	for {
		select {
		case <-b.done:
			b.releaseSelected()
			return
		case <-b.notify:
			_ = b.refreshDevices()
			b.ensureSelection()
			b.poke()
		case <-levels.C:
			b.readLevels()
		case <-sources.C:
			b.readSources()
		case c := <-b.cmds:
			c.errc <- b.handle(c)
		}
	}
}

func (b *backend) handle(c command) error {
	switch c.kind {
	case "select":
		return b.selectID(c.id)
	case "play":
		return b.play(c.pair, c.side)
	default:
		return fmt.Errorf("unknown command")
	}
}

func (b *backend) poke() {
	select {
	case b.changed <- struct{}{}:
	default:
	}
}

func (b *backend) refreshDevices() error {
	var collection *wca.IMMDeviceCollection
	if err := b.enum.EnumAudioEndpoints(wca.ERender, wca.DEVICE_STATE_ACTIVE, &collection); err != nil {
		return err
	}
	defer collection.Release()

	var defID string
	var def *wca.IMMDevice
	if err := b.enum.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &def); err == nil {
		_ = def.GetId(&defID)
		def.Release()
	}

	var count uint32
	if err := collection.GetCount(&count); err != nil {
		return err
	}
	devices := make([]audio.Device, 0, count)
	for i := uint32(0); i < count; i++ {
		var dev *wca.IMMDevice
		if err := collection.Item(i, &dev); err != nil {
			continue
		}
		id, name := deviceIdentity(dev)
		ch := deviceChannels(dev)
		dev.Release()
		devices = append(devices, audio.Device{
			ID: id, Name: name, Default: id == defID && id != "", Channels: ch, Active: id == defID,
		})
	}
	b.mu.Lock()
	b.devices = devices
	b.mu.Unlock()
	return nil
}

func deviceIdentity(dev *wca.IMMDevice) (string, string) {
	var id string
	_ = dev.GetId(&id)
	var store *wca.IPropertyStore
	name := id
	if err := dev.OpenPropertyStore(wca.STGM_READ, &store); err == nil {
		var pv wca.PROPVARIANT
		if err := store.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err == nil {
			if s := pv.String(); s != "" {
				name = s
			}
			_ = ole.VariantClear(&pv.VARIANT)
		}
		store.Release()
	}
	return id, name
}

func deviceChannels(dev *wca.IMMDevice) int {
	mask, ch := mixLayout(dev)
	if ch > 0 {
		return ch
	}
	_ = mask
	return 2
}

func mixLayout(dev *wca.IMMDevice) (uint32, int) {
	var client *wca.IAudioClient
	if err := dev.Activate(wca.IID_IAudioClient, wca.CLSCTX_ALL, nil, &client); err != nil {
		return 0, 0
	}
	defer client.Release()
	var wfx *wca.WAVEFORMATEX
	if err := client.GetMixFormat(&wfx); err != nil || wfx == nil {
		return 0, 0
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(wfx)))
	return channelMask(wfx), int(wfx.NChannels)
}

func channelMask(wfx *wca.WAVEFORMATEX) uint32 {
	const waveFormatExtensible = 0xFFFE
	if wfx.WFormatTag != waveFormatExtensible || wfx.CbSize < 22 {
		return 0
	}
	mask := unsafe.Add(unsafe.Pointer(wfx), unsafe.Sizeof(*wfx)+2)
	return *(*uint32)(mask)
}

func (b *backend) selectDefault() error {
	b.mu.RLock()
	devices := b.devices
	b.mu.RUnlock()
	for _, d := range devices {
		if d.Default {
			return b.selectID(d.ID)
		}
	}
	if len(devices) > 0 {
		return b.selectID(devices[0].ID)
	}
	return fmt.Errorf("no active playback devices")
}

func (b *backend) ensureSelection() {
	b.mu.RLock()
	devices := append([]audio.Device(nil), b.devices...)
	id := b.selectedID
	b.mu.RUnlock()
	for _, d := range devices {
		if d.ID == id {
			return
		}
	}
	_ = b.selectDefault()
}

func (b *backend) selectID(id string) error {
	if id == "" {
		return b.selectDefault()
	}
	var dev *wca.IMMDevice
	if err := getDevice(b.enum, id, &dev); err != nil {
		return err
	}
	var meter *wca.IAudioMeterInformation
	if err := dev.Activate(wca.IID_IAudioMeterInformation, wca.CLSCTX_ALL, nil, &meter); err != nil {
		dev.Release()
		return err
	}
	mask, ch := mixLayout(dev)
	b.releaseSelected()
	b.selected = dev
	b.meter = meter
	b.mask = mask
	b.channels = ch
	b.selectedID = id
	b.readLevels()
	return nil
}

func (b *backend) releaseSelected() {
	if b.meter != nil {
		b.meter.Release()
		b.meter = nil
	}
	if b.selected != nil {
		b.selected.Release()
		b.selected = nil
	}
}

func (b *backend) readLevels() {
	if b.meter == nil {
		return
	}
	var n uint32
	if err := b.meter.GetMeteringChannelCount(&n); err != nil || n == 0 {
		return
	}
	peaks := make([]float32, n)
	if err := b.meter.GetChannelsPeakValues(n, peaks); err != nil {
		return
	}
	grouped := audio.GroupPeaks(b.mask, peaks)
	b.mu.Lock()
	b.levels = grouped
	b.channels = int(n)
	b.mu.Unlock()
}

func (b *backend) readSources() {
	if b.selected == nil {
		return
	}
	var mgr *wca.IAudioSessionManager2
	if err := b.selected.Activate(wca.IID_IAudioSessionManager2, wca.CLSCTX_ALL, nil, &mgr); err != nil {
		return
	}
	defer mgr.Release()
	var enum *wca.IAudioSessionEnumerator
	if err := mgr.GetSessionEnumerator(&enum); err != nil {
		return
	}
	defer enum.Release()
	var count int
	if err := enum.GetCount(&count); err != nil {
		return
	}
	var sources []audio.Source
	for i := 0; i < count; i++ {
		var ctrl *wca.IAudioSessionControl
		if err := enum.GetSession(i, &ctrl); err != nil {
			continue
		}
		src, ok := b.sessionSource(ctrl)
		ctrl.Release()
		if ok {
			sources = append(sources, src)
		}
	}
	b.mu.Lock()
	b.sources = sources
	b.mu.Unlock()
}

func (b *backend) sessionSource(ctrl *wca.IAudioSessionControl) (audio.Source, bool) {
	disp, err := ctrl.QueryInterface(wca.IID_IAudioSessionControl2)
	if err != nil {
		return audio.Source{}, false
	}
	ctrl2 := (*wca.IAudioSessionControl2)(unsafe.Pointer(disp))
	defer ctrl2.Release()

	var state uint32
	if err := ctrl.GetState(&state); err != nil || state != wca.AudioSessionStateActive {
		return audio.Source{}, false
	}
	meterDisp, err := ctrl.QueryInterface(wca.IID_IAudioMeterInformation)
	if err != nil {
		return audio.Source{}, false
	}
	meter := (*wca.IAudioMeterInformation)(unsafe.Pointer(meterDisp))
	defer meter.Release()
	var peak float32
	if err := meter.GetPeakValue(&peak); err != nil || peak < playingThreshold {
		return audio.Source{}, false
	}

	system := ctrl2.IsSystemSoundsSession() == nil
	var pid uint32
	_ = ctrl2.GetProcessId(&pid)
	if system || pid == 0 {
		return audio.Source{PID: 0, Name: "System sounds", Peak: peak}, true
	}
	name, icon := processInfo(uint32(pid), b.icons)
	return audio.Source{PID: int(pid), Name: name, Icon: icon, Peak: peak}, true
}

func (b *backend) play(pairID, side string) error {
	if b.selected == nil {
		return fmt.Errorf("no device selected")
	}
	if side != "left" && side != "right" && side != "both" {
		return fmt.Errorf("side must be left, right, or both")
	}
	return playTone(b.selected, b.mask, b.channels, pairID, side)
}
