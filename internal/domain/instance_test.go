package domain

import (
	"testing"
	"time"
)

func TestEventInstanceExposesOccurrence(t *testing.T) {
	start := eventStart()
	e, err := NewEvent(EventInput{ID: "e1", Summary: "Standup", Start: start, End: start.Add(time.Hour), Recurrence: "FREQ=DAILY"})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	occStart := start.Add(24 * time.Hour)
	occEnd := occStart.Add(time.Hour)
	inst := NewEventInstance(e, occStart, occEnd, occStart)
	if inst.Event().ID() != "e1" {
		t.Errorf("Event() id = %q, want e1", inst.Event().ID())
	}
	if !inst.Start().Equal(occStart) || !inst.End().Equal(occEnd) {
		t.Errorf("times wrong: start=%v end=%v", inst.Start(), inst.End())
	}
	if !inst.HasEnd() {
		t.Errorf("expected instance to have an end")
	}
	if !inst.RecurrenceID().Equal(occStart) {
		t.Errorf("RecurrenceID = %v, want %v", inst.RecurrenceID(), occStart)
	}
}

func TestEventInstanceWithoutEnd(t *testing.T) {
	start := eventStart()
	e, _ := NewEvent(EventInput{ID: "e1", Summary: "Reminder", Start: start})
	inst := NewEventInstance(e, start, time.Time{}, time.Time{})
	if inst.HasEnd() {
		t.Errorf("expected no end")
	}
}
