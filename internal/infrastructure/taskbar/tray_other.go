//go:build !windows

package taskbar

// TrayActions holds the callbacks a tray context menu invokes. Off Windows there is no tray, so the
// callbacks are never called; the type exists so the composition root compiles identically everywhere.
type TrayActions struct {
	About        func()
	Licence      func()
	CheckUpdates func()
	Quit         func()
}

// Tray is a no-op on platforms without a Windows notification tray. It satisfies the same contract as
// the windows implementation so the composition root wires it identically everywhere.
type Tray struct{}

// NewTray returns a no-op tray. The window title and app name are ignored off Windows.
func NewTray(string, string) *Tray { return &Tray{} }

// Start does nothing off Windows.
func (t *Tray) Start(TrayActions) {}

// Notify does nothing off Windows.
func (t *Tray) Notify(string, string) {}

// Stop does nothing off Windows.
func (t *Tray) Stop() {}
