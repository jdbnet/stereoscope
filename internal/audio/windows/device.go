//go:build windows

package windows

import (
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

func getDevice(enum *wca.IMMDeviceEnumerator, id string, out **wca.IMMDevice) error {
	ptr, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		return err
	}
	hr, _, _ := syscall.SyscallN(
		enum.VTable().GetDevice,
		uintptr(unsafe.Pointer(enum)),
		uintptr(unsafe.Pointer(ptr)),
		uintptr(unsafe.Pointer(out)),
	)
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

func unregisterNotification(enum *wca.IMMDeviceEnumerator, client *wca.IMMNotificationClient) {
	_, _, _ = syscall.SyscallN(
		enum.VTable().UnregisterEndpointNotificationCallback,
		uintptr(unsafe.Pointer(enum)),
		uintptr(unsafe.Pointer(client)),
	)
}
