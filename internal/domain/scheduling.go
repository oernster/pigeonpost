package domain

import "strings"

// Method is the scheduling intent of an iTIP message, matching the RFC 5546 VCALENDAR METHOD property.
// PigeonPost handles the four methods that make up a two-way invite flow: PUBLISH for a plain feed,
// REQUEST to invite, REPLY to respond and CANCEL to withdraw.
type Method string

// The scheduling methods PigeonPost recognises.
const (
	MethodPublish Method = "PUBLISH"
	MethodRequest Method = "REQUEST"
	MethodReply   Method = "REPLY"
	MethodCancel  Method = "CANCEL"
)

// ParseMethod normalises and validates a METHOD property value. Unlike a role or a status a method has
// no default: an empty or unrecognised value is rejected, since a scheduling message with no known
// method carries no actionable intent.
func ParseMethod(s string) (Method, error) {
	v := Method(strings.ToUpper(strings.TrimSpace(s)))
	switch v {
	case MethodPublish, MethodRequest, MethodReply, MethodCancel:
		return v, nil
	}
	return "", ErrInvalidMethod
}

// SchedulingMessage is a parsed iTIP payload: a method and the one or more events it applies to. A
// recurring invite carries the series master plus any per-occurrence overrides, so events holds more
// than one entry. It is immutable once created.
type SchedulingMessage struct {
	method Method
	events []Event
}

// NewSchedulingMessage builds a scheduling message, rejecting an unknown method and an empty event set.
// The events slice is copied so the message does not share backing storage with the caller.
func NewSchedulingMessage(method Method, events []Event) (SchedulingMessage, error) {
	if _, err := ParseMethod(string(method)); err != nil {
		return SchedulingMessage{}, err
	}
	if len(events) == 0 {
		return SchedulingMessage{}, ErrNoSchedulingEvents
	}
	return SchedulingMessage{method: method, events: append([]Event(nil), events...)}, nil
}

// Method returns the scheduling intent of the message.
func (m SchedulingMessage) Method() Method { return m.method }

// Events returns a copy of the events the message applies to, so callers cannot mutate the message.
func (m SchedulingMessage) Events() []Event { return append([]Event(nil), m.events...) }

// PrimaryEvent returns the first event, the series master for a recurring invite and the sole event
// otherwise. A message always has at least one event, so this is always valid.
func (m SchedulingMessage) PrimaryEvent() Event { return m.events[0] }

// CalendarPart is an iMIP scheduling object (RFC 6047) to carry on an outgoing message: an iTIP method
// and the raw text/calendar payload it applies to. The zero value carries no scheduling object. It is
// immutable once constructed.
type CalendarPart struct {
	method  Method
	content []byte
}

// NewCalendarPart builds a calendar part, rejecting an unknown method and empty content. The bytes are
// copied so the part does not share backing storage with the caller.
func NewCalendarPart(method Method, content []byte) (CalendarPart, error) {
	if _, err := ParseMethod(string(method)); err != nil {
		return CalendarPart{}, err
	}
	if len(content) == 0 {
		return CalendarPart{}, ErrEmptyCalendarPart
	}
	return CalendarPart{method: method, content: append([]byte(nil), content...)}, nil
}

// Method returns the iTIP method the part carries.
func (c CalendarPart) Method() Method { return c.method }

// Content returns a copy of the raw text/calendar payload so callers cannot mutate the part.
func (c CalendarPart) Content() []byte { return append([]byte(nil), c.content...) }

// IsZero reports whether this is the empty part, the value an outgoing message carries when it has no
// scheduling object.
func (c CalendarPart) IsZero() bool { return len(c.content) == 0 }
