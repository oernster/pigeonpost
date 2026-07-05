//go:build windows

package taskbar

import (
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TrayActions holds the callbacks the tray context menu invokes. They are supplied by the composition
// root, which owns the Wails runtime, so this package stays free of any UI-framework dependency. Open
// restores the window (which may be hidden to the tray, so it goes through the runtime rather than a
// Win32 window search).
type TrayActions struct {
	Open         func()
	About        func()
	Licence      func()
	CheckUpdates func()
	Quit         func()
}

// Tray is a persistent Windows notification-area icon. Left-clicking it restores the main window;
// right-clicking opens a menu mirroring the Help menu plus Open and Quit; and Notify raises a balloon
// for a due reminder. It owns a hidden window on its own message-pumping thread, since tray callbacks
// arrive as window messages on the thread that created the icon.
type Tray struct {
	title    string // main window title, used to find and restore the window
	appName  string // tray icon tooltip
	actions  TrayActions
	balloons chan balloonMsg
	hwnd     atomic.Uintptr
	hIcon    windows.Handle
}

type balloonMsg struct{ title, body string }

// activeTray is the single tray instance the window procedure dispatches to. The tray is a process-wide
// singleton (one notification icon), and a Win32 window procedure is a C callback that cannot carry a Go
// receiver, so the instance is reached through this package variable, set before the window is created.
var activeTray *Tray

// taskbarCreated is the id of the shell broadcast sent when Explorer restarts. The icon is re-added on
// receipt so it survives an Explorer crash. It is registered once on the tray thread.
var taskbarCreated uint32

// trayWndProcCallback is the C-callable window procedure, created once.
var trayWndProcCallback = syscall.NewCallback(trayWndProc)

// NewTray constructs a tray targeting the window with the given title and using appName as its tooltip.
func NewTray(windowTitle, appName string) *Tray {
	return &Tray{title: windowTitle, appName: appName, balloons: make(chan balloonMsg, balloonBuffer)}
}

// CanHideToTray reports whether hiding the window leaves a restorable tray icon. On Windows the tray is
// a persistent clickable icon, so it does.
func (t *Tray) CanHideToTray() bool { return true }

// Start records the menu callbacks and launches the tray's message-pump thread.
func (t *Tray) Start(actions TrayActions) {
	t.actions = actions
	go t.run()
}

// Stop tears the tray down (which removes the icon) by closing its hidden window.
func (t *Tray) Stop() {
	if h := t.hwnd.Load(); h != 0 {
		procPostMessage.Call(h, wmClose, 0, 0)
	}
}

// Notify raises a balloon for a due reminder, unless the main window is already in the foreground (the
// in-app banner covers that case). It hands the text to the tray thread, which owns the icon.
func (t *Tray) Notify(title, body string) {
	if title == "" && body == "" {
		return
	}
	if hwnd := findMainWindow(t.title); hwnd != 0 {
		if fg, _, _ := procGetForegroundWindow.Call(); windows.HWND(fg) == hwnd {
			return
		}
	}
	h := t.hwnd.Load()
	if h == 0 {
		return
	}
	select {
	case t.balloons <- balloonMsg{title: title, body: body}:
		procPostMessage.Call(h, wmShowBalloon, 0, 0)
	default:
	}
}

// run owns the tray thread: it registers a window class, creates the hidden owner window, adds the icon
// and pumps messages until the window is closed, then removes the icon.
func (t *Tray) run() {
	runtime.LockOSThread()
	activeTray = t

	hinst, _, _ := procGetModuleHandle.Call(0)
	className, err := windows.UTF16PtrFromString("PigeonPostTray")
	if err != nil {
		return
	}
	wc := wndClassEx{
		lpfnWndProc:   trayWndProcCallback,
		hInstance:     windows.Handle(hinst),
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ := procCreateWindowEx.Call(
		0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(className)),
		0, 0, 0, 0, 0, 0, 0, hinst, 0)
	if hwnd == 0 {
		return
	}
	t.hwnd.Store(hwnd)
	registerTaskbarCreated()

	t.loadIcon()
	t.addIcon()

	var m msg
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 { // 0 is WM_QUIT, -1 is an error
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	t.deleteIcon()
}

// registerTaskbarCreated resolves the id of the Explorer-restart broadcast message.
func registerTaskbarCreated() {
	name, err := windows.UTF16PtrFromString("TaskbarCreated")
	if err != nil {
		return
	}
	if id, _, _ := procRegisterWindowMsg.Call(uintptr(unsafe.Pointer(name))); id != 0 {
		taskbarCreated = uint32(id)
	}
}

// trayWndProc handles the tray icon's window messages: mouse events on the icon, the balloon-show
// signal, the Explorer-restart broadcast and window destruction. Everything else defers to Windows.
func trayWndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	t := activeTray
	if t == nil {
		r, _, _ := procDefWindowProc.Call(hwnd, message, wParam, lParam)
		return r
	}
	if taskbarCreated != 0 && uint32(message) == taskbarCreated {
		t.addIcon()
		return 0
	}
	switch uint32(message) {
	case wmTrayCallback:
		switch lParam & 0xFFFF {
		case wmLButtonUp, wmLButtonDblClk:
			invoke(t.actions.Open)
		case wmRButtonUp:
			t.showMenu()
		}
		return 0
	case wmShowBalloon:
		t.drainBalloons()
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, message, wParam, lParam)
	return r
}

// showMenu pops up the tray context menu at the cursor and runs the chosen command. The foreground and
// trailing null-message calls are the standard Win32 idiom so the menu dismisses on an outside click.
func (t *Tray) showMenu() {
	h := t.hwnd.Load()
	procSetForegroundWindow.Call(h)
	menu, _, _ := procCreatePopupMenu.Call()
	appendMenuItem(menu, idOpen, "Open PigeonPost")
	appendMenuSeparator(menu)
	appendMenuItem(menu, idAbout, "About PigeonPost")
	appendMenuItem(menu, idLicence, "Licence")
	appendMenuItem(menu, idCheckUpdates, "Check for Updates")
	appendMenuSeparator(menu)
	appendMenuItem(menu, idQuit, "Quit")

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	cmd, _, _ := procTrackPopupMenu.Call(
		menu, tpmReturnCmd|tpmRightButton|tpmNonotify, uintptr(pt.x), uintptr(pt.y), 0, h, 0)
	procPostMessage.Call(h, wmNull, 0, 0)
	procDestroyMenu.Call(menu)
	t.dispatch(uint32(cmd))
}

// dispatch runs the selected menu command. The dialog-opening items restore the window first so the
// dialog is visible if the app was hidden to the tray or minimised.
func (t *Tray) dispatch(cmd uint32) {
	switch cmd {
	case idOpen:
		invoke(t.actions.Open)
	case idAbout:
		invoke(t.actions.Open)
		invoke(t.actions.About)
	case idLicence:
		invoke(t.actions.Open)
		invoke(t.actions.Licence)
	case idCheckUpdates:
		invoke(t.actions.CheckUpdates)
	case idQuit:
		invoke(t.actions.Quit)
	}
}

// invoke runs a menu callback if one was supplied.
func invoke(action func()) {
	if action != nil {
		action()
	}
}

// drainBalloons shows every queued balloon, one per queued reminder batch.
func (t *Tray) drainBalloons() {
	for {
		select {
		case b := <-t.balloons:
			t.showBalloon(b.title, b.body)
		default:
			return
		}
	}
}

// baseNID returns a NOTIFYICONDATAW addressing this tray's single icon.
func (t *Tray) baseNID() notifyIconData {
	var n notifyIconData
	n.cbSize = uint32(unsafe.Sizeof(n))
	n.hWnd = windows.HWND(t.hwnd.Load())
	n.uID = trayIconID
	return n
}

// addIcon installs the tray icon with its tooltip and click-callback message.
func (t *Tray) addIcon() {
	n := t.baseNID()
	n.uFlags = nifMessage | nifIcon | nifTip
	n.uCallbackMessage = wmTrayCallback
	n.hIcon = t.hIcon
	copyUTF16(n.szTip[:], t.appName)
	procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&n)))
}

// showBalloon raises a balloon notification on the existing tray icon.
func (t *Tray) showBalloon(title, body string) {
	n := t.baseNID()
	n.uFlags = nifInfo
	n.dwInfoFlags = niifInfo
	copyUTF16(n.szInfo[:], body)
	copyUTF16(n.szInfoTitle[:], title)
	procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&n)))
}

// deleteIcon removes the tray icon.
func (t *Tray) deleteIcon() {
	n := t.baseNID()
	procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&n)))
}

// loadIcon takes the application icon from the running executable, falling back to the generic
// application icon if the executable carries none.
func (t *Tray) loadIcon() {
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
