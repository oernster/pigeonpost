package domain

import "time"

// EventInstance is one concrete occurrence of an event within a queried window. A non-recurring event
// yields a single instance equal to itself; a recurring event yields one instance per expanded
// occurrence. RecurrenceID holds the original start of the occurrence, which identifies it for editing
// and matches an override event's RECURRENCE-ID.
type EventInstance struct {
	event        Event
	start        time.Time
	end          time.Time
	recurrenceID time.Time
}

// NewEventInstance builds an instance of the given event. The start is this occurrence's start; the end
// is its end (the zero time when the source event has no end); recurrenceID identifies the occurrence
// within its series.
func NewEventInstance(event Event, start, end, recurrenceID time.Time) EventInstance {
	return EventInstance{event: event, start: start, end: end, recurrenceID: recurrenceID}
}

// Event returns the source event this instance was expanded from.
func (i EventInstance) Event() Event { return i.event }

// Start returns this occurrence's start time.
func (i EventInstance) Start() time.Time { return i.start }

// End returns this occurrence's end time, which is the zero time when the source event has no end.
func (i EventInstance) End() time.Time { return i.end }

// HasEnd reports whether this occurrence has an end time.
func (i EventInstance) HasEnd() bool { return !i.end.IsZero() }

// RecurrenceID returns the original start of this occurrence within its series.
func (i EventInstance) RecurrenceID() time.Time { return i.recurrenceID }
