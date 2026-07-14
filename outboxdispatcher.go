package main

import (
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// outboxDispatchTick is how often the dispatcher checks whether a held send has come due. It bounds how
// late past its undo window a message can leave; a hold is seconds-scale, so a tick of this size reads
// as immediate.
const outboxDispatchTick = 2 * time.Second

// outboxChangedEvent tells the front end the dispatcher sent a held item, so the outbox view and the
// unread surfaces refresh without polling.
const outboxChangedEvent = "outbox:changed"

// runOutboxDispatcher sends held outbox items as their undo-send windows elapse. It wakes on a short
// tick, asks for the earliest hold and replays only when one is actually due, so the plain offline
// queue is never touched here (that waits for a sync) and an idle app does no send work at all. It
// runs until the application context is cancelled.
func (a *App) runOutboxDispatcher() {
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
