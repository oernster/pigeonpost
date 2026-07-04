//go:build windows

package taskbar

import (
	"fmt"
	"image"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// COM identifiers for the taskbar list object and the version 3 interface that carries SetOverlayIcon.
const (
	clsidTaskbarList  = "{56FDF344-FD6D-11D0-958A-006097C9A090}"
	iidITaskbarList3  = "{EA1AFB91-9E28-4B86-90E9-9E9F8A5EEFAF}"
	vtblRelease       = 2  // IUnknown::Release
	vtblHrInit        = 3  // ITaskbarList::HrInit
	vtblSetOverlay    = 18 // ITaskbarList3::SetOverlayIcon
	windowTextMaxRune = 256
	dibRGBColours     = 0 // DIB_RGB_COLORS
)

var (
	modole32  = windows.NewLazySystemDLL("ole32.dll")
	modgdi32  = windows.NewLazySystemDLL("gdi32.dll")
	moduser32 = windows.NewLazySystemDLL("user32.dll")

	procCoCreateInstance   = modole32.NewProc("CoCreateInstance")
	procCreateDIBSection   = modgdi32.NewProc("CreateDIBSection")
	procCreateBitmap       = modgdi32.NewProc("CreateBitmap")
	procDeleteObject       = modgdi32.NewProc("DeleteObject")
	procCreateIconIndirect = moduser32.NewProc("CreateIconIndirect")
	procDestroyIcon        = moduser32.NewProc("DestroyIcon")
	procGetWindowText      = moduser32.NewProc("GetWindowTextW")
)

// Overlay drives the taskbar badge from a single dedicated STA thread, so all COM calls happen on the
// one apartment. Counts arrive on a coalescing channel where only the latest value matters.
type Overlay struct {
	title   string
	updates chan string
}

// NewOverlay constructs an overlay that targets the window with the given title (the main window's
// title, used to locate its HWND).
func NewOverlay(windowTitle string) *Overlay {
	return &Overlay{title: windowTitle, updates: make(chan string, 1)}
}

// Start launches the COM worker. It is called once, before the blocking UI loop, and runs for the life
// of the process.
func (o *Overlay) Start() {
	go o.run()
}

// SetUnread requests the badge show the given total (0 clears it). It never blocks: if an update is
// already queued it is replaced, because only the newest count matters.
func (o *Overlay) SetUnread(total int) {
	label := BadgeLabel(total)
	select {
	case o.updates <- label:
	default:
		select {
		case <-o.updates:
		default:
		}
		select {
		case o.updates <- label:
		default:
		}
	}
}

// run owns the STA thread: it initialises COM, creates the taskbar-list object once, then applies each
// requested label to the window's overlay icon. COM is left initialised for the thread's whole life
// (the thread lives until process exit), which sidesteps re-initialisation edge cases.
func (o *Overlay) run() {
	runtime.LockOSThread()
	// Result is ignored: S_FALSE (already initialised) is fine, and a hard failure simply means the
	// SetOverlayIcon calls below will no-op.
	_ = windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)

	tbl, err := createTaskbarList()
	if err != nil {
		return
	}
	defer tbl.release()

	var (
		current string
		hwnd    windows.HWND
	)
	for label := range o.updates {
		if hwnd == 0 {
			hwnd = findMainWindow(o.title)
		}
		if hwnd == 0 || label == current {
			// The window is not ready yet, or nothing changed; a later update retries.
			continue
		}
		if err := applyOverlay(tbl, hwnd, label); err != nil {
			continue
		}
		current = label
	}
}

// applyOverlay renders the label (or clears it when empty) and sets it as the window's overlay icon,
// destroying the icon afterwards since the shell copies it.
func applyOverlay(tbl *taskbarList, hwnd windows.HWND, label string) error {
	var hicon windows.Handle
	if label != "" {
		img := renderBadge(label)
		icon, err := iconFromImage(img)
		if err != nil {
			return err
		}
		hicon = icon
	}
	tbl.setOverlayIcon(hwnd, hicon, label)
	if hicon != 0 {
		_, _, _ = procDestroyIcon.Call(uintptr(hicon))
	}
	return nil
}

// comVtbl mirrors the leading slots of the ITaskbarList3 method table, up to and including
// SetOverlayIcon, as an array of function addresses. comObject is the memory layout of a COM interface
// pointer: a single pointer to its vtable. Keeping the interface as a typed pointer throughout means
// the method dispatch never converts a uintptr back into an unsafe.Pointer.
type comVtbl struct {
	methods [vtblSetOverlay + 1]uintptr
}

type comObject struct {
	vtbl *comVtbl
}

// taskbarList wraps the COM interface pointer to ITaskbarList3.
type taskbarList struct {
	ptr unsafe.Pointer
}

func createTaskbarList() (*taskbarList, error) {
	clsid, err := windows.GUIDFromString(clsidTaskbarList)
	if err != nil {
		return nil, err
	}
	iid, err := windows.GUIDFromString(iidITaskbarList3)
	if err != nil {
		return nil, err
	}
	var ptr unsafe.Pointer
	r, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsid)),
		0,
		uintptr(windows.CLSCTX_INPROC_SERVER),
		uintptr(unsafe.Pointer(&iid)),
		uintptr(unsafe.Pointer(&ptr)),
	)
	if r != 0 || ptr == nil {
		return nil, fmt.Errorf("CoCreateInstance(TaskbarList): hresult 0x%x", r)
	}
	tbl := &taskbarList{ptr: ptr}
	comCall(ptr, vtblHrInit)
	return tbl, nil
}

