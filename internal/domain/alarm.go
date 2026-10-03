package domain

import "time"

// Alarm is a display reminder for an event. Offset is the signed duration from the event's start at which
// the reminder fires, negative for the usual "before the event" case, matching an RFC 5545 VALARM TRIGGER
// relative to the start. A trigger the source anchored to the end (TRIGGER;RELATED=END) is held as its
// start-relative equivalent, so every occurrence of the event (which shares the event's length) fires it at
// the right instant through the same TriggerAt.
type Alarm struct {
	offset time.Duration
}

// NewAlarm builds an alarm that fires offset from the event start (a negative offset is before it).
func NewAlarm(offset time.Duration) Alarm {
	return Alarm{offset: offset}
}

// NewAlarmFromEnd builds an alarm that fires fromEnd from the end of an event lasting eventLength (a
// negative fromEnd is before the end), expressed as its start-relative equivalent.
func NewAlarmFromEnd(fromEnd, eventLength time.Duration) Alarm {
	return Alarm{offset: eventLength + fromEnd}
}

// Offset returns the signed duration from the event start at which the alarm fires.
func (a Alarm) Offset() time.Duration { return a.offset }

// OffsetFromEnd returns the signed duration from the end of an event lasting eventLength at which the
// alarm fires, the inverse of NewAlarmFromEnd.
func (a Alarm) OffsetFromEnd(eventLength time.Duration) time.Duration { return a.offset - eventLength }

// TriggerAt returns the absolute time this alarm fires for an occurrence starting at start.
func (a Alarm) TriggerAt(start time.Time) time.Time { return start.Add(a.offset) }
