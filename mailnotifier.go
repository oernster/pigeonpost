package main

import (
	"context"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/taskbar"
)

// mailPollInterval is how often the backstop poll checks every account's inbox. IMAP accounts get instant
// pushes from an IDLE watcher, so this poll is a safety net for a missed push and the only mechanism for
// POP3, which has no IDLE. It is independent of the front end's own on-screen folder refresh.
const mailPollInterval = 60 * time.Second

// mailNewEventName is the Wails event the front end listens on to refresh its counts and message list
// after the poller brings in new mail.
const mailNewEventName = "mail:new"

// calendarChangedEventName is the Wails event the front end listens on to reload the calendar after the
// poller auto-applies an incoming meeting reply or cancellation.
const calendarChangedEventName = "calendar:changed"

// runMailNotifier watches every account's inbox and raises a desktop notification for newly arrived mail,
// so the user is alerted even when the window is hidden to the tray and whatever folder is on screen. IMAP
// accounts are watched by an IDLE push for instant detection; a backstop poll covers POP3 and a missed
// push. It primes a baseline first so an existing inbox is not announced, and stops when the runtime
// context is cancelled at shutdown.
func (a *App) runMailNotifier() {
	runtime.LogDebugf(a.ctx, "mail-notifier: starting, poll backstop %s, tray=%t", mailPollInterval, a.tray != nil)
	// Prime the baseline: this first pass caches the current inbox so an existing mailbox is not announced
	// as new. Only mail arriving after it counts, yet a message into a previously empty inbox still does,
	// because detection is by cached-id rather than by the folder being empty.
	primed, err := a.sync.SyncInboxes(a.ctx)
	if err != nil {
		runtime.LogErrorf(a.ctx, "mail-notifier: baseline prime failed: %v", err)
	} else {
		runtime.LogDebugf(a.ctx, "mail-notifier: baseline primed, ignoring %d already-present message(s)", len(primed))
	}
	a.startMailWatchers()
	ticker := time.NewTicker(mailPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.checkMail("poll")
		}
	}
}

// startMailWatchers launches an IMAP IDLE watcher for every IMAP account at startup, each calling checkMail
// the instant the server reports new mail. Accounts added, reconfigured or removed after launch are kept in
// sync by AddAccount, UpdateAccount and RemoveAccount through startMailWatcher and stopMailWatcher, so no
// restart is needed. A POP3 account has no IDLE and relies on the backstop poll.
func (a *App) startMailWatchers() {
	if a.watcher == nil {
		return
	}
	accounts, err := a.accounts.List(a.ctx)
	if err != nil {
		runtime.LogErrorf(a.ctx, "mail-notifier: listing accounts for IDLE watchers failed: %v", err)
		return
	}
	for _, account := range accounts {
		a.startMailWatcher(account)
	}
}

// startMailWatcher starts an IMAP account's IDLE watcher, replacing any watcher already running for that
// account so a freshly added or reconfigured account gets instant push without a restart. It first stops an
// existing watcher for the id (so changed server settings take effect, and a switch to POP3 leaves no stale
// IMAP watcher), then starts a new one only for an IMAP account. A nil watcher or a non-IMAP account leaves
// the backstop poll as the only mechanism. Each watcher runs under a child of the app context, so shutdown
// stops them all and stopMailWatcher can stop one on its own.
func (a *App) startMailWatcher(account domain.Account) {
	if a.watcher == nil {
		return
	}
	a.watchersMu.Lock()
	defer a.watchersMu.Unlock()
	if cancel, ok := a.watchers[account.ID()]; ok {
		cancel()
		delete(a.watchers, account.ID())
	}
	if account.Protocol() != domain.ProtocolIMAP {
		return
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.watchers[account.ID()] = cancel
	acc := account
	runtime.LogDebugf(a.ctx, "mail-notifier: starting IDLE watcher for %q", acc.ID())
	go a.watcher.Watch(ctx, acc, func() { a.checkMail("idle") })
}

// stopMailWatcher stops the IDLE watcher for an account, if one is running, so a removed account leaves no
// stale server connection. It is safe to call for an account that has no watcher (a POP3 account, or one
// added before the watcher existed).
func (a *App) stopMailWatcher(accountID string) {
	a.watchersMu.Lock()
	defer a.watchersMu.Unlock()
	if cancel, ok := a.watchers[accountID]; ok {
		cancel()
		delete(a.watchers, accountID)
	}
}

// checkMail syncs every inbox and, for any newly arrived mail, applies its scheduling, refreshes the front
// end and raises a notification. It is serialised so a backstop poll and an IDLE push cannot run
// concurrently and double-notify; trigger names what invoked it, for the log.
func (a *App) checkMail(trigger string) {
	a.mailCheck.Lock()
	defer a.mailCheck.Unlock()
	fresh, err := a.sync.SyncInboxes(a.ctx)
	if err != nil {
		runtime.LogErrorf(a.ctx, "mail-notifier: %s check failed: %v", trigger, err)
		return
	}
	if len(fresh) == 0 {
		return
	}
	a.applyIncomingScheduling(fresh)
	runtime.EventsEmit(a.ctx, mailNewEventName)
	if a.tray != nil {
		title, body := taskbar.MailBalloonText(mailSummaries(fresh))
		// force: show the new-mail notification even when PigeonPost is focused, the way a mail client
		// alerts regardless. A reminder suppresses when focused because its in-app banner covers it, but
		// new mail has no such in-window cue.
		a.tray.Notify(title, body, true)
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
		if err != nil || !applied {
			continue
		}
		changed = true
		// The reply or cancellation needed no action from the user, so mark it read (kept, not deleted)
		// once applied, so it does not linger as unread. Best-effort: a mark-read failure must not undo
		// the apply that already happened.
		_ = a.actions.MarkRead(a.ctx, m.ID(), true)
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
