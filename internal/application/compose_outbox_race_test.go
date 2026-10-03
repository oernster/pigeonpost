package application

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// claimingOutbox is a hand-written, goroutine-safe in-memory outbox that keeps the send state the real
// store keeps. An item is queued until claimed; a claim, a release and a cancel each happen under one
// lock, as the store's single conditional statements do. Error fields inject store failures.
type claimingOutbox struct {
	mu         sync.Mutex
	items      []domain.OutboxItem
	sending    map[string]bool
	claimErr   error
	releaseErr error
	recoverErr error
	orphans    int
}

func newClaimingOutbox(items ...domain.OutboxItem) *claimingOutbox {
	return &claimingOutbox{items: items, sending: map[string]bool{}}
}

func (c *claimingOutbox) EnqueueOutbox(_ context.Context, item domain.OutboxItem) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, item)
	return nil
}

func (c *claimingOutbox) ListOutbox(context.Context) ([]domain.OutboxItem, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]domain.OutboxItem(nil), c.items...), nil
}

// remove drops id from the queue and reports whether it was there; the caller holds the lock.
func (c *claimingOutbox) remove(id string) bool {
	for i, item := range c.items {
		if item.ID() == id {
			c.items = append(c.items[:i], c.items[i+1:]...)
			delete(c.sending, id)
			return true
		}
	}
	return false
}

func (c *claimingOutbox) DeleteOutbox(_ context.Context, id string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.remove(id), nil
}

// update replaces the item with id by change(item); the caller holds the lock.
func (c *claimingOutbox) update(id string, change func(domain.OutboxItem) domain.OutboxItem) {
	for i, item := range c.items {
		if item.ID() == id {
			c.items[i] = change(item)
		}
	}
}

func (c *claimingOutbox) MarkOutboxFailed(_ context.Context, id, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.update(id, func(item domain.OutboxItem) domain.OutboxItem { return item.WithFailure(reason) })
	return nil
}

func (c *claimingOutbox) ClearOutboxHold(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.update(id, func(item domain.OutboxItem) domain.OutboxItem { return item.WithHoldUntil(time.Time{}) })
	return nil
}

func (c *claimingOutbox) NextOutboxHold(context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func (c *claimingOutbox) ClaimOutbox(_ context.Context, id string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.claimErr != nil {
		return false, c.claimErr
	}
	for _, item := range c.items {
		if item.ID() == id && !item.Failed() && !c.sending[id] {
			c.sending[id] = true
			return true, nil
		}
	}
	return false, nil
}

func (c *claimingOutbox) ReleaseOutbox(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.releaseErr != nil {
		return c.releaseErr
	}
	delete(c.sending, id)
	return nil
}

func (c *claimingOutbox) CancelQueuedOutbox(_ context.Context, id string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sending[id] {
		return false, nil
	}
	return c.remove(id), nil
}

func (c *claimingOutbox) RecoverOutboxClaims(context.Context) (int, error) {
	return c.orphans, c.recoverErr
}

// isSending reports whether id is currently claimed.
func (c *claimingOutbox) isSending(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sending[id]
}

// blockingTransport is a MailTransport whose first Send holds until release is closed, so a test can
// act while a send is on the wire. Every Send counts a delivery; only the first blocks, so a second
// send that should never happen shows up as a count rather than as a hung test.
type blockingTransport struct {
	delivered atomic.Int32
	entered   chan struct{}
	release   chan struct{}
	first     sync.Once
}

func newBlockingTransport() *blockingTransport {
	return &blockingTransport{entered: make(chan struct{}), release: make(chan struct{})}
}

func (b *blockingTransport) Send(context.Context, domain.Account, domain.OutgoingMessage) error {
	b.delivered.Add(1)
	blocked := false
	b.first.Do(func() { blocked = true })
	if blocked {
		close(b.entered)
		<-b.release
	}
	return nil
}

// raceService builds a ComposeService over the claiming outbox and the blocking transport.
func raceService(t *testing.T, outbox *claimingOutbox, transport *blockingTransport) *ComposeService {
	t.Helper()
	d := newComposeDeps().withAccount(t)
	return NewComposeService(d.accounts, d.store, transport, d.drafts, d.sent, outbox, d.recovery,
		fakeClock{now: time.Unix(0, 0).UTC()}, func() string { return "queued-id" })
}

// replayInBackground runs replay on its own goroutine, waits until its send is on the wire and returns
// a channel carrying its count once it finishes.
func replayInBackground(t *testing.T, transport *blockingTransport, replay func(context.Context) (int, error)) <-chan int {
	t.Helper()
	done := make(chan int, 1)
	go func() {
		n, err := replay(context.Background())
		if err != nil {
			t.Errorf("background replay: %v", err)
		}
		done <- n
	}()
	<-transport.entered
	return done
}

// Two account syncs back to back each replay the whole outbox. The second must skip the item the first
// is still sending; otherwise the recipient gets it twice (the audit measured delivered=2).
func TestOutboxConcurrentReplaysDeliverOnce(t *testing.T) {
	outbox := newClaimingOutbox(outboxItem(t, "q1", "a1", domain.OutboxSend))
	transport := newBlockingTransport()
	svc := raceService(t, outbox, transport)

	first := replayInBackground(t, transport, svc.ReplayOutbox)
	second, err := svc.ReplayOutbox(context.Background())
	if err != nil {
		t.Fatalf("second replay: %v", err)
	}
	close(transport.release)
	firstCount := <-first

	if got := transport.delivered.Load(); got != 1 {
		t.Fatalf("delivered = %d, want 1 (replay counts %d and %d)", got, firstCount, second)
	}
}

// A send-later item past its time is due for both the dispatcher and a sync's replay. Only one of them
// may send it (the audit measured delivered=2).
func TestOutboxReplayAndDueHeldDeliverOnce(t *testing.T) {
	epoch := time.Unix(0, 0).UTC()
	outbox := newClaimingOutbox(heldItem(t, "q-due", epoch))
	transport := newBlockingTransport()
	svc := raceService(t, outbox, transport)

	first := replayInBackground(t, transport, svc.ReplayDueHeld)
	if _, err := svc.ReplayOutbox(context.Background()); err != nil {
		t.Fatalf("sync replay: %v", err)
	}
	close(transport.release)
	<-first

	if got := transport.delivered.Load(); got != 1 {
		t.Fatalf("delivered = %d, want 1", got)
	}
}

// A cancel that lands while the message is on the wire cannot stop it, so it must say so: reporting
// the item cancelled tells the user "will not be sent" about mail that was (the audit measured
// cancelled=true with delivered=1).
func TestOutboxCancelDuringSendReportsAlreadySent(t *testing.T) {
	outbox := newClaimingOutbox(outboxItem(t, "q1", "a1", domain.OutboxSend))
	transport := newBlockingTransport()
	svc := raceService(t, outbox, transport)

	replay := replayInBackground(t, transport, svc.ReplayOutbox)
	cancelled, err := svc.CancelOutbox(context.Background(), "q1")
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	close(transport.release)
	<-replay

	if cancelled {
		t.Errorf("cancel during send reported cancelled=true; the message was not stopped")
	}
	if got := transport.delivered.Load(); got != 1 {
		t.Errorf("delivered = %d, want 1", got)
	}
}
