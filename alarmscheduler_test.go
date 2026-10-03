package main

import (
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
)

// After a sleep the first tick's window covers the whole slept span; reminders for occurrences that ended
// meanwhile are skipped, while anything still running, still to come or merely due now still fires.
func TestLiveRemindersSkipsEventsOverAfterSleep(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	ends := map[string]time.Time{
		"over":    now.Add(-2 * time.Hour),
		"running": now.Add(time.Hour),
		"future":  now.Add(3 * time.Hour),
		"justDue": now.Add(-time.Hour),
	}
	endOf := func(r application.DueReminder) (time.Time, bool) {
		end, ok := ends[r.EventID]
		return end, ok
	}
	stale := now.Add(-3 * time.Hour)
	due := []application.DueReminder{
		{EventID: "over", TriggerAt: stale},
		{EventID: "running", TriggerAt: stale},
		{EventID: "future", TriggerAt: stale},
		{EventID: "unknown", TriggerAt: stale},
		// An alarm set to fire after its event ended, coming due on an ordinary tick, still fires.
		{EventID: "justDue", TriggerAt: now.Add(-time.Second)},
	}
	got := liveReminders(due, now, endOf)
	var ids []string
	for _, r := range got {
		ids = append(ids, r.EventID)
	}
	want := []string{"running", "future", "unknown", "justDue"}
	if len(ids) != len(want) {
		t.Fatalf("kept %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("kept %v, want %v", ids, want)
		}
	}
}

// The occurrence end is the occurrence start plus the event's length; an event without an end is over at
// its start (a day later when it is all-day).
func TestOccurrenceEnd(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	if got := occurrenceEnd(start, time.Hour, true, false); !got.Equal(start.Add(time.Hour)) {
		t.Errorf("timed end = %v", got)
	}
	if got := occurrenceEnd(start, 0, false, false); !got.Equal(start) {
		t.Errorf("open timed end = %v", got)
	}
	if got := occurrenceEnd(start, 0, false, true); !got.Equal(start.Add(allDaySpan)) {
		t.Errorf("open all-day end = %v", got)
	}
}
