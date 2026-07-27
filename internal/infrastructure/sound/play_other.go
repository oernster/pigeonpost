//go:build !windows

package sound

// play does nothing away from Windows. The macOS and Linux notification services choose the sound
// that accompanies a notification themselves, from the desktop's own theme, so PigeonPost has no
// collision to fix there and no business overriding the user's choice.
func play([]byte) {}
