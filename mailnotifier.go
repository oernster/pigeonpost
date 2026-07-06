package main

import (
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/taskbar"
)

// mailPollInterval is how often the background poller checks every account's inbox for new mail to notify
// about. It is independent of the front end's own on-screen folder refresh.
const mailPollInterval = 2 * time.Minute

// mailNewEventName is the Wails event the front end listens on to refresh its counts and message list
// after the poller brings in new mail.
const mailNewEventName = "mail:new"

// calendarChangedEventName is the Wails event the front end listens on to reload the calendar after the
// poller auto-applies an incoming meeting reply or cancellation.
const calendarChangedEventName = "calendar:changed"

// runMailNotifier polls every account's inbox on an interval and raises a desktop notification for newly
// arrived unread mail, so the user is alerted even when the window is hidden to the tray and whatever
// folder is on screen. Each folder's first population is silent (see SyncInboxes). It stops when the
// runtime context is cancelled at shutdown.
func (a *App) runMailNotifier() {
	ticker := time.NewTicker(mailPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			fresh, err := a.sync.SyncInboxes(a.ctx)
			if err != nil || len(fresh) == 0 {
				continue
			}
			a.applyIncomingScheduling(fresh)
			runtime.EventsEmit(a.ctx, mailNewEventName)
			if a.tray != nil {
				title, body := taskbar.MailBalloonText(mailSummaries(fresh))
				a.tray.Notify(title, body)
			}
		}
	}
}

// applyIncomingScheduling folds any meeting replies and cancellations the newly arrived messages carry
// into the calendar, so an attendee's reply updates the organizer's meeting and a cancellation removes the
// withdrawn one without the user opening each message. It fetches each body first so the scheduling decode
// can read its calendar part, and asks the front end to reload the calendar when anything changed. A
// message that is not a meeting, or whose body cannot be fetched, contributes nothing.
func (a *App) applyIncomingScheduling(messages []domain.MessageSummary) {
	changed := false
	for _, m := range messages {
		if _, err := a.body.Body(a.ctx, m.ID()); err != nil {
			continue
		}
		applied, err := a.scheduling.ApplyIncoming(a.ctx, m.ID())
		if err == nil && applied {
			changed = true
		}
	}
	if changed {
		runtime.EventsEmit(a.ctx, calendarChangedEventName)
	}
}

// mailSummaries reduces the newly arrived messages to the subject and sender the notification shows,
// preferring the sender's display name and falling back to its address.
func mailSummaries(messages []domain.MessageSummary) []taskbar.MailSummary {
	out := make([]taskbar.MailSummary, 0, len(messages))
	for _, m := range messages {
		sender := m.From().Display()
		if sender == "" {
			sender = m.From().Address()
		}
		out = append(out, taskbar.MailSummary{Subject: m.Subject(), Sender: sender})
	}
	return out
}
