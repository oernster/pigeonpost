//go:build !windows

package taskbar

// Overlay is a no-op on platforms without a Windows taskbar overlay badge. It satisfies the same
// contract as the windows implementation so the composition root wires it identically everywhere.
type Overlay struct{}

// NewOverlay returns a no-op overlay. The window title is ignored off Windows.
func NewOverlay(string) *Overlay { return &Overlay{} }

// Start does nothing off Windows.
func (o *Overlay) Start() {}

// SetUnread does nothing off Windows.
func (o *Overlay) SetUnread(int) {}
