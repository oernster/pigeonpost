package application

import (
	"context"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// A server that says it is temporarily unavailable is treated as one that cannot be reached: the work
// waits in the Outbox for the next replay rather than failing. Measured on 2026-10-10, when a provider
// outage answered every sign-in with UNAVAILABLE for hours.

func TestComposeSendQueuesWhenTheServerIsUnavailable(t *testing.T) {
	d := newComposeDeps().withAccount(t)
	d.transport.sendErr = domain.ErrServerUnavailable
	if err := d.service().Send(context.Background(), "a1", draftTo(t, "f@example.com")); err != nil {
		t.Fatalf("a send to an unavailable server should queue, not fail: %v", err)
	}
	if len(d.outbox.items) != 1 || d.outbox.items[0].Kind() != domain.OutboxSend {
		t.Fatalf("expected 1 queued send, got %+v", d.outbox.items)
	}
}

func TestComposeSaveDraftQueuesWhenTheServerIsUnavailable(t *testing.T) {
	d := newComposeDeps().withAccount(t).withDrafts(t)
	d.drafts.saveErr = domain.ErrServerUnavailable
	if err := d.service().SaveDraft(context.Background(), "a1", draftTo(t, "f@example.com")); err != nil {
		t.Fatalf("a draft for an unavailable server should queue, not fail: %v", err)
	}
	if len(d.outbox.items) != 1 || d.outbox.items[0].Kind() != domain.OutboxDraft {
		t.Fatalf("expected 1 queued draft, got %+v", d.outbox.items)
	}
}

// A replayed item that meets the outage again is kept for the next replay, not marked failed.
func TestOutboxReplayKeepsAnItemWhenTheServerIsUnavailable(t *testing.T) {
	outbox := newClaimingOutbox(outboxItem(t, "q1", "a1", domain.OutboxSend))
	svc, d := claimService(t, outbox)
	d.transport.sendErr = domain.ErrServerUnavailable
	if n, err := svc.ReplayOutbox(context.Background()); err != nil || n != 0 {
		t.Fatalf("ReplayOutbox = %d, %v; want 0, nil", n, err)
	}
	items, _ := outbox.ListOutbox(context.Background())
	if len(items) != 1 || items[0].Failed() {
		t.Errorf("want the item kept and not marked failed; items=%+v", items)
	}
}

func TestRespondQueuesWhenTheServerIsUnavailable(t *testing.T) {
	event := schedMeeting(t, "m1", "chair@example.com", time.Time{}, me)
	f := newSchedFixture(t, schedMessage(t, domain.MethodRequest, event))
	f.transport.sendErr = domain.ErrServerUnavailable
	if err := f.svc.Respond(context.Background(), "m1", domain.PartStatAccepted); err != nil {
		t.Fatalf("a reply to an unavailable server should queue, not fail: %v", err)
	}
	if len(f.outbox.items) != 1 {
		t.Fatalf("queued %d outbox items, want 1", len(f.outbox.items))
	}
}
