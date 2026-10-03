package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// claimService builds a ComposeService over a claiming outbox and the plain transport fake.
func claimService(t *testing.T, outbox *claimingOutbox) (*ComposeService, composeDeps) {
	t.Helper()
	d := newComposeDeps().withAccount(t)
	return NewComposeService(d.accounts, d.store, d.transport, d.drafts, d.sent, outbox, d.recovery,
		fakeClock{now: time.Unix(0, 0).UTC()}, func() string { return "id" }), d
}

// A send that does not deliver must give its claim back. Otherwise the item would sit in sending until
// the next start and nothing (not even a cancel) could act on it.
func TestOutboxReplayReleasesClaimWhenNotDelivered(t *testing.T) {
	for name, sendErr := range map[string]error{"offline": domain.ErrOffline, "refused": errBoom} {
		t.Run(name, func(t *testing.T) {
			outbox := newClaimingOutbox(outboxItem(t, "q1", "a1", domain.OutboxSend))
			svc, d := claimService(t, outbox)
			d.transport.sendErr = sendErr
			if n, _ := svc.ReplayOutbox(context.Background()); n != 0 {
				t.Errorf("replayed = %d, want 0", n)
			}
			if outbox.isSending("q1") {
				t.Error("the claim was not released after an undelivered send")
			}
		})
	}
}

// A claim the store cannot record stops the replay with that error rather than sending without the
// guard; so does a claim it cannot give back, rather than leaving the outcome unreported.
func TestOutboxReplayStopsOnClaimFailures(t *testing.T) {
	epoch := time.Unix(0, 0).UTC()
	replays := map[string]func(*ComposeService) (int, error){
		"ReplayOutbox":  func(s *ComposeService) (int, error) { return s.ReplayOutbox(context.Background()) },
		"ReplayDueHeld": func(s *ComposeService) (int, error) { return s.ReplayDueHeld(context.Background()) },
	}
	for name, replay := range replays {
		t.Run(name+"/claim", func(t *testing.T) {
			outbox := newClaimingOutbox(heldItem(t, "q1", epoch))
			outbox.claimErr = errBoom
			svc, d := claimService(t, outbox)
			if _, err := replay(svc); !errors.Is(err, errBoom) {
				t.Errorf("err = %v, want errBoom", err)
			}
			if len(d.transport.sent) != 0 {
				t.Errorf("sent %d without a claim", len(d.transport.sent))
			}
		})
		t.Run(name+"/release", func(t *testing.T) {
			outbox := newClaimingOutbox(heldItem(t, "q1", epoch))
			outbox.releaseErr = errBoom
			svc, d := claimService(t, outbox)
			d.transport.sendErr = errors.New("rejected")
			if _, err := replay(svc); !errors.Is(err, errBoom) {
				t.Errorf("err = %v, want errBoom", err)
			}
		})
	}
}

// A claimed due item whose server is unreachable loses its hold and is released, so the next sync
// replays it as an ordinary queued item.
func TestOutboxDueHeldOfflineClearsHoldAndReleases(t *testing.T) {
	outbox := newClaimingOutbox(heldItem(t, "q-due", time.Unix(0, 0).UTC()))
	svc, d := claimService(t, outbox)
	d.transport.sendErr = domain.ErrOffline
	sent, err := svc.ReplayDueHeld(context.Background())
	if err != nil || sent != 0 {
		t.Fatalf("ReplayDueHeld = %d, %v; want 0, nil", sent, err)
	}
	items, _ := outbox.ListOutbox(context.Background())
	if len(items) != 1 || !items[0].HoldUntil().IsZero() || outbox.isSending("q-due") {
		t.Errorf("want the item kept, its hold cleared and its claim released; items=%+v", items)
	}
}

// Recovery reports how many interrupted sends it re-queued; a store failure reaches the caller.
func TestOutboxRecover(t *testing.T) {
	outbox := newClaimingOutbox()
	outbox.orphans = 2
	svc, _ := claimService(t, outbox)
	if n, err := svc.RecoverOutbox(context.Background()); err != nil || n != 2 {
		t.Errorf("RecoverOutbox = %d, %v; want 2, nil", n, err)
	}
	outbox.recoverErr = errBoom
	if _, err := svc.RecoverOutbox(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want errBoom", err)
	}
}
