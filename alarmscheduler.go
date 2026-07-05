package main

import (
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// reminderPollInterval is how often the scheduler checks for reminders that have come due.
const reminderPollInterval = 30 * time.Second

// reminderEventName is the Wails event the front end listens on to show a reminder.
const reminderEventName = "calendar:reminder"

// ReminderDTO is a reminder pushed to the front end when it fires.
type ReminderDTO struct {
	EventID string `json:"eventId"`
	Summary string `json:"summary"`
	Start   string `json:"start"`
}

// runReminderScheduler polls for reminders that have come due and pushes each to the front end. It starts
// from the current time so a backlog of past reminders is not fired on launch, advances its checkpoint
// each tick so a reminder fires once, and stops when the runtime context is cancelled at shutdown.
func (a *App) runReminderScheduler() {
	last := time.Now()
	ticker := time.NewTicker(reminderPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			due, err := a.calendar.DueReminders(a.ctx, last, now)
			last = now
			if err != nil {
				continue
			}
			for _, r := range due {
				runtime.EventsEmit(a.ctx, reminderEventName, ReminderDTO{
					EventID: r.EventID,
					Summary: r.Summary,
					Start:   r.OccurrenceStart.Format(time.RFC3339),
				})
			}
		}
	}
}
