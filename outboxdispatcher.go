package main

import (
	"log"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// outboxDispatchTick is how often the dispatcher checks whether a held send has come due. It bounds how
// late past its scheduled time a send-later message can leave; one tick of this size is negligible
// against a time the user picked.
const outboxDispatchTick = 2 * time.Second

// outboxChangedEvent tells the front end the dispatcher sent a held item, so the outbox view and the
// unread surfaces refresh without polling.
const outboxChangedEvent = "outbox:changed"

// runOutboxDispatcher sends held outbox items as their holds elapse. It wakes on a short
// tick, asks for the earliest hold and replays only when one is actually due, so the plain offline
// queue is never touched here (that waits for a sync) and an idle app does no send work at all. It
// runs until the application context is cancelled. It first recovers the sends an earlier run left
// unfinished (see recoverOutbox).
func (a *App) runOutboxDispatcher() {
	a.recoverOutbox()
	ticker := time.NewTicker(outboxDispatchTick)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
		}
		next, ok, err := a.compose.NextHold(a.ctx)
		if err != nil || !ok || next.After(time.Now()) {
			continue
		}
		if sent, err := a.compose.ReplayDueHeld(a.ctx); sent > 0 || err != nil {
			runtime.EventsEmit(a.ctx, outboxChangedEvent)
		}
	}
}

// recoverOutbox returns to the queue every item an earlier run claimed for sending but never finished
// (it crashed or was killed mid-send), so the next replay sends it rather than leaving it stuck. Only
// claims stamped by another run are touched, so it is safe beside a replay this run has already
// started. A failure is logged because the dispatcher has no window of its own to report to; the items
// stay visible in the Outbox either way.
func (a *App) recoverOutbox() {
	released, err := a.compose.RecoverOutbox(a.ctx)
	if err != nil {
		log.Printf("outbox: %v", err)
		return
	}
	if released > 0 {
		log.Printf("outbox: re-queued %d send(s) interrupted by an earlier run", released)
		runtime.EventsEmit(a.ctx, outboxChangedEvent)
	}
}