func (t *taskbarList) release() { comCall(t.ptr, vtblRelease) }

func (t *taskbarList) setOverlayIcon(hwnd windows.HWND, hicon windows.Handle, desc string) {
	var descPtr uintptr
	if desc != "" {
		if p, err := windows.UTF16PtrFromString(desc); err == nil {
			descPtr = uintptr(unsafe.Pointer(p))
		}
	}
	comCall(t.ptr, vtblSetOverlay, uintptr(hwnd), uintptr(hicon), descPtr)
}

// comCall invokes the method at the given vtable index on a COM interface pointer, passing the
// interface as the implicit first argument.
func comCall(this unsafe.Pointer, index int, args ...uintptr) uintptr {
	obj := (*comObject)(this)
	fn := obj.vtbl.methods[index]
	full := make([]uintptr, 0, len(args)+1)
	full = append(full, uintptr(this))
	full = append(full, args...)
	ret, _, _ := syscall.SyscallN(fn, full...)
	return ret
}

// findMainWindow returns the HWND of this process's main window, matched by exact title. A visible
// top-level window of this process is used as a fallback if the title cannot be matched.
func findMainWindow(title string) windows.HWND {
	pid := windows.GetCurrentProcessId()
	var exact, anyVisible windows.HWND
	cb := syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var wpid uint32
		_, _ = windows.GetWindowThreadProcessId(hwnd, &wpid)
		if wpid != pid || !windows.IsWindowVisible(hwnd) {
			return 1 // keep enumerating
		}
		if anyVisible == 0 {
			anyVisible = hwnd
		}
		if strings.EqualFold(windowText(hwnd), title) {
			exact = hwnd
			return 0 // stop
		}
		return 1
	})
	_ = windows.EnumWindows(cb, nil)
	if exact != 0 {
		return exact
	}
	return anyVisible
}

func windowText(hwnd windows.HWND) string {
	buf := make([]uint16, windowTextMaxRune)
	n, _, _ := procGetWindowText.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf[:n])
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  windows.Handle
	hbmColor windows.Handle
}

// iconFromImage turns a premultiplied RGBA image into an HICON via a 32bpp top-down DIB (per-pixel
// alpha) plus an all-zero monochrome mask, so the shell blends the badge with its rounded, antialiased
// edge intact.
func iconFromImage(img *image.RGBA) (windows.Handle, error) {
	w := int32(img.Bounds().Dx())
	h := int32(img.Bounds().Dy())
	header := bitmapInfoHeader{Size: 40, Width: w, Height: -h, Planes: 1, BitCount: 32}

	var bits unsafe.Pointer
	hbmColor, _, _ := procCreateDIBSection.Call(
		0, uintptr(unsafe.Pointer(&header)), dibRGBColours, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbmColor == 0 || bits == nil {
		return 0, fmt.Errorf("CreateDIBSection failed")
	}
	src := bgraFromImage(img)
	copy(unsafe.Slice((*byte)(bits), len(src)), src)

	maskRowBytes := ((int(w) + 15) / 16) * 2
	mask := make([]byte, maskRowBytes*int(h)) // all zero: transparency comes from the colour alpha
	hbmMask, _, _ := procCreateBitmap.Call(uintptr(w), uintptr(h), 1, 1, uintptr(unsafe.Pointer(&mask[0])))
	if hbmMask == 0 {
		_, _, _ = procDeleteObject.Call(hbmColor)
		return 0, fmt.Errorf("CreateBitmap(mask) failed")
	}

	info := iconInfo{fIcon: 1, hbmMask: windows.Handle(hbmMask), hbmColor: windows.Handle(hbmColor)}
	hicon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	_, _, _ = procDeleteObject.Call(hbmColor)
	_, _, _ = procDeleteObject.Call(hbmMask)
	if hicon == 0 {
		return 0, fmt.Errorf("CreateIconIndirect failed")
	}
	return windows.Handle(hicon), nil
}

// bgraFromImage converts a premultiplied RGBA image to the straight-alpha BGRA byte order Windows
// expects for a 32bpp alpha icon.
func bgraFromImage(img *image.RGBA) []byte {
	pixels := img.Bounds().Dx() * img.Bounds().Dy()
	out := make([]byte, pixels*4)
	for i := 0; i < pixels; i++ {
		r := img.Pix[i*4+0]
		g := img.Pix[i*4+1]
		b := img.Pix[i*4+2]
		a := img.Pix[i*4+3]
		if a != 0 && a != 0xFF {
			r = uint8(uint32(r) * 0xFF / uint32(a))
			g = uint8(uint32(g) * 0xFF / uint32(a))
			b = uint8(uint32(b) * 0xFF / uint32(a))
		}
		out[i*4+0] = b
		out[i*4+1] = g
		out[i*4+2] = r
		out[i*4+3] = a
	}
	return out
}
