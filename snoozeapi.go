package main

// Snooze facade and its resurface scheduler. Snoozing hides a message from the visible listings until
// a chosen instant; this file exposes the Wails methods the front end calls and runs the goroutine
// that pops due snoozes, announces them and tells the front end to refresh. Kept apart from mailapi.go
// so the composition root stays within the module-size limit.

import (
	"fmt"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/sound"
)

// snoozeCheckTick is how often the scheduler checks whether a snooze has come due. It bounds how late
// past its chosen instant a message resurfaces; snooze times are minutes-scale, so a tick of this size
// reads as on time.
const snoozeCheckTick = 15 * time.Second

// snoozeChangedEvent tells the front end the set of snoozed messages changed, so the message list, the
// Snoozed view and the unread badges refresh without polling.
const snoozeChangedEvent = "snooze:changed"

// snoozeResurfacedEvent carries the messages that just came back, so the front end can raise an
// in-window toast for each. It is separate from snoozeChangedEvent because the two answer different
// questions: the change event says the hidden set moved and the badges are stale, the resurfaced event
// says here is what to show the user. A snooze can change the set without producing anything to show.
const snoozeResurfacedEvent = "snooze:resurfaced"

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
// all. Each resurfaced message is announced in the window as a toast and on the desktop as a
// notification (a snooze is an alarm the user set); the front end is told to refresh. A snooze
// missed while the app was closed pops on the first tick after the next launch. It runs until the
// application context is cancelled.
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
		resurfaced, cleared, err := a.snooze.PopDue(a.ctx)
		if err != nil {
			runtime.LogErrorf(a.ctx, "snooze: pop due failed: %v", err)
			continue
		}
		if cleared == 0 {
			continue
		}
		// Emitted on cleared rather than on the messages: a snooze that came due has changed what the
		// Snoozed view and the badges should show, whether or not it left a message to announce.
		runtime.EventsEmit(a.ctx, snoozeChangedEvent)
		a.announceResurfaced(resurfaced)
		a.recordOrphanedSnoozes(cleared - len(resurfaced))
	}
}

// announceResurfaced shows the messages that just came back: a toast for each in the window and one
// desktop notification for the batch. The toast is what makes an expiry observable at all, since a
// Windows balloon reaches the user only if the shell's notification pipeline lets it through, which the
// app cannot see and does not control.
func (a *App) announceResurfaced(messages []application.SnoozedMessage) {
	if len(messages) == 0 {
		return
	}
	runtime.EventsEmit(a.ctx, snoozeResurfacedEvent, resurfacedDTOs(messages))
	a.notifyResurfaced(messages)
}

// resurfacedDTOs maps the resurfaced messages onto the wire, each stamped with its owning account so a
// clicked toast can open it even when it belongs to an account other than the one on screen. Tag
// colours are left off: a toast shows no tag dots, so fetching them here would put a query on the path
// of an announcement that must not fail for a decoration.
func resurfacedDTOs(messages []application.SnoozedMessage) []MessageDTO {
	out := make([]MessageDTO, 0, len(messages))
	for _, m := range messages {
		dto := toMessageDTO(m.Summary, nil)
		dto.AccountID = m.AccountID
		out = append(out, dto)
	}
	return out
}

// recordOrphanedSnoozes notes snoozes that came due with no message left to show: the message was moved
// by a rule, deleted on the server or dropped with its folder while it was hidden. The row is gone
// either way, so this is the only surviving trace; without it the case is indistinguishable from
// the scheduler never running.
func (a *App) recordOrphanedSnoozes(count int) {
	if count <= 0 || a.mailErrors == nil {
		return
	}
	a.mailErrors.Record(fmt.Errorf(
		"snooze: %d due snooze(s) cleared with no cached message to resurface", count))
}

// notifyResurfaced raises one desktop notification for the messages that just came back, naming the
// first and counting the rest. force: a snooze is an alarm, so it shows even when the window is focused.
func (a *App) notifyResurfaced(messages []application.SnoozedMessage) {
	if a.tray == nil {
		return
	}
	title, body, ok := resurfacedNotification(messages)
	if !ok {
		return
	}
	a.tray.Notify(title, body, true, sound.Resurfaced)
}

// resurfacedNotification composes the notification text for a batch of returned messages: the first
// message's subject plus how many others came with it. It reports false when there is nothing to
// announce, so the caller never raises an empty notification.
func resurfacedNotification(messages []application.SnoozedMessage) (title, body string, ok bool) {
	if len(messages) == 0 {
		return "", "", false
	}
	body = messages[0].Summary.Subject()
	if body == "" {
		body = noSubjectLabel
	}
	title = "Snoozed message is back"
	if len(messages) > 1 {
		title = "Snoozed messages are back"
		body = fmt.Sprintf("%s and %d more", body, len(messages)-1)
	}
	return title, body, true
}

// noSubjectLabel stands in for the subject of a message that carries none, so a notification never
// shows an empty line where its subject should be.
const noSubjectLabel = "(no subject)"
