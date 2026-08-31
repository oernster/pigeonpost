package sound

import "sync"

// Kind is what a chime is announcing. Each kind has its own voice, so the three things PigeonPost
// raises a notification for are told apart by ear without reading the notification. The voices differ
// by note count and rhythm rather than by pitch alone, since that is what a listener actually
// distinguishes across small speakers and background noise: three rising notes, two knocks on one
// pitch, one slow low note.
type Kind int

const (
	// NewMail announces arriving mail.
	NewMail Kind = iota
	// Reminder announces a calendar reminder falling due.
	Reminder
	// Resurfaced announces a snoozed message coming back.
	Resurfaced
)

// Timbre constants. warmPartials is an almost pure tone with one quiet octave above it, which reads as
// wooden rather than metallic; brightPartials adds the third harmonic, which is what makes the reminder
// carry across a room; deepPartials lifts a low note's harmonics so it stays audible on the small
// speakers a laptop has, where the fundamental alone would be felt more than heard.
var (
	warmPartials   = []partial{{ratio: 2, level: 0.08}}
	brightPartials = []partial{{ratio: 2, level: 0.10}, {ratio: 3, level: 0.14}}
	deepPartials   = []partial{{ratio: 2, level: 0.22}, {ratio: 3, level: 0.09}}
)

// New mail is three short wooden notes climbing a major triad. A rising run of three is the shape
// furthest from the soft falling pair every desktop assistant and notification tool reaches for, which
// is the collision this voice exists to avoid.
const (
	newMailLengthSecs = 1.00
	newMailFirstFreq  = 349.23 // F4
	newMailSecondFreq = 440.00 // A4
	newMailThirdFreq  = 523.25 // C5
	newMailStepSecs   = 0.10   // the gap between notes, short enough to read as one figure
	newMailAttackSecs = 0.012
	newMailDecaySecs  = 0.19
	newMailFirstLevel = 1.0
	newMailStepLevel  = 0.05 // each note a shade quieter than the one below it
)

// A reminder is two quick knocks on a single bright pitch. Repeating one note rather than moving
// between two is what separates it from the mail chime at a glance; the brighter timbre suits
// something that wants answering now.
const (
	reminderLengthSecs      = 0.90
	reminderFreq            = 587.33 // D5
	reminderSecondStartSecs = 0.17
	reminderAttackSecs      = 0.006 // fast enough to read as struck rather than breathed
	reminderDecaySecs       = 0.12
	reminderFirstLevel      = 1.0
	reminderSecondLevel     = 0.95
)

// A resurfaced snooze is one low note with a slow rise: the quietest of the three, because the message
// coming back is something the user asked for at a time they chose, not news. A single note cannot be
// confused with either of the others, both of which are more than one.
const (
	resurfacedLengthSecs = 1.30
	resurfacedFreq       = 261.63 // C4
	resurfacedAttackSecs = 0.090  // slow enough to sound breathed in rather than struck
	resurfacedDecaySecs  = 0.40
	resurfacedLevel      = 1.0
)

// wavFor renders a kind's chime once and reuses it. Rendering is cheap but not free, the bytes never
// change, so holding a single buffer per kind for the life of the process is what makes asynchronous
// playback safe on Windows: the buffer outlives every call that reads it.
var wavFor = func() func(Kind) []byte {
	cached := map[Kind]func() []byte{
		NewMail:    sync.OnceValue(func() []byte { return encodeWAV(render(newMailLengthSecs, newMailNotes())) }),
		Reminder:   sync.OnceValue(func() []byte { return encodeWAV(render(reminderLengthSecs, reminderNotes())) }),
		Resurfaced: sync.OnceValue(func() []byte { return encodeWAV(render(resurfacedLengthSecs, resurfacedNotes())) }),
	}
	return func(k Kind) []byte {
		render, ok := cached[k]
		if !ok {
			// An unknown kind is a programming error, not a runtime condition. Sounding new mail rather
			// than falling silent keeps a miswired call audible instead of hiding it.
			return cached[NewMail]()
		}
		return render()
	}
}()

// Play sounds the chime for what is being announced. It is a no-op on platforms where the notification
// sound is left to the desktop's own notification service.
func Play(kind Kind) { play(wavFor(kind)) }

// newMailNotes is the arriving-mail score: three notes climbing at a fixed step.
func newMailNotes() []note {
	freqs := []float64{newMailFirstFreq, newMailSecondFreq, newMailThirdFreq}
	notes := make([]note, 0, len(freqs))
	for i, freq := range freqs {
		notes = append(notes, note{
			startSecs:  float64(i) * newMailStepSecs,
			freq:       freq,
			level:      newMailFirstLevel - float64(i)*newMailStepLevel,
			attackSecs: newMailAttackSecs,
			decaySecs:  newMailDecaySecs,
			partials:   warmPartials,
		})
	}
	return notes
}

// reminderNotes is the reminder score: the same pitch struck twice.
func reminderNotes() []note {
	return []note{
		{
			startSecs:  0,
			freq:       reminderFreq,
			level:      reminderFirstLevel,
			attackSecs: reminderAttackSecs,
			decaySecs:  reminderDecaySecs,
			partials:   brightPartials,
		},
		{
			startSecs:  reminderSecondStartSecs,
			freq:       reminderFreq,
			level:      reminderSecondLevel,
			attackSecs: reminderAttackSecs,
			decaySecs:  reminderDecaySecs,
			partials:   brightPartials,
		},
	}
}

// resurfacedNotes is the snooze-returned score: one low note, slowly voiced.
func resurfacedNotes() []note {
	return []note{{
		startSecs:  0,
		freq:       resurfacedFreq,
		level:      resurfacedLevel,
		attackSecs: resurfacedAttackSecs,
		decaySecs:  resurfacedDecaySecs,
		partials:   deepPartials,
	}}
}
