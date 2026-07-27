//go:build windows

package sound

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// PlaySound flags. sndMemory reads the sound from a WAV image already in memory, so nothing has to be
// written to disk or resolved from the install directory; sndAsync returns as soon as playback starts,
// so the tray's message pump is never blocked; and sndNoDefault suppresses the system default beep if
// the buffer cannot be played at all, since falling back to the shell's sound would reintroduce the
// very collision this chime exists to remove.
const (
	sndAsync     = 0x0001
	sndNoDefault = 0x0002
	sndMemory    = 0x0004
)

var (
	modwinmm      = windows.NewLazySystemDLL("winmm.dll")
	procPlaySound = modwinmm.NewProc("PlaySoundW")
)

// play sounds the chime through the default audio device. Asynchronous playback reads the buffer
// after this returns, so the buffer has to outlive the call: it does, because it is the package's
// single cached rendering, which lives for the life of the process and is never rewritten. A second
// call while the first is still sounding replaces it rather than layering on top, which is the right
// behaviour for a burst of arriving mail.
func play(wav []byte) {
	if len(wav) == 0 {
		return
	}
	procPlaySound.Call(uintptr(unsafe.Pointer(&wav[0])), 0, sndMemory|sndAsync|sndNoDefault)
}
