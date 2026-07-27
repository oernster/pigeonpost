// Package sound gives PigeonPost its own notification voice. The Windows shell plays one default
// sound for every balloon notification, so a new message is indistinguishable by ear from any other
// app or tool raising a stock notification. The tray therefore silences the shell's sound and plays
// the chime rendered here instead.
//
// The chime is synthesised rather than shipped as an asset: it is a pure function of a handful of
// named constants, so there is no binary blob in the repository, nothing to resolve at runtime across
// the dev, Wails and packaged builds, and the result is unit testable.
package sound

import (
	"encoding/binary"
	"math"
	"sync"
)

// PCM format of the rendered chime: CD-quality mono, the one format every platform plays without a
// codec.
const (
	sampleRate    = 44100
	bitsPerSample = 16
	channels      = 1
	bytesPerSlot  = bitsPerSample / 8
	maxSample     = 32767.0
)

// Mix constants. peakLevel keeps the chime well below full scale so it sits under speech and music
// rather than cutting across them, and tailFadeSecs removes the click a hard buffer end would
// otherwise produce.
const (
	peakLevel    = 0.32
	tailFadeSecs = 0.02
)

// The chime is a soft falling pair, the shape of a pigeon's call. It is deliberately low and wooden
// where the system notification sounds are bright and glassy, since telling the two apart by ear is
// the whole point of having our own.
const (
	cooLengthSecs      = 1.10
	cooFirstFreq       = 392.00 // G4
	cooSecondFreq      = 311.13 // Eb4, the minor third below that gives the call its falling shape
	cooSecondStartSecs = 0.26
	cooAttackSecs      = 0.030 // slow enough to sound breathed rather than struck
	cooFirstDecaySecs  = 0.16
	cooSecondDecaySecs = 0.22
	cooFirstLevel      = 1.0
	cooSecondLevel     = 0.9
)

// partial is one harmonic of a note: a multiple of the fundamental at a fraction of its amplitude.
type partial struct {
	ratio float64
	level float64
}

// note is one voiced tone: when it starts, its pitch, how loud it is and how its amplitude rises
// then falls.
type note struct {
	startSecs  float64
	freq       float64
	level      float64
	attackSecs float64
	decaySecs  float64 // exponential time constant, not a hard cutoff
	partials   []partial
}

// cooPartials is an almost pure tone with one quiet octave above it, which reads as warm and wooden
// rather than metallic.
var cooPartials = []partial{{ratio: 2, level: 0.08}}

// notificationWAV renders the chime once and reuses it. Rendering is cheap but not free, the bytes
// never change, and holding a single buffer for the life of the process is what makes asynchronous
// playback safe on Windows.
var notificationWAV = sync.OnceValue(func() []byte {
	return encodeWAV(render(cooLengthSecs, notificationNotes()))
})

// Play sounds PigeonPost's notification chime. It is a no-op on platforms where the notification
// sound is left to the desktop's own notification service.
func Play() { play(notificationWAV()) }

// notificationNotes is the chime's score.
func notificationNotes() []note {
	return []note{
		{
			startSecs:  0,
			freq:       cooFirstFreq,
			level:      cooFirstLevel,
			attackSecs: cooAttackSecs,
			decaySecs:  cooFirstDecaySecs,
			partials:   cooPartials,
		},
		{
			startSecs:  cooSecondStartSecs,
			freq:       cooSecondFreq,
			level:      cooSecondLevel,
			attackSecs: cooAttackSecs,
			decaySecs:  cooSecondDecaySecs,
			partials:   cooPartials,
		},
	}
}

// render sums the notes into a buffer of samples in the range -1 to 1.
func render(lengthSecs float64, notes []note) []float64 {
	total := int(lengthSecs * sampleRate)
	buf := make([]float64, total)
	for _, n := range notes {
		start := int(n.startSecs * sampleRate)
		for i := start; i < total; i++ {
			t := float64(i-start) / sampleRate
			buf[i] += n.level * envelope(t, n.attackSecs, n.decaySecs) * timbre(t, n.freq, n.partials)
		}
	}
	fadeTail(buf)
	return normalise(buf)
}

// envelope is the amplitude shape of a voiced note: a short rise to full level, then an exponential
// fall that never quite reaches zero, which is what makes it sound sounded rather than switched on.
func envelope(t, attackSecs, decaySecs float64) float64 {
	if t < 0 {
		return 0
	}
	if t < attackSecs {
		return t / attackSecs
	}
	return math.Exp(-(t - attackSecs) / decaySecs)
}

// timbre sums the fundamental and its partials at one instant.
func timbre(t, freq float64, partials []partial) float64 {
	v := math.Sin(2 * math.Pi * freq * t)
	for _, p := range partials {
		v += p.level * math.Sin(2*math.Pi*freq*p.ratio*t)
	}
	return v
}

// fadeTail ramps the last few milliseconds to silence so the end of the buffer produces no click.
func fadeTail(buf []float64) {
	fade := int(tailFadeSecs * sampleRate)
	if fade <= 0 || fade > len(buf) {
		return
	}
	// The ramp is measured from the last sample rather than from one past it, so the buffer ends on
	// exact silence instead of on a small residual.
	last := len(buf) - 1
	for i := len(buf) - fade; i < len(buf); i++ {
		buf[i] *= float64(last-i) / float64(fade)
	}
}

// normalise scales the buffer so its loudest point sits at peakLevel, which fixes the chime's
// loudness independently of how many notes were summed and guarantees it cannot clip.
func normalise(buf []float64) []float64 {
	peak := 0.0
	for _, v := range buf {
		if a := math.Abs(v); a > peak {
			peak = a
		}
	}
	if peak == 0 {
		return buf
	}
	scale := peakLevel / peak
	for i := range buf {
		buf[i] *= scale
	}
	return buf
}

// encodeWAV wraps the samples as 16-bit PCM in the canonical 44-byte RIFF/WAVE header.
func encodeWAV(samples []float64) []byte {
	const (
		headerSize    = 44
		fmtChunkSize  = 16
		pcmFormat     = 1
		riffPrefixLen = 8 // the RIFF tag and size field, which the declared size excludes
	)
	dataSize := len(samples) * bytesPerSlot * channels
	out := make([]byte, 0, headerSize+dataSize)
	out = append(out, []byte("RIFF")...)
	out = binary.LittleEndian.AppendUint32(out, uint32(headerSize-riffPrefixLen+dataSize))
	out = append(out, []byte("WAVEfmt ")...)
	out = binary.LittleEndian.AppendUint32(out, fmtChunkSize)
	out = binary.LittleEndian.AppendUint16(out, pcmFormat)
	out = binary.LittleEndian.AppendUint16(out, channels)
	out = binary.LittleEndian.AppendUint32(out, sampleRate)
	out = binary.LittleEndian.AppendUint32(out, sampleRate*channels*bytesPerSlot)
	out = binary.LittleEndian.AppendUint16(out, channels*bytesPerSlot)
	out = binary.LittleEndian.AppendUint16(out, bitsPerSample)
	out = append(out, []byte("data")...)
	out = binary.LittleEndian.AppendUint32(out, uint32(dataSize))
	for _, s := range samples {
		out = binary.LittleEndian.AppendUint16(out, uint16(int16(s*maxSample)))
	}
	return out
}
