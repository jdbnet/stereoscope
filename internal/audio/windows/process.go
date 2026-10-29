//go:build windows

package windows

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32       = windows.NewLazySystemDLL("shell32.dll")
	user32        = windows.NewLazySystemDLL("user32.dll")
	gdi32         = windows.NewLazySystemDLL("gdi32.dll")
	extractIconEx = shell32.NewProc("ExtractIconExW")
	destroyIcon   = user32.NewProc("DestroyIcon")
	getDC         = user32.NewProc("GetDC")
	releaseDC     = user32.NewProc("ReleaseDC")
	drawIconEx    = user32.NewProc("DrawIconEx")
	createDC      = gdi32.NewProc("CreateCompatibleDC")
	deleteDC      = gdi32.NewProc("DeleteDC")
	createDIB     = gdi32.NewProc("CreateDIBSection")
	selectObject  = gdi32.NewProc("SelectObject")
	deleteObject  = gdi32.NewProc("DeleteObject")
)

func processInfo(pid uint32, cache map[string]string) (string, string) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "Unknown", ""
	}
	defer windows.CloseHandle(h)
	var buf [windows.MAX_PATH]uint16
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "Unknown", ""
	}
	path := windows.UTF16ToString(buf[:size])
	name := filepath.Base(path)
	if icon, ok := cache[path]; ok {
		return name, icon
	}
	icon := exeIcon(path)
	if len(cache) > 64 {
		clear(cache)
	}
	cache[path] = icon
	return name, icon
}

func exeIcon(path string) string {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	var large windows.Handle
	r, _, _ := extractIconEx.Call(uintptr(unsafe.Pointer(ptr)), 0, uintptr(unsafe.Pointer(&large)), 0, 1)
	if r == 0 || large == 0 {
		return ""
	}
	defer destroyIcon.Call(uintptr(large))

	const size = 32
	hdc, _, _ := getDC.Call(0)
	if hdc == 0 {
		return ""
	}
	defer releaseDC.Call(0, hdc)
	mem, _, _ := createDC.Call(hdc)
	if mem == 0 {
		return ""
	}
	defer deleteDC.Call(mem)

	var bmi struct {
		biSize          uint32
		biWidth         int32
		biHeight        int32
		biPlanes        uint16
		biBitCount      uint16
		biCompression   uint32
		biSizeImage     uint32
		biXPelsPerMeter int32
		biYPelsPerMeter int32
		biClrUsed       uint32
		biClrImportant  uint32
	}
	bmi.biSize = uint32(unsafe.Sizeof(bmi))
	bmi.biWidth = size
	bmi.biHeight = -size
	bmi.biPlanes = 1
	bmi.biBitCount = 32
	var bits unsafe.Pointer
	dib, _, _ := createDIB.Call(mem, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if dib == 0 || bits == nil {
		return ""
	}
	defer deleteObject.Call(dib)
	old, _, _ := selectObject.Call(mem, dib)
	defer selectObject.Call(mem, old)
	const diNormal = 0x3
	drawIconEx.Call(mem, 0, 0, uintptr(large), size, size, 0, 0, diNormal)

	raw := unsafe.Slice((*byte)(bits), size*size*4)
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for i := 0; i < size*size; i++ {
		img.Pix[i*4+0] = raw[i*4+2]
		img.Pix[i*4+1] = raw[i*4+1]
		img.Pix[i*4+2] = raw[i*4+0]
		img.Pix[i*4+3] = raw[i*4+3]
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}
