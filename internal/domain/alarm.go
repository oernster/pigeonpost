package domain

import "time"

// Alarm is a display reminder for an event. Offset is the signed duration from the event's start at which
// the reminder fires, negative for the usual "before the event" case, matching an RFC 5545 VALARM TRIGGER
// relative to the start.
type Alarm struct {
	offset time.Duration
}

// NewAlarm builds an alarm that fires offset from the event start (a negative offset is before it).
func NewAlarm(offset time.Duration) Alarm {
	return Alarm{offset: offset}
}

// Offset returns the signed duration from the event start at which the alarm fires.
func (a Alarm) Offset() time.Duration { return a.offset }

// TriggerAt returns the absolute time this alarm fires for an occurrence starting at start.
func (a Alarm) TriggerAt(start time.Time) time.Time { return start.Add(a.offset) }
