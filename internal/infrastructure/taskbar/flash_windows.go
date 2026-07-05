//go:build windows

package taskbar

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Taskbar-flash flags from winuser.h: flash both the caption and the taskbar button, and keep flashing
// until the window is brought to the foreground.
const (
	flashwAll       = 0x00000003 // FLASHW_ALL: caption plus taskbar button
	flashwTimerNoFG = 0x0000000C // FLASHW_TIMERNOFG: flash until the window comes to the foreground
)

var (
	procFlashWindowEx       = moduser32.NewProc("FlashWindowEx")
	procGetForegroundWindow = moduser32.NewProc("GetForegroundWindow")
)

// flashInfo mirrors the Win32 FLASHWINFO structure passed to FlashWindowEx.
type flashInfo struct {
	cbSize    uint32
	hwnd      windows.HWND
	dwFlags   uint32
	uCount    uint32
	dwTimeout uint32
}

// Flasher flashes the main window's taskbar button to draw attention to a background event, namely a
// calendar reminder that has come due while the window is not in the foreground. It locates the window
// by title, exactly as Overlay does, so it is given the same window title.
type Flasher struct {
	title string
}

// NewFlasher constructs a flasher targeting the window with the given title.
func NewFlasher(windowTitle string) *Flasher { return &Flasher{title: windowTitle} }

// Flash flashes the taskbar button until the user focuses the window. It is a no-op when the window is
// already in the foreground (the user is looking at it) or cannot be found, so a reminder that fires
// while the app is in view does not flash needlessly. FlashWindowEx posts to the window, so it is safe
// to call from the scheduler goroutine without an STA thread.
func (f *Flasher) Flash() {
	hwnd := findMainWindow(f.title)
	if hwnd == 0 {
		return
	}
	if fg, _, _ := procGetForegroundWindow.Call(); windows.HWND(fg) == hwnd {
		return
	}
	info := flashInfo{
		cbSize:  uint32(unsafe.Sizeof(flashInfo{})),
		hwnd:    hwnd,
		dwFlags: flashwAll | flashwTimerNoFG,
	}
	_, _, _ = procFlashWindowEx.Call(uintptr(unsafe.Pointer(&info)))
}
