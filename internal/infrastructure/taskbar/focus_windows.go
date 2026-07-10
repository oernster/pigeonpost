//go:build windows

package taskbar

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// gwChild selects a window's first child; webviewHostClass is the WebView2 host window class that must
// hold keyboard focus; classNameMaxRune bounds the class-name buffer.
const (
	gwChild          = 5 // GW_CHILD
	webviewHostClass = "Chrome_WidgetWin_1"
	classNameMaxRune = 256
)

// procSetForegroundWindow is declared alongside the tray's Win32 procs; the rest are used only here.
var (
	procSetFocus          = moduser32.NewProc("SetFocus")
	procAttachThreadInput = moduser32.NewProc("AttachThreadInput")
	procEnumChildWindows  = moduser32.NewProc("EnumChildWindows")
	procGetClassName      = moduser32.NewProc("GetClassNameW")
	procGetWindow         = moduser32.NewProc("GetWindow")
)

// FocusMainWindow gives keyboard focus to this process's main window and to the WebView2 control nested
// inside it. On Windows the WebView2 control does not take keyboard focus when the window first appears, so a
// cold launch drops every keystroke (including the first Tab) until the user clicks; forcing focus here
// makes the keyboard work from launch. It is best-effort: every failure is ignored.
func FocusMainWindow(title string) {
	hwnd := findMainWindow(title)
	if hwnd == 0 {
		return
	}
	target := findWebViewChild(hwnd)
	if target == 0 {
		target = hwnd
	}
	// SetFocus only takes effect from the thread that owns the target window's input queue, so attach this
	// thread to it for the duration. Bring the window to the foreground first so the focus change sticks.
	winThread, _ := windows.GetWindowThreadProcessId(hwnd, nil)
	curThread := windows.GetCurrentThreadId()
	if winThread != 0 && winThread != curThread {
		_, _, _ = procAttachThreadInput.Call(uintptr(curThread), uintptr(winThread), 1)
		defer procAttachThreadInput.Call(uintptr(curThread), uintptr(winThread), 0)
	}
	_, _, _ = procSetForegroundWindow.Call(uintptr(hwnd))
	_, _, _ = procSetFocus.Call(uintptr(target))
}

// findWebViewChild returns the WebView2 host window nested in parent; failing that, the parent's first
// child. WebView2 hosts the page in a child window of class "Chrome_WidgetWin_1"; focusing it routes
// keyboard input to the page.
func findWebViewChild(parent windows.HWND) windows.HWND {
	var found windows.HWND
	cb := syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		if className(hwnd) == webviewHostClass {
			found = hwnd
			return 0 // stop enumerating
		}
		return 1 // keep enumerating
	})
	_, _, _ = procEnumChildWindows.Call(uintptr(parent), cb, 0)
	if found != 0 {
		return found
	}
	child, _, _ := procGetWindow.Call(uintptr(parent), gwChild)
	return windows.HWND(child)
}

// className returns the window-class name of a window.
func className(hwnd windows.HWND) string {
	buf := make([]uint16, classNameMaxRune)
	n, _, _ := procGetClassName.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf[:n])
}
