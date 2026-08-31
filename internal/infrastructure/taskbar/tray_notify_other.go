//go:build !windows && !linux && !darwin

package taskbar

import "github.com/oernster/pigeonpost/internal/infrastructure/sound"

// Notify does nothing on platforms without a supported notification service.
func (t *Tray) Notify(string, string, bool, sound.Kind) {}
