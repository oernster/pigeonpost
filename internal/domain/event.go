package domain

import (
	"strings"
	"time"
)

// EventInput carries the fields for constructing an Event. ID, Summary and Start are required; the rest
// are optional. Times are supplied as already-resolved values; the domain never reads the wall clock.
type EventInput struct {
	ID          string
	UID         string
	CalendarID  string
	Summary     string
	Description string
	Location    string
	Start       time.Time
	End         time.Time
	AllDay      bool
	Recurrence  string
	// RDates are extra occurrence start times added to the recurrence set (RFC 5545 RDATE). ExDates are
	// occurrence start times removed from it (EXDATE). Both are optional and independent of Recurrence.
	RDates  []time.Time
	ExDates []time.Time
	// RecurrenceID marks this event as an override of a single occurrence of the series sharing its UID.
	// It holds the original start of the occurrence being replaced; the zero time means "not an override".
	RecurrenceID time.Time
	// Extra is an opaque store of the original ICS VEVENT so import and export never strip the
	// properties PigeonPost does not model yet (categories, status, alarms and the rest). It is empty
	// for an event created in the app and is round-tripped unchanged through storage and the UI.
	Extra string
}

// Event is a single calendar entry. It is immutable once constructed. UID carries the ICS UID for a
// lossless round-trip and may be empty on a new event; Recurrence holds the raw RRULE text. Expansion of
// the recurrence set into concrete instances is performed outside the domain by an injected expander.
type Event struct {
	id           string
	uid          string
	calendarID   string
	summary      string
	description  string
	location     string
	start        time.Time
	end          time.Time
	allDay       bool
	recurrence   string
	rdates       []time.Time
	exdates      []time.Time
	recurrenceID time.Time
	extra        string
}

// NewEvent validates and constructs an event. An end before the start is rejected; an unset (zero) end
// is allowed and means an open-ended or point-in-time event.
func NewEvent(in EventInput) (Event, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return Event{}, ErrEmptyEventID
	}
	summary := strings.TrimSpace(in.Summary)
	if summary == "" {
		return Event{}, ErrEmptyEventSummary
	}
	if in.Start.IsZero() {
		return Event{}, ErrEmptyEventStart
	}
	if !in.End.IsZero() && in.End.Before(in.Start) {
		return Event{}, ErrEventEndsBeforeStart
	}
	return Event{
		id:           id,
		uid:          strings.TrimSpace(in.UID),
		calendarID:   strings.TrimSpace(in.CalendarID),
		summary:      summary,
		description:  strings.TrimSpace(in.Description),
		location:     strings.TrimSpace(in.Location),
		start:        in.Start,
		end:          in.End,
		allDay:       in.AllDay,
		recurrence:   strings.TrimSpace(in.Recurrence),
		rdates:       copyNonZeroTimes(in.RDates),
		exdates:      copyNonZeroTimes(in.ExDates),
		recurrenceID: in.RecurrenceID,
		extra:        in.Extra,
	}, nil
}

// copyNonZeroTimes returns a fresh slice holding the non-zero entries of src, so the constructed Event
// neither shares backing storage with the caller nor carries zero placeholders into the recurrence set.
func copyNonZeroTimes(src []time.Time) []time.Time {
	if len(src) == 0 {
		return nil
	}
	out := make([]time.Time, 0, len(src))
	for _, t := range src {
		if !t.IsZero() {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ID returns the local event identifier.
func (e Event) ID() string { return e.id }

// UID returns the ICS UID for round-trip, which may be empty.
func (e Event) UID() string { return e.uid }

// CalendarID returns the id of the calendar the event belongs to.
func (e Event) CalendarID() string { return e.calendarID }

// Summary returns the event title.
func (e Event) Summary() string { return e.summary }

// Description returns the optional description.
func (e Event) Description() string { return e.description }

// Location returns the optional location.
func (e Event) Location() string { return e.location }

// Start returns the event start time.
func (e Event) Start() time.Time { return e.start }

// End returns the event end time, which is the zero time when the event has no end.
func (e Event) End() time.Time { return e.end }

// HasEnd reports whether the event has an end time.
func (e Event) HasEnd() bool { return !e.end.IsZero() }

// Duration returns the span from start to end, or zero when the event has no end.
func (e Event) Duration() time.Duration {
	if !e.HasEnd() {
		return 0
	}
	return e.end.Sub(e.start)
}

// AllDay reports whether the event spans whole days rather than a timed range.
func (e Event) AllDay() bool { return e.allDay }

// Recurrence returns the raw RRULE text, which may be empty for a non-recurring event.
func (e Event) Recurrence() string { return e.recurrence }

// IsRecurring reports whether the event carries a recurrence rule or extra recurrence dates, so it
// defines a series rather than a single occurrence.
func (e Event) IsRecurring() bool { return e.recurrence != "" || len(e.rdates) > 0 }

// RDates returns a copy of the extra occurrence start times (RDATE) so callers cannot mutate the event.
func (e Event) RDates() []time.Time { return append([]time.Time(nil), e.rdates...) }

// ExDates returns a copy of the excluded occurrence start times (EXDATE).
func (e Event) ExDates() []time.Time { return append([]time.Time(nil), e.exdates...) }

// RecurrenceID returns the original start of the occurrence this event overrides, or the zero time when
// the event is not an override.
func (e Event) RecurrenceID() time.Time { return e.recurrenceID }

// IsOverride reports whether the event overrides a single occurrence of its series (RECURRENCE-ID set).
func (e Event) IsOverride() bool { return !e.recurrenceID.IsZero() }

// Extra returns the opaque original ICS VEVENT preserved for a lossless round-trip, or an empty string
// for an event that did not come from an import.
func (e Event) Extra() string { return e.extra }

// WithRecurrence returns a copy of the event with its recurrence rule replaced (trimmed); an empty rule
// clears it. The event stays immutable: the receiver is unchanged.
func (e Event) WithRecurrence(rule string) Event {
	e.recurrence = strings.TrimSpace(rule)
	return e
}

// WithExDates returns a copy of the event with its excluded occurrence starts replaced. Zero times are
// dropped and the slice is copied, so neither the receiver nor the caller's slice is shared.
func (e Event) WithExDates(dates []time.Time) Event {
	e.exdates = copyNonZeroTimes(dates)
	return e
}

// WithUID returns a copy of the event with its ICS UID replaced (trimmed), used when a this-and-future
// split moves an occurrence to a new series.
func (e Event) WithUID(uid string) Event {
	e.uid = strings.TrimSpace(uid)
	return e
}
