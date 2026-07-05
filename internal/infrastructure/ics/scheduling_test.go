package ics

import (
	"strings"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// The codec must satisfy the application SchedulingCodec port as well as CalendarCodec.
var _ application.SchedulingCodec = Codec{}

func meeting(t *testing.T, id string, recurrenceID time.Time) domain.Event {
	t.Helper()
	organizer, err := domain.NewOrganizer(mustAddr(t, "chair@example.com"), "The Chair")
	if err != nil {
		t.Fatalf("NewOrganizer: %v", err)
	}
	guest, err := domain.NewAttendee(domain.AttendeeInput{
		Address: mustAddr(t, "guest@example.com"), Role: domain.RoleRequired,
		Status: domain.PartStatNeedsAction, RSVP: true,
	})
	if err != nil {
		t.Fatalf("NewAttendee: %v", err)
	}
	ev, err := domain.NewEvent(domain.EventInput{
		ID: id, UID: id, Summary: "Sync", Start: time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
		Organizer: organizer, Attendees: []domain.Attendee{guest}, RecurrenceID: recurrenceID,
	})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	return ev
}

func TestDecodeSchedulingRequest(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//EN", "METHOD:REQUEST",
		"BEGIN:VEVENT", "UID:m1", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T090000Z",
		"SUMMARY:Sync", "ORGANIZER:mailto:chair@example.com",
		"ATTENDEE;PARTSTAT=NEEDS-ACTION;RSVP=TRUE:mailto:guest@example.com",
		"END:VEVENT", "END:VCALENDAR",
	)
	msg, err := New().DecodeScheduling(data)
	if err != nil {
		t.Fatalf("DecodeScheduling: %v", err)
	}
	if msg.Method() != domain.MethodRequest {
		t.Errorf("Method() = %q, want REQUEST", msg.Method())
	}
	if len(msg.Events()) != 1 {
		t.Fatalf("decoded %d events, want 1", len(msg.Events()))
	}
	if !msg.PrimaryEvent().HasOrganizer() {
		t.Errorf("primary event lost its organizer")
	}
}

func TestDecodeSchedulingRecurringCarriesOverrides(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//EN", "METHOD:REQUEST",
		"BEGIN:VEVENT", "UID:m1", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T090000Z",
		"SUMMARY:Weekly", "RRULE:FREQ=WEEKLY;COUNT=4", "ORGANIZER:mailto:chair@example.com",
		"END:VEVENT",
		"BEGIN:VEVENT", "UID:m1", "DTSTAMP:20260704T090000Z", "DTSTART:20260711T100000Z",
		"RECURRENCE-ID:20260711T090000Z", "SUMMARY:Weekly moved", "ORGANIZER:mailto:chair@example.com",
		"END:VEVENT", "END:VCALENDAR",
	)
	msg, err := New().DecodeScheduling(data)
	if err != nil {
		t.Fatalf("DecodeScheduling: %v", err)
	}
	events := msg.Events()
	if len(events) != 2 {
		t.Fatalf("decoded %d events, want 2 (master + override)", len(events))
	}
	if !msg.PrimaryEvent().IsRecurring() {
		t.Errorf("master not recurring: %q", msg.PrimaryEvent().Recurrence())
	}
	if !events[1].IsOverride() {
		t.Errorf("second event is not an override")
	}
}

func TestDecodeSchedulingRejectsBadMethodAndEmpty(t *testing.T) {
	noMethod := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//EN",
		"BEGIN:VEVENT", "UID:m1", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T090000Z",
		"SUMMARY:Sync", "END:VEVENT", "END:VCALENDAR",
	)
	if _, err := New().DecodeScheduling(noMethod); err == nil {
		t.Errorf("payload with no METHOD should be rejected")
	}
	noEvent := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//EN", "METHOD:REQUEST", "END:VCALENDAR",
	)
	if _, err := New().DecodeScheduling(noEvent); err == nil {
		t.Errorf("payload with a method but no event should be rejected")
	}
}

func TestEncodeRequestRoundTrips(t *testing.T) {
	data, err := New().EncodeRequest([]domain.Event{meeting(t, "m1", time.Time{})})
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	if !strings.Contains(string(data), "METHOD:REQUEST") {
		t.Errorf("REQUEST method missing from wire form:\n%s", data)
	}
	msg, err := New().DecodeScheduling(data)
	if err != nil {
		t.Fatalf("DecodeScheduling: %v", err)
	}
	if msg.Method() != domain.MethodRequest || len(msg.PrimaryEvent().Attendees()) != 1 {
		t.Errorf("request did not round-trip: method=%q attendees=%d", msg.Method(), len(msg.PrimaryEvent().Attendees()))
	}
}

func TestEncodeCancel(t *testing.T) {
	data, err := New().EncodeCancel([]domain.Event{meeting(t, "m1", time.Time{})})
	if err != nil {
		t.Fatalf("EncodeCancel: %v", err)
	}
	if !strings.Contains(string(data), "METHOD:CANCEL") {
		t.Errorf("CANCEL method missing from wire form:\n%s", data)
	}
}

func TestEncodeReplyCarriesOnlyTheResponder(t *testing.T) {
	source := meeting(t, "m1", time.Time{})
	data, err := New().EncodeReply(source, mustAddr(t, "guest@example.com"), domain.PartStatAccepted)
	if err != nil {
		t.Fatalf("EncodeReply: %v", err)
	}
	if !strings.Contains(string(data), "METHOD:REPLY") {
		t.Errorf("REPLY method missing from wire form:\n%s", data)
	}
	msg, err := New().DecodeScheduling(data)
	if err != nil {
		t.Fatalf("DecodeScheduling: %v", err)
	}
	e := msg.PrimaryEvent()
	if !e.HasOrganizer() {
		t.Errorf("reply lost its organizer")
	}
	attendees := e.Attendees()
	if len(attendees) != 1 {
		t.Fatalf("reply carried %d attendees, want 1 (the responder only)", len(attendees))
	}
	if attendees[0].Address().Address() != "guest@example.com" || attendees[0].Status() != domain.PartStatAccepted {
		t.Errorf("reply attendee wrong: %+v", attendees[0])
	}
}

func TestEncodeReplyKeepsRecurrenceID(t *testing.T) {
	occurrence := time.Date(2026, 7, 11, 9, 0, 0, 0, time.UTC)
	source := meeting(t, "m1", occurrence)
	data, err := New().EncodeReply(source, mustAddr(t, "guest@example.com"), domain.PartStatDeclined)
	if err != nil {
		t.Fatalf("EncodeReply: %v", err)
	}
	msg, err := New().DecodeScheduling(data)
	if err != nil {
		t.Fatalf("DecodeScheduling: %v", err)
	}
	if !msg.PrimaryEvent().IsOverride() || !msg.PrimaryEvent().RecurrenceID().Equal(occurrence) {
		t.Errorf("per-occurrence reply lost its RECURRENCE-ID: %v", msg.PrimaryEvent().RecurrenceID())
	}
}

func TestEncodeRequestRejectsNoEvents(t *testing.T) {
	if _, err := New().EncodeRequest(nil); err == nil {
		t.Errorf("encoding no events should be rejected")
	}
}

func TestEncodeReplyRejectsZeroResponder(t *testing.T) {
	source := meeting(t, "m1", time.Time{})
	if _, err := New().EncodeReply(source, domain.EmailAddress{}, domain.PartStatAccepted); err == nil {
		t.Errorf("a reply with no responder address should be rejected")
	}
}
