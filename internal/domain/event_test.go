package domain

import (
	"errors"
	"testing"
	"time"
)

func eventStart() time.Time { return time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC) }

func TestNewEventValidatesRequiredFields(t *testing.T) {
	start := eventStart()
	if _, err := NewEvent(EventInput{ID: " ", Summary: "Standup", Start: start}); !errors.Is(err, ErrEmptyEventID) {
		t.Errorf("blank id err = %v, want ErrEmptyEventID", err)
	}
	if _, err := NewEvent(EventInput{ID: "e1", Summary: "  ", Start: start}); !errors.Is(err, ErrEmptyEventSummary) {
		t.Errorf("blank summary err = %v, want ErrEmptyEventSummary", err)
	}
	if _, err := NewEvent(EventInput{ID: "e1", Summary: "Standup"}); !errors.Is(err, ErrEmptyEventStart) {
		t.Errorf("zero start err = %v, want ErrEmptyEventStart", err)
	}
}

func TestNewEventRejectsEndBeforeStart(t *testing.T) {
	start := eventStart()
	end := start.Add(-time.Hour)
	if _, err := NewEvent(EventInput{ID: "e1", Summary: "Standup", Start: start, End: end}); !errors.Is(err, ErrEventEndsBeforeStart) {
		t.Errorf("err = %v, want ErrEventEndsBeforeStart", err)
	}
}

func TestNewEventFullRoundTrip(t *testing.T) {
	start := eventStart()
	end := start.Add(time.Hour)
	e, err := NewEvent(EventInput{
		ID: "  e1 ", UID: "  uid-1 ", CalendarID: " cal1 ", Summary: "  Standup ",
		Description: " daily ", Location: " Room 1 ", Start: start, End: end, TimeZone: " Europe/London ",
		AllDay: false, Recurrence: " FREQ=DAILY ", Alarms: []Alarm{NewAlarm(-15 * time.Minute)},
		Extra: "BEGIN:VEVENT\r\nCATEGORIES:WORK\r\nEND:VEVENT\r\n",
	})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if e.ID() != "e1" || e.UID() != "uid-1" || e.CalendarID() != "cal1" || e.Summary() != "Standup" ||
		e.Description() != "daily" || e.Location() != "Room 1" || e.Recurrence() != "FREQ=DAILY" ||
		e.TimeZone() != "Europe/London" {
		t.Errorf("fields not trimmed/exposed: %+v", e)
	}
	if alarms := e.Alarms(); len(alarms) != 1 || alarms[0].Offset() != -15*time.Minute {
		t.Errorf("alarms not exposed: %v", e.Alarms())
	}
	// Extra is preserved verbatim (not trimmed): it is an opaque ICS blob, not a display field.
	if e.Extra() != "BEGIN:VEVENT\r\nCATEGORIES:WORK\r\nEND:VEVENT\r\n" {
		t.Errorf("Extra not preserved verbatim: %q", e.Extra())
	}
	if !e.Start().Equal(start) || !e.End().Equal(end) || !e.HasEnd() || e.AllDay() {
		t.Errorf("times/flags wrong: start=%v end=%v hasEnd=%v allDay=%v", e.Start(), e.End(), e.HasEnd(), e.AllDay())
	}
}

func TestNewEventAllowsNoEnd(t *testing.T) {
	e, err := NewEvent(EventInput{ID: "e1", Summary: "Reminder", Start: eventStart(), AllDay: true})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if e.HasEnd() {
		t.Errorf("expected no end")
	}
	if e.Duration() != 0 {
		t.Errorf("no-end duration = %v, want 0", e.Duration())
	}
	if !e.AllDay() {
		t.Errorf("expected all-day")
	}
}

func TestEventDurationSpansStartToEnd(t *testing.T) {
	start := eventStart()
	e, err := NewEvent(EventInput{ID: "e1", Summary: "Standup", Start: start, End: start.Add(90 * time.Minute)})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if e.Duration() != 90*time.Minute {
		t.Errorf("duration = %v, want 90m", e.Duration())
	}
}

