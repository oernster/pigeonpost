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
}

// Event is a single calendar entry. It is immutable once constructed. UID carries the ICS UID for a
// lossless round-trip and may be empty on a new event; Recurrence holds the raw RRULE text preserved
// across import and export (recurrence expansion is a later concern).
type Event struct {
	id          string
	uid         string
	calendarID  string
	summary     string
	description string
	location    string
	start       time.Time
	end         time.Time
	allDay      bool
	recurrence  string
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
		id:          id,
		uid:         strings.TrimSpace(in.UID),
		calendarID:  strings.TrimSpace(in.CalendarID),
		summary:     summary,
		description: strings.TrimSpace(in.Description),
		location:    strings.TrimSpace(in.Location),
		start:       in.Start,
		end:         in.End,
		allDay:      in.AllDay,
		recurrence:  strings.TrimSpace(in.Recurrence),
	}, nil
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

// AllDay reports whether the event spans whole days rather than a timed range.
func (e Event) AllDay() bool { return e.allDay }

// Recurrence returns the raw RRULE text, which may be empty for a non-recurring event.
func (e Event) Recurrence() string { return e.recurrence }
