package sound

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// wavHeaderSize is the length of the canonical RIFF/WAVE header the encoder writes.
const wavHeaderSize = 44

// floatTolerance is the slack allowed when comparing computed amplitudes, which are the result of
// floating-point sums and a quantisation to 16 bits.
const floatTolerance = 1e-9

func TestNotificationWAVHasWellFormedHeader(t *testing.T) {
	t.Parallel()
	wav := notificationWAV()
	if len(wav) <= wavHeaderSize {
		t.Fatalf("wav is %d bytes: expected a header plus sample data", len(wav))
	}
	if got := string(wav[0:4]); got != "RIFF" {
		t.Errorf("riff tag = %q, want RIFF", got)
	}
	if got := string(wav[8:12]); got != "WAVE" {
		t.Errorf("format tag = %q, want WAVE", got)
	}
	if got := string(wav[12:16]); got != "fmt " {
		t.Errorf("fmt chunk tag = %q, want %q", got, "fmt ")
	}
	if got := string(wav[36:40]); got != "data" {
		t.Errorf("data chunk tag = %q, want data", got)
	}
	declared := binary.LittleEndian.Uint32(wav[4:8])
	if want := uint32(len(wav) - 8); declared != want {
		t.Errorf("declared riff size = %d, want %d", declared, want)
	}
	dataSize := binary.LittleEndian.Uint32(wav[40:44])
	if want := uint32(len(wav) - wavHeaderSize); dataSize != want {
		t.Errorf("declared data size = %d, want %d", dataSize, want)
	}
}