func TestNewEventRecurrenceDatesAreCopiedAndZerosDropped(t *testing.T) {
	start := eventStart()
	rd := start.Add(48 * time.Hour)
	ed := start.Add(24 * time.Hour)
	in := EventInput{
		ID: "e1", Summary: "Standup", Start: start, Recurrence: "FREQ=DAILY",
		RDates:  []time.Time{rd, {}},
		ExDates: []time.Time{{}, ed},
	}
	e, err := NewEvent(in)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if got := e.RDates(); len(got) != 1 || !got[0].Equal(rd) {
		t.Errorf("RDates = %v, want [%v]", got, rd)
	}
	if got := e.ExDates(); len(got) != 1 || !got[0].Equal(ed) {
		t.Errorf("ExDates = %v, want [%v]", got, ed)
	}
	// Mutating the caller's input slice after construction must not affect the event.
	in.RDates[0] = time.Time{}
	if got := e.RDates(); len(got) != 1 || !got[0].Equal(rd) {
		t.Errorf("event shares backing storage with caller input: %v", got)
	}
	// Mutating a returned slice must not affect the event either.
	got := e.RDates()
	got[0] = time.Time{}
	if again := e.RDates(); len(again) != 1 || !again[0].Equal(rd) {
		t.Errorf("returned slice aliases event storage: %v", again)
	}
}

func TestNewEventAllZeroRecurrenceDatesYieldNil(t *testing.T) {
	e, err := NewEvent(EventInput{
		ID: "e1", Summary: "Standup", Start: eventStart(),
		RDates: []time.Time{{}, {}},
	})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if e.RDates() != nil {
		t.Errorf("all-zero RDates = %v, want nil", e.RDates())
	}
}

func TestEventIsRecurring(t *testing.T) {
	start := eventStart()
	base := EventInput{ID: "e1", Summary: "Standup", Start: start}
	plain, _ := NewEvent(base)
	if plain.IsRecurring() {
		t.Errorf("plain event reported recurring")
	}
	byRule := base
	byRule.Recurrence = "FREQ=WEEKLY"
	if r, _ := NewEvent(byRule); !r.IsRecurring() {
		t.Errorf("RRULE event not reported recurring")
	}
	byDate := base
	byDate.RDates = []time.Time{start.Add(48 * time.Hour)}
	if r, _ := NewEvent(byDate); !r.IsRecurring() {
		t.Errorf("RDATE-only event not reported recurring")
	}
}

func TestEventCopyMethods(t *testing.T) {
	start := eventStart()
	e, err := NewEvent(EventInput{
		ID: "e1", UID: "uid-1", Summary: "Standup", Start: start, Recurrence: "FREQ=DAILY;COUNT=5",
		ExDates: []time.Time{start.Add(24 * time.Hour)},
	})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	rekeyed := e.WithUID("uid-2")
	if rekeyed.UID() != "uid-2" || e.UID() != "uid-1" {
		t.Errorf("WithUID mutated the receiver: got %q, receiver %q", rekeyed.UID(), e.UID())
	}
	zoned := e.WithTimeZone("  Europe/London  ")
	if zoned.TimeZone() != "Europe/London" || e.TimeZone() != "" {
		t.Errorf("WithTimeZone wrong: got %q, receiver %q", zoned.TimeZone(), e.TimeZone())
	}
	alarmed := e.WithAlarms([]Alarm{NewAlarm(-10 * time.Minute)})
	if got := alarmed.Alarms(); len(got) != 1 || got[0].Offset() != -10*time.Minute {
		t.Errorf("WithAlarms wrong: %v", got)
	}
	if len(e.Alarms()) != 0 {
		t.Errorf("WithAlarms mutated the receiver: %v", e.Alarms())
	}
	// A returned slice must not alias the event's storage.
	got := alarmed.Alarms()
	got[0] = NewAlarm(0)
	if again := alarmed.Alarms(); again[0].Offset() != -10*time.Minute {
		t.Errorf("returned slice aliases event storage: %v", again)
	}
	reruled := e.WithRecurrence("  FREQ=WEEKLY  ")
	if reruled.Recurrence() != "FREQ=WEEKLY" || e.Recurrence() != "FREQ=DAILY;COUNT=5" {
		t.Errorf("WithRecurrence wrong: got %q, receiver %q", reruled.Recurrence(), e.Recurrence())
	}
	newEx := start.Add(48 * time.Hour)
	reexed := e.WithExDates([]time.Time{newEx, {}})
	if got := reexed.ExDates(); len(got) != 1 || !got[0].Equal(newEx) {
		t.Errorf("WithExDates wrong: %v", got)
	}
	if len(e.ExDates()) != 1 || !e.ExDates()[0].Equal(start.Add(24*time.Hour)) {
		t.Errorf("WithExDates mutated the receiver: %v", e.ExDates())
	}
}

