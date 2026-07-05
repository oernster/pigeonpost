package ics

import (
	"strings"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

func mustAddr(t *testing.T, address string) domain.EmailAddress {
	t.Helper()
	a, err := domain.NewEmailAddress("", address)
	if err != nil {
		t.Fatalf("NewEmailAddress(%q): %v", address, err)
	}
	return a
}

func TestICSSchedulingRoundTrip(t *testing.T) {
	start := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	organizer, err := domain.NewOrganizer(mustAddr(t, "chair@example.com"), "The Chair")
	if err != nil {
		t.Fatalf("NewOrganizer: %v", err)
	}
	required, err := domain.NewAttendee(domain.AttendeeInput{
		Address: mustAddr(t, "req@example.com"), CommonName: "Required Guest",
		Role: domain.RoleRequired, Status: domain.PartStatNeedsAction, RSVP: true,
	})
	if err != nil {
		t.Fatalf("NewAttendee required: %v", err)
	}
	optional, err := domain.NewAttendee(domain.AttendeeInput{
		Address: mustAddr(t, "opt@example.com"),
		Role:    domain.RoleOptional, Status: domain.PartStatAccepted,
	})
	if err != nil {
		t.Fatalf("NewAttendee optional: %v", err)
	}
	ev, err := domain.NewEvent(domain.EventInput{
		ID: "m1", UID: "m1", Summary: "Sync", Start: start,
		Organizer: organizer, Attendees: []domain.Attendee{required, optional},
	})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	data, err := New().Encode([]domain.Event{ev}, nil)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// The wire form carries the mailto scheme and the RSVP request.
	wire := string(data)
	if !strings.Contains(wire, "mailto:chair@example.com") {
		t.Errorf("organizer mailto missing from wire form:\n%s", wire)
	}
	if !strings.Contains(wire, "RSVP=TRUE") {
		t.Errorf("RSVP request missing from wire form:\n%s", wire)
	}
	got, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d events, want 1", len(got))
	}
	e := got[0]
	if !e.HasOrganizer() || e.Organizer().Address().Address() != "chair@example.com" ||
		e.Organizer().CommonName() != "The Chair" {
		t.Errorf("organizer not preserved: %+v", e.Organizer())
	}
	attendees := e.Attendees()
	if len(attendees) != 2 {
		t.Fatalf("decoded %d attendees, want 2", len(attendees))
	}
	byAddr := map[string]domain.Attendee{}
	for _, a := range attendees {
		byAddr[a.Address().Address()] = a
	}
	req := byAddr["req@example.com"]
	if req.CommonName() != "Required Guest" || req.Role() != domain.RoleRequired ||
		req.Status() != domain.PartStatNeedsAction || !req.RSVP() {
		t.Errorf("required attendee not preserved: %+v", req)
	}
	opt := byAddr["opt@example.com"]
	if opt.Role() != domain.RoleOptional || opt.Status() != domain.PartStatAccepted || opt.RSVP() {
		t.Errorf("optional attendee not preserved: %+v", opt)
	}
}

func TestICSDecodeSchedulingIsLenient(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//EN",
		"BEGIN:VEVENT", "UID:m2", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T090000Z",
		"SUMMARY:Sync",
		// No mailto scheme, and an unknown ROLE and PARTSTAT that must fall back to the defaults.
		"ORGANIZER:chair@example.com",
		"ATTENDEE;ROLE=MODERATOR;PARTSTAT=MAYBE:mailto:guest@example.com",
		// A malformed address that must be skipped rather than fail the import.
		"ATTENDEE:mailto:not-an-address",
		"END:VEVENT", "END:VCALENDAR",
	)
	got, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d events, want 1", len(got))
	}
	e := got[0]
	if !e.HasOrganizer() || e.Organizer().Address().Address() != "chair@example.com" {
		t.Errorf("scheme-less organizer not parsed: %+v", e.Organizer())
	}
	attendees := e.Attendees()
	if len(attendees) != 1 {
		t.Fatalf("decoded %d attendees, want 1 (malformed skipped)", len(attendees))
	}
	if attendees[0].Role() != domain.RoleRequired || attendees[0].Status() != domain.PartStatNeedsAction {
		t.Errorf("unknown role/status did not fall back to defaults: %+v", attendees[0])
	}
}

func TestICSDecodeRejectsMalformedOrganizer(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//EN",
		"BEGIN:VEVENT", "UID:m3", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T090000Z",
		"SUMMARY:Sync", "ORGANIZER:mailto:not-an-address",
		"END:VEVENT", "END:VCALENDAR",
	)
	got, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d events, want 1", len(got))
	}
	if got[0].HasOrganizer() {
		t.Errorf("malformed organizer should yield no organizer: %+v", got[0].Organizer())
	}
}

func TestICSEncodePlainEventHasNoSchedulingProps(t *testing.T) {
	ev, err := domain.NewEvent(domain.EventInput{
		ID: "p1", UID: "p1", Summary: "Solo", Start: time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	data, err := New().Encode([]domain.Event{ev}, nil)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	wire := string(data)
	if strings.Contains(wire, "ORGANIZER") || strings.Contains(wire, "ATTENDEE") {
		t.Errorf("plain event emitted scheduling properties:\n%s", wire)
	}
}