func TestNotificationWAVDeclaresMono16BitCDAudio(t *testing.T) {
	t.Parallel()
	wav := notificationWAV()
	cases := []struct {
		name string
		got  uint32
		want uint32
	}{
		{"fmt chunk size", binary.LittleEndian.Uint32(wav[16:20]), 16},
		{"format tag", uint32(binary.LittleEndian.Uint16(wav[20:22])), 1},
		{"channels", uint32(binary.LittleEndian.Uint16(wav[22:24])), channels},
		{"sample rate", binary.LittleEndian.Uint32(wav[24:28]), sampleRate},
		{"byte rate", binary.LittleEndian.Uint32(wav[28:32]), sampleRate * channels * bytesPerSlot},
		{"block align", uint32(binary.LittleEndian.Uint16(wav[32:34])), channels * bytesPerSlot},
		{"bits per sample", uint32(binary.LittleEndian.Uint16(wav[34:36])), bitsPerSample},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestNotificationWAVRunsForTheChimeLength(t *testing.T) {
	t.Parallel()
	wav := notificationWAV()
	samples := (len(wav) - wavHeaderSize) / bytesPerSlot
	if want := int(cooLengthSecs * sampleRate); samples != want {
		t.Errorf("sample count = %d, want %d", samples, want)
	}
}

func TestNotificationWAVIsRenderedOnceAndReused(t *testing.T) {
	t.Parallel()
	first, second := notificationWAV(), notificationWAV()
	if !bytes.Equal(first, second) {
		t.Error("successive renderings differ: the chime must be deterministic")
	}
	if &first[0] != &second[0] {
		t.Error("successive calls returned different buffers: the rendering must be cached")
	}
}

// TestNotificationWAVOpensAndClosesInSilence guards against the click a buffer starting or ending at
// full amplitude would produce.
func TestNotificationWAVOpensAndClosesInSilence(t *testing.T) {
	t.Parallel()
	wav := notificationWAV()
	first := int16(binary.LittleEndian.Uint16(wav[wavHeaderSize : wavHeaderSize+bytesPerSlot]))
	last := int16(binary.LittleEndian.Uint16(wav[len(wav)-bytesPerSlot:]))
	if first != 0 {
		t.Errorf("first sample = %d, want 0", first)
	}
	if last != 0 {
		t.Errorf("last sample = %d, want 0", last)
	}
}

func TestRenderNormalisesToPeakLevelWithoutClipping(t *testing.T) {
	t.Parallel()
	buf := render(cooLengthSecs, notificationNotes())
	peak := 0.0
	for _, v := range buf {
		if a := math.Abs(v); a > peak {
			peak = a
		}
	}
	if math.Abs(peak-peakLevel) > floatTolerance {
		t.Errorf("peak = %v, want %v", peak, peakLevel)
	}
	if peak >= 1 {
		t.Errorf("peak = %v: the chime must not clip", peak)
	}
}

func TestRenderPlacesEachNoteAtItsStart(t *testing.T) {
	t.Parallel()
	notes := notificationNotes()
	if len(notes) != 2 {
		t.Fatalf("chime has %d notes, want 2", len(notes))
	}
	buf := render(cooLengthSecs, notes)
	// The second note starts after the first has decayed, so the sample just before it must be
	// quieter than the peak reached shortly after it is voiced.
	start := int(notes[1].startSecs * sampleRate)
	before := math.Abs(buf[start-1])
	after := 0.0
	for i := start; i < start+int(notes[1].attackSecs*sampleRate)*2; i++ {
		if a := math.Abs(buf[i]); a > after {
			after = a
		}
	}
	if after <= before {
		t.Errorf("second note peaked at %v, no louder than the %v preceding it", after, before)
	}
}

func TestRenderIgnoresNotesWithoutLength(t *testing.T) {
	t.Parallel()
	if got := render(0, notificationNotes()); len(got) != 0 {
		t.Errorf("zero-length render produced %d samples, want 0", len(got))
	}
}

func TestEnvelopeRisesThenDecays(t *testing.T) {
	t.Parallel()
	const attack, decay = 0.02, 0.10
	cases := []struct {
		name string
		t    float64
		want float64
	}{
		{"before the note", -0.01, 0},
		{"at the start", 0, 0},
		{"mid attack", attack / 2, 0.5},
		{"at full level", attack, 1},
		{"one time constant after attack", attack + decay, math.Exp(-1)},
	}
	for _, c := range cases {
		if got := envelope(c.t, attack, decay); math.Abs(got-c.want) > floatTolerance {
			t.Errorf("%s: envelope = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestTimbreAddsPartialsToTheFundamental(t *testing.T) {
	t.Parallel()
	const freq = 100.0
	quarterCycle := 1 / (4 * freq)
	// At a quarter cycle the fundamental is at its maximum and the octave partial is back at zero,
	// so the sum is the fundamental alone.
	if got := timbre(quarterCycle, freq, cooPartials); math.Abs(got-1) > 1e-6 {
		t.Errorf("timbre at the fundamental's peak = %v, want 1", got)
	}
	if got := timbre(0, freq, nil); math.Abs(got) > floatTolerance {
		t.Errorf("timbre at zero = %v, want 0", got)
	}
}

func TestFadeTailRampsTheEndToSilence(t *testing.T) {
	t.Parallel()
	fade := int(tailFadeSecs * sampleRate)
	buf := make([]float64, fade*2)
	for i := range buf {
		buf[i] = 1
	}
	fadeTail(buf)
	if buf[fade-1] != 1 {
		t.Errorf("sample before the fade = %v, want 1", buf[fade-1])
	}
	if buf[len(buf)-1] != 0 {
		t.Errorf("last sample = %v, want 0", buf[len(buf)-1])
	}
	if buf[len(buf)-fade/2] >= buf[len(buf)-fade] {
		t.Error("the fade is not monotonically decreasing")
	}
}

func TestFadeTailLeavesABufferShorterThanTheFadeAlone(t *testing.T) {
	t.Parallel()
	buf := []float64{1, 1}
	fadeTail(buf)
	for i, v := range buf {
		if v != 1 {
			t.Errorf("sample %d = %v, want 1: too short a buffer must be left untouched", i, v)
		}
	}
}

func TestNormaliseLeavesSilenceAlone(t *testing.T) {
	t.Parallel()
	buf := make([]float64, 8)
	for i, v := range normalise(buf) {
		if v != 0 {
			t.Errorf("sample %d = %v, want 0", i, v)
		}
	}
}

func TestNormaliseScalesQuietAndLoudBuffersToTheSameLevel(t *testing.T) {
	t.Parallel()
	quiet := normalise([]float64{0.01, -0.005})
	loud := normalise([]float64{9, -4.5})
	if math.Abs(quiet[0]-peakLevel) > floatTolerance {
		t.Errorf("quiet peak = %v, want %v", quiet[0], peakLevel)
	}
	if math.Abs(loud[0]-peakLevel) > floatTolerance {
		t.Errorf("loud peak = %v, want %v", loud[0], peakLevel)
	}
	if math.Abs(quiet[1]-loud[1]) > floatTolerance {
		t.Errorf("relative shape diverged: %v against %v", quiet[1], loud[1])
	}
}

func TestEncodeWAVWritesSamplesAsSigned16Bit(t *testing.T) {
	t.Parallel()
	wav := encodeWAV([]float64{0, 1, -1})
	want := []int16{0, maxSample, -maxSample}
	for i, w := range want {
		off := wavHeaderSize + i*bytesPerSlot
		got := int16(binary.LittleEndian.Uint16(wav[off : off+bytesPerSlot]))
		if got != w {
			t.Errorf("sample %d = %d, want %d", i, got, w)
		}
	}
}

func TestPlayToleratesAnEmptyBuffer(t *testing.T) {
	t.Parallel()
	play(nil)
}