func TestEventOverride(t *testing.T) {
	start := eventStart()
	plain, _ := NewEvent(EventInput{ID: "e1", Summary: "Standup", Start: start})
	if plain.IsOverride() || !plain.RecurrenceID().IsZero() {
		t.Errorf("plain event reported as override")
	}
	override, _ := NewEvent(EventInput{
		ID: "e2", UID: "uid-1", Summary: "Standup moved", Start: start.Add(time.Hour),
		RecurrenceID: start,
	})
	if !override.IsOverride() || !override.RecurrenceID().Equal(start) {
		t.Errorf("override not exposed: isOverride=%v id=%v", override.IsOverride(), override.RecurrenceID())
	}
}

func TestNewEventWithoutSchedulingIsPlain(t *testing.T) {
	e, err := NewEvent(EventInput{ID: "e1", Summary: "Standup", Start: eventStart()})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if e.HasOrganizer() || !e.Organizer().IsZero() {
		t.Errorf("plain event reported an organizer")
	}
	if e.Attendees() != nil {
		t.Errorf("plain event Attendees() = %v, want nil", e.Attendees())
	}
}

func TestNewEventCarriesOrganizerAndAttendees(t *testing.T) {
	organizer, err := NewOrganizer(mustAddress(t, "chair@example.com"), "Chair")
	if err != nil {
		t.Fatalf("NewOrganizer: %v", err)
	}
	guest, err := NewAttendee(AttendeeInput{Address: mustAddress(t, "guest@example.com"), Status: PartStatNeedsAction})
	if err != nil {
		t.Fatalf("NewAttendee: %v", err)
	}
	in := EventInput{
		ID: "e1", Summary: "Sync", Start: eventStart(),
		Organizer: organizer, Attendees: []Attendee{guest},
	}
	e, err := NewEvent(in)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if !e.HasOrganizer() || e.Organizer().Address().Address() != "chair@example.com" {
		t.Errorf("organizer not exposed: %+v", e.Organizer())
	}
	if got := e.Attendees(); len(got) != 1 || got[0].Address().Address() != "guest@example.com" {
		t.Errorf("attendees not exposed: %v", got)
	}
	// Mutating the caller's input slice after construction must not affect the event.
	in.Attendees[0] = guest.WithStatus(PartStatDeclined)
	if e.Attendees()[0].Status() != PartStatNeedsAction {
		t.Errorf("event shares attendee backing storage with caller input")
	}
	// A returned slice must not alias the event storage.
	got := e.Attendees()
	got[0] = guest.WithStatus(PartStatDeclined)
	if e.Attendees()[0].Status() != PartStatNeedsAction {
		t.Errorf("returned slice aliases event attendee storage")
	}
}

func TestEventWithOrganizerAndAttendees(t *testing.T) {
	e, err := NewEvent(EventInput{ID: "e1", Summary: "Sync", Start: eventStart()})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	organizer, _ := NewOrganizer(mustAddress(t, "chair@example.com"), "Chair")
	withOrg := e.WithOrganizer(organizer)
	if !withOrg.HasOrganizer() || e.HasOrganizer() {
		t.Errorf("WithOrganizer mutated the receiver: recv=%v", e.HasOrganizer())
	}
	guest, _ := NewAttendee(AttendeeInput{Address: mustAddress(t, "guest@example.com")})
	withAtt := e.WithAttendees([]Attendee{guest})
	if got := withAtt.Attendees(); len(got) != 1 {
		t.Errorf("WithAttendees = %v, want one attendee", got)
	}
	if e.Attendees() != nil {
		t.Errorf("WithAttendees mutated the receiver: %v", e.Attendees())
	}
}
