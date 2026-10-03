package main

import (
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/sound"
	"github.com/oernster/pigeonpost/internal/infrastructure/taskbar"
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

// staleReminderAge separates a reminder that is merely due from one a sleep left behind. An ordinary
// tick's window spans one poll interval; ticker jitter can stretch it, so a trigger older than two
// intervals can only have been carried over a gap such as the machine sleeping.
const staleReminderAge = 2 * reminderPollInterval

// allDaySpan is how long an all-day event without an end lasts: the one day it is on.
const allDaySpan = 24 * time.Hour

// runReminderScheduler pushes reminders to the front end as they come due, reading the wall clock through
// the production clock.
func (a *App) runReminderScheduler() {
	a.scheduleReminders(systemClock{})
}

// scheduleReminders is the scheduler loop. On launch it first catches up reminders for still-imminent
// events whose trigger lapsed while the app was closed (so a reminder for an upcoming event is not
// missed), without resurrecting reminders for events already past. It then polls every interval,
// advancing its checkpoint so each reminder fires once; it stops when the runtime context is cancelled at
// shutdown. After the machine wakes from sleep the first poll's window spans the whole slept time, so its
// reminders are filtered: one whose occurrence has already ended is skipped rather than fired late.
func (a *App) scheduleReminders(clock domain.Clock) {
	last := clock.Now()
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
			now := clock.Now()
			due, err := a.calendar.DueReminders(a.ctx, last, now)
			last = now
			if err != nil {
				continue
			}
			a.emitReminders(liveReminders(due, now, a.reminderOccurrenceEnd))
		}
	}
}

// liveReminders drops the stale reminders (older than staleReminderAge at now) whose occurrence ended at
// or before now. A fresh reminder always fires, even for an alarm set after its event's end; so does a
// stale one whose end endOf cannot tell: the filter only ever withholds a reminder it knows is moot.
func liveReminders(due []application.DueReminder, now time.Time, endOf func(application.DueReminder) (time.Time, bool)) []application.DueReminder {
	var live []application.DueReminder
	for _, r := range due {
		if r.TriggerAt.Before(now.Add(-staleReminderAge)) {
			if end, known := endOf(r); known && !end.After(now) {
				continue
			}
		}
		live = append(live, r)
	}
	return live
}

// reminderOccurrenceEnd looks up the reminder's event to find when its occurrence ends. A failed lookup
// reports the end as unknown, so the reminder still fires: a late reminder is better than a lost one.
func (a *App) reminderOccurrenceEnd(r application.DueReminder) (time.Time, bool) {
	ev, err := a.calendar.GetEvent(a.ctx, r.EventID)
	if err != nil {
		return time.Time{}, false
	}
	return occurrenceEnd(r.OccurrenceStart, ev.Duration(), ev.HasEnd(), ev.AllDay()), true
}

// occurrenceEnd is when an occurrence starting at start ends, for an event lasting length. An event
// without an end is over once it starts (at the end of its day when it is all-day).
func occurrenceEnd(start time.Time, length time.Duration, hasEnd, allDay bool) time.Time {
	switch {
	case hasEnd:
		return start.Add(length)
	case allDay:
		return start.Add(allDaySpan)
	default:
		return start
	}
}

// emitReminders pushes each reminder to the front end as a Wails event, then draws attention from
// outside the window for the batch: it flashes the taskbar button and raises a tray balloon. Both are a
// no-op when the window is already in the foreground, so an in-view reminder relies on its banner alone.
func (a *App) emitReminders(reminders []application.DueReminder) {
	for _, r := range reminders {
		runtime.EventsEmit(a.ctx, reminderEventName, ReminderDTO{
			EventID: r.EventID,
			Summary: r.Summary,
			Start:   r.OccurrenceStart.Format(time.RFC3339),
		})
	}
	if len(reminders) == 0 {
		return
	}
	if a.alerter != nil {
		a.alerter.Flash()
	}
	if a.tray != nil {
		summaries := make([]string, len(reminders))
		for i, r := range reminders {
			summaries[i] = r.Summary
		}
		title, body := taskbar.BalloonText(summaries)
		// A reminder suppresses when the window is focused: its in-app banner covers that case.
		a.tray.Notify(title, body, false, sound.Reminder)
	}
}
