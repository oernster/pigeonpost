//go:build windows

package taskbar

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// prepareIcon decodes the app icon PNG and scales it for compositing with the unread badge. If the PNG
// cannot be decoded it falls back to the executable's own icon (with no badge available), and failing
// that the generic application icon.
func (t *Tray) prepareIcon() {
	if img, err := decodeScaledIcon(t.iconPNG, trayIconPx); err == nil {
		t.baseImg = img
		return
	}
	if exe, err := os.Executable(); err == nil {
		if p, perr := windows.UTF16PtrFromString(exe); perr == nil {
			var large, small windows.Handle
			n, _, _ := procExtractIconEx.Call(
				uintptr(unsafe.Pointer(p)), 0,
				uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&small)), 1)
			if n > 0 && small != 0 {
				t.hIcon = small
				return
			}
			if large != 0 {
				t.hIcon = large
				return
			}
		}
	}
	h, _, _ := procLoadIcon.Call(0, idiApplication)
	t.hIcon = windows.Handle(h)
}

// buildIcon returns the tray icon for the given unread total and whether the caller owns it (must
// destroy it after use). With a decoded base image it composites the app icon and, for a non-zero
// total, the red count badge into a freshly created icon; otherwise it returns the shared fallback
// handle, which must not be destroyed.
func (t *Tray) buildIcon(unread int) (windows.Handle, bool) {
	if t.baseImg == nil {
		return t.hIcon, false
	}
	img := t.baseImg
	if label := BadgeLabel(unread); label != "" {
		img = compositeBadge(t.baseImg, renderBadge(label))
	}
	icon, err := iconFromImage(img)
	if err != nil {
		return t.hIcon, false
	}
	return icon, true
}

// refreshIcon rebuilds the tray icon for the current unread total and applies it, freeing the icon
// afterwards since the shell copies it.
func (t *Tray) refreshIcon() {
	icon, owned := t.buildIcon(int(t.unread.Load()))
	n := t.baseNID()
	n.uFlags = nifIcon
	n.hIcon = icon
	procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&n)))
	if owned {
		procDestroyIcon.Call(uintptr(icon))
	}
}
