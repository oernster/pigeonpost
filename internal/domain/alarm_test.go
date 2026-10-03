package domain

import (
	"testing"
	"time"
)

func TestAlarmOffsetAndTrigger(t *testing.T) {
	a := NewAlarm(-15 * time.Minute)
	if a.Offset() != -15*time.Minute {
		t.Errorf("Offset() = %v, want -15m", a.Offset())
	}
	start := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	if got := a.TriggerAt(start); !got.Equal(start.Add(-15 * time.Minute)) {
		t.Errorf("TriggerAt = %v, want 08:45", got)
	}
}

// An alarm anchored to the end of an hour-long event fires that long before the end; it reports the same
// end-relative offset back for export.
func TestAlarmFromEnd(t *testing.T) {
	length := time.Hour
	a := NewAlarmFromEnd(-15*time.Minute, length)
	start := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	if got := a.TriggerAt(start); !got.Equal(start.Add(45 * time.Minute)) {
		t.Errorf("TriggerAt = %v, want 09:45", got)
	}
	if got := a.OffsetFromEnd(length); got != -15*time.Minute {
		t.Errorf("OffsetFromEnd = %v, want -15m", got)
	}
}
