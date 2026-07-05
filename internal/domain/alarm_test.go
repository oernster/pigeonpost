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
