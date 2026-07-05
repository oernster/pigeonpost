package domain

import "errors"

// Passthrough component kinds: the non-event calendar components PigeonPost preserves verbatim rather
// than modelling.
const (
	PassthroughToDo    = "VTODO"
	PassthroughJournal = "VJOURNAL"
)

var (
	// ErrEmptyPassthroughUID is returned when a passthrough component has no identifier.
	ErrEmptyPassthroughUID = errors.New("calendar passthrough uid must not be empty")
	// ErrUnknownPassthroughKind is returned for a kind that is not a preserved component type.
	ErrUnknownPassthroughKind = errors.New("calendar passthrough kind must be VTODO or VJOURNAL")
	// ErrEmptyPassthroughRaw is returned when a passthrough component has no serialised content.
	ErrEmptyPassthroughRaw = errors.New("calendar passthrough raw content must not be empty")
)

// CalendarPassthrough is a calendar component PigeonPost does not model (a to-do or a journal entry)
// preserved verbatim so it survives an import and export round-trip. It is identified by its UID, so a
// re-import replaces rather than duplicates it, and it carries its kind and its serialised iCalendar
// text (a standalone VCALENDAR wrapping the single component).
type CalendarPassthrough struct {
	uid  string
	kind string
	raw  string
}

// NewCalendarPassthrough builds a passthrough component, validating that it has an id, a known kind and
// serialised content.
func NewCalendarPassthrough(uid, kind, raw string) (CalendarPassthrough, error) {
	if uid == "" {
		return CalendarPassthrough{}, ErrEmptyPassthroughUID
	}
	if kind != PassthroughToDo && kind != PassthroughJournal {
		return CalendarPassthrough{}, ErrUnknownPassthroughKind
	}
	if raw == "" {
		return CalendarPassthrough{}, ErrEmptyPassthroughRaw
	}
	return CalendarPassthrough{uid: uid, kind: kind, raw: raw}, nil
}

// UID returns the component's identifier, used to replace it on a re-import.
func (c CalendarPassthrough) UID() string { return c.uid }

// Kind returns the component type, VTODO or VJOURNAL.
func (c CalendarPassthrough) Kind() string { return c.kind }

// Raw returns the serialised iCalendar text of the component.
func (c CalendarPassthrough) Raw() string { return c.raw }
