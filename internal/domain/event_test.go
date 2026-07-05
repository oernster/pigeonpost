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
		Description: " daily ", Location: " Room 1 ", Start: start, End: end,
		AllDay: false, Recurrence: " FREQ=DAILY ", Extra: "BEGIN:VEVENT\r\nCATEGORIES:WORK\r\nEND:VEVENT\r\n",
	})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if e.ID() != "e1" || e.UID() != "uid-1" || e.CalendarID() != "cal1" || e.Summary() != "Standup" ||
		e.Description() != "daily" || e.Location() != "Room 1" || e.Recurrence() != "FREQ=DAILY" {
		t.Errorf("fields not trimmed/exposed: %+v", e)
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
	if !e.AllDay() {
		t.Errorf("expected all-day")
	}
}
