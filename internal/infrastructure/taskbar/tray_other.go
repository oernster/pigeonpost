//go:build !windows

package taskbar

// TrayActions holds the callbacks a tray context menu invokes. Off Windows there is no tray icon or
// menu, so the callbacks are never called; the type exists so the composition root compiles identically
// everywhere.
type TrayActions struct {
	Open         func()
	About        func()
	Licence      func()
	CheckUpdates func()
	Quit         func()
}

// Tray off Windows has no persistent icon or menu (those are Windows shell features); it only forwards
// reminder notifications to the platform's notification service, implemented per-OS in the Notify
// method. It satisfies the same contract as the windows implementation so the composition root wires it
// identically everywhere.
type Tray struct {
	appName string // shown as the notification's application name
}

// NewTray returns a tray that only raises notifications. The window title and app icon are unused off
// Windows, where there is no tray icon to show a badge on.
func NewTray(_ string, appName string, _ []byte) *Tray { return &Tray{appName: appName} }

// CanHideToTray reports whether hiding the window leaves a restorable tray icon. Off Windows there is no
// tray icon, so hiding the window would strand it; closing therefore quits instead.
func (t *Tray) CanHideToTray() bool { return false }

// SetUnread does nothing off Windows, where there is no tray icon to badge.
func (t *Tray) SetUnread(int) {}

// Start does nothing off Windows: there is no icon or message loop to run.
func (t *Tray) Start(TrayActions) {}

// Stop does nothing off Windows.
func (t *Tray) Stop() {}
