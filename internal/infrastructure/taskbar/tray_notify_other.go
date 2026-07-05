//go:build !windows && !linux && !darwin

package taskbar

// Notify does nothing on platforms without a supported notification service.
func (t *Tray) Notify(string, string) {}
