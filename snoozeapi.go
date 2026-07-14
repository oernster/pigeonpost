package main

// Snooze facade and its resurface scheduler. Snoozing hides a message from the visible listings until
// a chosen instant; this file exposes the Wails methods the front end calls and runs the goroutine
// that pops due snoozes, announces them and tells the front end to refresh. Kept apart from mailapi.go
// so the composition root stays within the module-size limit.

import (
	"fmt"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/domain"
)

// snoozeCheckTick is how often the scheduler checks whether a snooze has come due. It bounds how late
// past its chosen instant a message resurfaces; snooze times are minutes-scale, so a tick of this size
// reads as on time.
const snoozeCheckTick = 15 * time.Second

// snoozeChangedEvent tells the front end snoozed messages resurfaced, so the message list, the Snoozed
// view and the unread badges refresh without polling.
const snoozeChangedEvent = "snooze:changed"

// SnoozeMessage hides a message until the given Unix-millisecond instant, replacing any snooze it
// already carries. The instant must be in the future.
func (a *App) SnoozeMessage(messageID string, untilMs int64) error {
	return a.snooze.Snooze(a.ctx, messageID, time.UnixMilli(untilMs))
}

// UnsnoozeMessage brings a hidden message back at once.
func (a *App) UnsnoozeMessage(messageID string) error {
	return a.snooze.Unsnooze(a.ctx, messageID)
}

// ListSnoozedMessages returns every snoozed message, soonest due first, each stamped with when it
// resurfaces, for the Snoozed view.
func (a *App) ListSnoozedMessages() ([]MessageDTO, error) {
	snoozed, err := a.snooze.Snoozed(a.ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]domain.MessageSummary, len(snoozed))
	for i, s := range snoozed {
		summaries[i] = s.Summary
	}
	colours, coloursErr := a.tags.ColoursForMessages(a.ctx, messageIDs(summaries))
	if coloursErr != nil {
		// Tag colours are decorative; a failure to load them must not break the list.
		colours = nil
	}
	out := make([]MessageDTO, 0, len(snoozed))
	for _, s := range snoozed {
		dto := toMessageDTO(s.Summary, colours[s.Summary.ID()])
		dto.SnoozedUntilMs = s.Until.UnixMilli()
		// The Snoozed view spans accounts, so each row says whose it is: the account dot shows in the
		// list and a reply composes from the row's own account, exactly as in the unified mailbox.
		dto.AccountID = s.AccountID
		out = append(out, dto)
	}
	return out, nil
}

// SnoozedCount returns how many messages are currently snoozed, for the sidebar entry's badge.
func (a *App) SnoozedCount() (int, error) {
	snoozed, err := a.snooze.Snoozed(a.ctx)
	if err != nil {
		return 0, err
	}
	return len(snoozed), nil
}

// runSnoozeScheduler resurfaces snoozed messages as their instants pass. It wakes on a short tick, asks
// for the earliest snooze and pops only when one is actually due, so an idle app does no snooze work at
// all. Each resurfaced message is announced with a desktop notification (a snooze is an alarm the user
// set) and the front end is told to refresh. A snooze missed while the app was closed pops on the first
// tick after the next launch. It runs until the application context is cancelled.
func (a *App) runSnoozeScheduler() {
	ticker := time.NewTicker(snoozeCheckTick)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
		}
		next, ok, err := a.snooze.NextDue(a.ctx)
		if err != nil || !ok || next.After(time.Now()) {
			continue
		}
		resurfaced, err := a.snooze.PopDue(a.ctx)
		if err != nil {
			runtime.LogErrorf(a.ctx, "snooze: pop due failed: %v", err)
			continue
		}
		if len(resurfaced) == 0 {
			continue
		}
		runtime.EventsEmit(a.ctx, snoozeChangedEvent)
		a.notifyResurfaced(resurfaced)
	}
}

// notifyResurfaced raises one desktop notification for the messages that just came back, naming the
// first and counting the rest. force: a snooze is an alarm, so it shows even when the window is focused.
func (a *App) notifyResurfaced(messages []domain.MessageSummary) {
	if a.tray == nil || len(messages) == 0 {
		return
	}
	first := messages[0]
	body := first.Subject()
	if body == "" {
		body = "(no subject)"
	}
	title := "Snoozed message is back"
	if len(messages) > 1 {
		title = "Snoozed messages are back"
		body = fmt.Sprintf("%s and %d more", body, len(messages)-1)
	}
	a.tray.Notify(title, body, true)
}
