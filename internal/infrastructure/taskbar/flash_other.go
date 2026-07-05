//go:build !windows

package taskbar

// Flasher is a no-op on platforms without a Windows taskbar button to flash. It satisfies the same
// contract as the windows implementation so the composition root wires it identically everywhere.
type Flasher struct{}

// NewFlasher returns a no-op flasher. The window title is ignored off Windows.
func NewFlasher(string) *Flasher { return &Flasher{} }

// Flash does nothing off Windows.
func (f *Flasher) Flash() {}
