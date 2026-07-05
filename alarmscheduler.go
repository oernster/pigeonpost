package main

import (
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/application"
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

// runReminderScheduler pushes reminders to the front end as they come due. On launch it first catches up
// reminders for still-imminent events whose trigger lapsed while the app was closed (so a reminder for an
// upcoming event is not missed), without resurrecting reminders for events already past. It then polls
// every interval, advancing its checkpoint so each reminder fires once, and stops when the runtime
// context is cancelled at shutdown.
func (a *App) runReminderScheduler() {
	last := time.Now()
	if pending, err := a.calendar.PendingReminders(a.ctx, last); err == nil {
		a.emitReminders(pending)
	}
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
			a.emitReminders(due)
		}
	}
}

// emitReminders pushes each reminder to the front end as a Wails event.
func (a *App) emitReminders(reminders []application.DueReminder) {
	for _, r := range reminders {
		runtime.EventsEmit(a.ctx, reminderEventName, ReminderDTO{
			EventID: r.EventID,
			Summary: r.Summary,
			Start:   r.OccurrenceStart.Format(time.RFC3339),
		})
	}
}
