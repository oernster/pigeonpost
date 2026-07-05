package recurrence

import (
	"strings"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// The expander must satisfy the application RecurrenceService port.
var _ application.RecurrenceService = (*Expander)(nil)

func at(day, hour int) time.Time { return time.Date(2026, time.July, day, hour, 0, 0, 0, time.UTC) }

func windowStart() time.Time { return time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC) }
func windowEnd() time.Time   { return time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC) }

func mustEvent(t *testing.T, in domain.EventInput) domain.Event {
	t.Helper()
	e, err := domain.NewEvent(in)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	return e
}

func startTimes(insts []domain.EventInstance) []time.Time {
	out := make([]time.Time, len(insts))
	for i, x := range insts {
		out[i] = x.Start()
	}
	return out
}

func wantStarts(t *testing.T, insts []domain.EventInstance, want ...time.Time) {
	t.Helper()
	got := startTimes(insts)
	if len(got) != len(want) {
		t.Fatalf("got %d starts %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Errorf("start[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestExpandDailyCount(t *testing.T) {
	e := mustEvent(t, domain.EventInput{
		ID: "e1", Summary: "Standup", Start: at(4, 9), End: at(4, 10), Recurrence: "FREQ=DAILY;COUNT=3",
	})
	insts, err := New().Expand(e, windowStart(), windowEnd())
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	wantStarts(t, insts, at(4, 9), at(5, 9), at(6, 9))
	// End is start plus the event's one-hour duration; RecurrenceID equals the occurrence start.
	if !insts[1].End().Equal(at(5, 10)) || !insts[1].HasEnd() {
		t.Errorf("end = %v, want %v", insts[1].End(), at(5, 10))
	}
	if !insts[1].RecurrenceID().Equal(at(5, 9)) {
		t.Errorf("recurrenceID = %v, want %v", insts[1].RecurrenceID(), at(5, 9))
	}
}

func TestExpandWindowFilters(t *testing.T) {
	e := mustEvent(t, domain.EventInput{
		ID: "e1", Summary: "Standup", Start: at(4, 9), End: at(4, 10), Recurrence: "FREQ=DAILY;COUNT=10",
	})
	insts, err := New().Expand(e, at(6, 0), at(7, 23))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	wantStarts(t, insts, at(6, 9), at(7, 9))
}

func TestExpandExcludesExDate(t *testing.T) {
	e := mustEvent(t, domain.EventInput{
		ID: "e1", Summary: "Standup", Start: at(4, 9), End: at(4, 10), Recurrence: "FREQ=DAILY;COUNT=3",
		ExDates: []time.Time{at(5, 9)},
	})
	insts, err := New().Expand(e, windowStart(), windowEnd())
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	wantStarts(t, insts, at(4, 9), at(6, 9))
}

func TestExpandAddsRDate(t *testing.T) {
	e := mustEvent(t, domain.EventInput{
		ID: "e1", Summary: "Standup", Start: at(4, 9), End: at(4, 10), Recurrence: "FREQ=DAILY;COUNT=2",
		RDates: []time.Time{at(10, 9)},
	})
	insts, err := New().Expand(e, windowStart(), windowEnd())
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	wantStarts(t, insts, at(4, 9), at(5, 9), at(10, 9))
}

func TestExpandRDateOnlyIncludesStart(t *testing.T) {
	e := mustEvent(t, domain.EventInput{
		ID: "e1", Summary: "Standup", Start: at(4, 9), End: at(4, 10),
		RDates: []time.Time{at(6, 9), at(8, 9)},
	})
	insts, err := New().Expand(e, windowStart(), windowEnd())
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	wantStarts(t, insts, at(4, 9), at(6, 9), at(8, 9))
}

func TestExpandDeduplicatesRDateEqualToRule(t *testing.T) {
	e := mustEvent(t, domain.EventInput{
		ID: "e1", Summary: "Standup", Start: at(4, 9), End: at(4, 10), Recurrence: "FREQ=DAILY;COUNT=2",
		RDates: []time.Time{at(5, 9)},
	})
	insts, err := New().Expand(e, windowStart(), windowEnd())
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	wantStarts(t, insts, at(4, 9), at(5, 9))
}

func TestExpandNoEndYieldsInstancesWithoutEnd(t *testing.T) {
	e := mustEvent(t, domain.EventInput{
		ID: "e1", Summary: "Reminder", Start: at(4, 9), Recurrence: "FREQ=DAILY;COUNT=2",
	})
	insts, err := New().Expand(e, windowStart(), windowEnd())
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	for i, x := range insts {
		if x.HasEnd() {
			t.Errorf("instance %d has an end, want none", i)
		}
	}
}

func TestExpandStripsRRulePrefix(t *testing.T) {
	e := mustEvent(t, domain.EventInput{
		ID: "e1", Summary: "Standup", Start: at(4, 9), End: at(4, 10), Recurrence: "RRULE:FREQ=DAILY;COUNT=1",
	})
	insts, err := New().Expand(e, windowStart(), windowEnd())
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	wantStarts(t, insts, at(4, 9))
}

func TestTruncateBeforeDropsCountAndSetsUntil(t *testing.T) {
	rule, err := New().TruncateBefore("FREQ=DAILY;COUNT=5", at(6, 9))
	if err != nil {
		t.Fatalf("TruncateBefore: %v", err)
	}
	if strings.Contains(rule, "COUNT") {
		t.Errorf("truncated rule still has a COUNT: %q", rule)
	}
	if !strings.Contains(rule, "UNTIL=20260706T085959Z") {
		t.Errorf("truncated rule = %q, want UNTIL one second before the occurrence", rule)
	}
	// Expanding the truncated rule yields only occurrences before the cut.
	e := mustEvent(t, domain.EventInput{
		ID: "e1", Summary: "Standup", Start: at(4, 9), End: at(4, 10), Recurrence: rule,
	})
	insts, err := New().Expand(e, windowStart(), windowEnd())
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	wantStarts(t, insts, at(4, 9), at(5, 9))
}

func TestTruncateBeforePreservesByDay(t *testing.T) {
	rule, err := New().TruncateBefore("FREQ=WEEKLY;BYDAY=MO,WE;INTERVAL=2", at(20, 9))
	if err != nil {
		t.Fatalf("TruncateBefore: %v", err)
	}
	if !strings.Contains(rule, "BYDAY=MO,WE") || !strings.Contains(rule, "INTERVAL=2") {
		t.Errorf("truncated rule dropped rule parts: %q", rule)
	}
}

func TestTruncateBeforeRejectsInvalidRule(t *testing.T) {
	if _, err := New().TruncateBefore("FREQ=DAILY;INTERVAL=oops", at(6, 9)); err == nil {
		t.Fatalf("expected an error for an invalid rule")
	}
}

func TestExpandRejectsInvalidRule(t *testing.T) {
	e := mustEvent(t, domain.EventInput{
		ID: "e1", Summary: "Standup", Start: at(4, 9), End: at(4, 10), Recurrence: "FREQ=DAILY;INTERVAL=oops",
	})
	_, err := New().Expand(e, windowStart(), windowEnd())
	if err == nil {
		t.Fatalf("expected an error for an invalid rule")
	}
	if !strings.Contains(err.Error(), "recurrence: parse rule") {
		t.Errorf("error = %v, want it to mention parse rule", err)
	}
}
