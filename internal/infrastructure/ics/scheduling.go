package ics

import (
	"bytes"
	"fmt"
	"io"

	goical "github.com/emersion/go-ical"

	"github.com/oernster/pigeonpost/internal/domain"
)

// DecodeScheduling parses an iTIP payload (a VCALENDAR carrying a METHOD) into a scheduling message. It
// reuses the VEVENT decoder, so every event keeps its organizer and attendees. An absent or unknown
// METHOD is rejected, as is a payload with no usable event: an iMIP part without an actionable method or
// event is not a scheduling message.
func (Codec) DecodeScheduling(data []byte) (domain.SchedulingMessage, error) {
	dec := goical.NewDecoder(bytes.NewReader(data))
	var events []domain.Event
	method := ""
	for {
		cal, err := dec.Decode()
		if err == io.EOF {
			break
		}
		if err != nil {
			return domain.SchedulingMessage{}, fmt.Errorf("ics: decode scheduling: %w", err)
		}
		if method == "" {
			method = text(cal.Props, goical.PropMethod)
		}
		for _, e := range cal.Events() {
			if event, ok := eventFromICS(e); ok {
				events = append(events, event)
			}
		}
	}
	parsed, err := domain.ParseMethod(method)
	if err != nil {
		return domain.SchedulingMessage{}, err
	}
	return domain.NewSchedulingMessage(parsed, events)
}

// EncodeRequest builds a METHOD:REQUEST calendar inviting the attendees carried on the events. The
// events already hold their organizer and attendee list from the editor.
func (Codec) EncodeRequest(events []domain.Event) ([]byte, error) {
	return encodeScheduling(domain.MethodRequest, events)
}

// EncodeCancel builds a METHOD:CANCEL calendar withdrawing the events.
func (Codec) EncodeCancel(events []domain.Event) ([]byte, error) {
	return encodeScheduling(domain.MethodCancel, events)
}

// EncodeReply builds a METHOD:REPLY calendar carrying the responder's decision. Per RFC 5546 a reply
// names the organizer and a single attendee, the responder, with the PARTSTAT that is their answer, so
// the event's own attendee list is replaced by that one attendee. A per-occurrence reply keeps the
// event's RECURRENCE-ID, which eventToComponent already writes for an override.
func (Codec) EncodeReply(event domain.Event, responder domain.EmailAddress, status domain.ParticipationStatus) ([]byte, error) {
	attendee, err := domain.NewAttendee(domain.AttendeeInput{Address: responder, Status: status})
	if err != nil {
		return nil, err
	}
	reply := event.WithAttendees([]domain.Attendee{attendee})
	return encodeScheduling(domain.MethodReply, []domain.Event{reply})
}

// encodeScheduling writes the events as a single VCALENDAR stamped with the scheduling method. It reuses
// the VEVENT encoder, so each event keeps its organizer and attendees, and prepends the VTIMEZONE
// definitions so the events' TZID references resolve within the payload.
func encodeScheduling(method domain.Method, events []domain.Event) ([]byte, error) {
	if len(events) == 0 {
		return nil, domain.ErrNoSchedulingEvents
	}
	cal := goical.NewCalendar()
	cal.Props.SetText(goical.PropVersion, "2.0")
	cal.Props.SetText(goical.PropProductID, productID)
	cal.Props.SetText(goical.PropMethod, string(method))
	cal.Children = append(cal.Children, timezoneComponents(events)...)
	for _, ev := range events {
		cal.Children = append(cal.Children, eventToComponent(ev))
	}
	var buf bytes.Buffer
	if err := goical.NewEncoder(&buf).Encode(cal); err != nil {
		return nil, fmt.Errorf("ics: encode scheduling: %w", err)
	}
	return buf.Bytes(), nil
}
