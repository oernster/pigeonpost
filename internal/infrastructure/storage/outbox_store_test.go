package storage

import (
	"context"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

func outboxTestItem(t *testing.T, id string, kind domain.OutboxKind) domain.OutboxItem {
	t.Helper()
	from, err := domain.NewEmailAddress("Me", "me@example.com")
	if err != nil {
		t.Fatalf("from: %v", err)
	}
	to, err := domain.NewEmailAddress("", "friend@example.com")
	if err != nil {
		t.Fatalf("to: %v", err)
	}
	cc, err := domain.NewEmailAddress("", "cc@example.com")
	if err != nil {
		t.Fatalf("cc: %v", err)
	}
	bcc, err := domain.NewEmailAddress("", "bcc@example.com")
	if err != nil {
		t.Fatalf("bcc: %v", err)
	}
	attachment, err := domain.NewAttachment("note.txt", "text/plain", []byte("queued bytes"))
	if err != nil {
		t.Fatalf("attachment: %v", err)
	}
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From: from, To: []domain.EmailAddress{to}, Cc: []domain.EmailAddress{cc},
		Bcc:         []domain.EmailAddress{bcc},
		Attachments: []domain.Attachment{attachment},
		Subject:     "Queued", Body: "hi", HTMLBody: "<p>hi</p>",
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	item, err := domain.NewOutboxItem(id, "a1", kind, msg, time.UnixMilli(1000).UTC())
	if err != nil {
		t.Fatalf("item: %v", err)
	}
	return item
}

func TestOutboxRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	// Enqueued oldest first by created time; q-old has the earlier timestamp.
	older := outboxTestItem(t, "q-old", domain.OutboxSend)
	newer := outboxTestItem(t, "q-new", domain.OutboxDraft)
	newer, err := domain.NewOutboxItem(newer.ID(), newer.AccountID(), newer.Kind(), newer.Message(), time.UnixMilli(2000).UTC())
	if err != nil {
		t.Fatalf("rebuild newer: %v", err)
	}
	if err := store.EnqueueOutbox(ctx, newer); err != nil {
		t.Fatalf("enqueue newer: %v", err)
	}
	if err := store.EnqueueOutbox(ctx, older); err != nil {
		t.Fatalf("enqueue older: %v", err)
	}

	items, err := store.ListOutbox(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID() != "q-old" || items[1].ID() != "q-new" {
		t.Errorf("expected oldest-first order, got %q then %q", items[0].ID(), items[1].ID())
	}

	// The send round-trips its recipients, subject and bodies intact.
	got := items[0]
	msg := got.Message()
	if msg.From().Address() != "me@example.com" || msg.From().Display() != "Me" {
		t.Errorf("sender lost: %q / %q", msg.From().Display(), msg.From().Address())
	}
	if len(msg.To()) != 1 || msg.To()[0].Address() != "friend@example.com" {
		t.Errorf("recipients lost: %+v", msg.To())
	}
	if len(msg.Cc()) != 1 || msg.Cc()[0].Address() != "cc@example.com" {
		t.Errorf("cc lost: %+v", msg.Cc())
	}
	if len(msg.Bcc()) != 1 || msg.Bcc()[0].Address() != "bcc@example.com" {
		t.Errorf("bcc lost: %+v", msg.Bcc())
	}
	if a := msg.Attachments(); len(a) != 1 || a[0].Filename() != "note.txt" || string(a[0].Content()) != "queued bytes" {
		t.Errorf("attachment lost: %+v", msg.Attachments())
	}
	if msg.Subject() != "Queued" || msg.HTMLBody() != "<p>hi</p>" {
		t.Errorf("body lost: %q / %q", msg.Subject(), msg.HTMLBody())
	}
	if got.Kind() != domain.OutboxSend || items[1].Kind() != domain.OutboxDraft {
		t.Errorf("kinds lost: %v / %v", got.Kind(), items[1].Kind())
	}

	removed, err := store.DeleteOutbox(ctx, "q-old")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !removed {
		t.Error("deleting a queued item must report it was removed")
	}
	remaining, err := store.ListOutbox(ctx)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID() != "q-new" {
		t.Errorf("expected only q-new to remain, got %+v", remaining)
	}
	// Deleting an item that is already gone reports false, the signal the undo race relies on.
	removed, err = store.DeleteOutbox(ctx, "q-old")
	if err != nil {
		t.Fatalf("delete missing: %v", err)
	}
	if removed {
		t.Error("deleting a missing item must report nothing was removed")
	}
}

func TestOutboxHoldRoundTripAndClear(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	holdUntil := time.Date(2026, time.July, 14, 12, 0, 10, 0, time.UTC)
	held := outboxTestItem(t, "q-held", domain.OutboxSend).WithHoldUntil(holdUntil)
	plain := outboxTestItem(t, "q-plain", domain.OutboxSend)
	if err := store.EnqueueOutbox(ctx, held); err != nil {
		t.Fatalf("enqueue held: %v", err)
	}
	if err := store.EnqueueOutbox(ctx, plain); err != nil {
		t.Fatalf("enqueue plain: %v", err)
	}

	items, err := store.ListOutbox(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	byID := map[string]domain.OutboxItem{}
	for _, item := range items {
		byID[item.ID()] = item
	}
	if !byID["q-held"].HoldUntil().Equal(holdUntil) {
		t.Errorf("hold not round-tripped: %v", byID["q-held"].HoldUntil())
	}
	if !byID["q-plain"].HoldUntil().IsZero() {
		t.Errorf("a plain item must carry no hold: %v", byID["q-plain"].HoldUntil())
	}

	next, ok, err := store.NextOutboxHold(ctx)
	if err != nil {
		t.Fatalf("next hold: %v", err)
	}
	if !ok || !next.Equal(holdUntil) {
		t.Errorf("next hold = %v ok=%v, want %v", next, ok, holdUntil)
	}

	if err := store.ClearOutboxHold(ctx, "q-held"); err != nil {
		t.Fatalf("clear hold: %v", err)
	}
	items, err = store.ListOutbox(ctx)
	if err != nil {
		t.Fatalf("list after clear: %v", err)
	}
	for _, item := range items {
		if !item.HoldUntil().IsZero() {
			t.Errorf("hold survived the clear on %q", item.ID())
		}
	}
	if _, ok, err := store.NextOutboxHold(ctx); err != nil || ok {
		t.Errorf("no hold must remain: ok=%v err=%v", ok, err)
	}
}

func TestNextOutboxHoldSkipsFailedItems(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	holdUntil := time.Date(2026, time.July, 14, 12, 0, 10, 0, time.UTC)
	held := outboxTestItem(t, "q-held", domain.OutboxSend).WithHoldUntil(holdUntil)
	if err := store.EnqueueOutbox(ctx, held); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := store.MarkOutboxFailed(ctx, "q-held", "rejected"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	if _, ok, err := store.NextOutboxHold(ctx); err != nil || ok {
		t.Errorf("a failed item must not drive the dispatcher: ok=%v err=%v", ok, err)
	}
}

func TestOutboxMarkFailed(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	item := outboxTestItem(t, "q1", domain.OutboxSend)
	if err := store.EnqueueOutbox(ctx, item); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// A freshly enqueued item is not failed.
	items, err := store.ListOutbox(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].Failed() {
		t.Fatalf("expected one un-failed item, got %+v", items)
	}

	// Marking it failed persists the reason, and the item stays in the queue.
	const reason = "550 mailbox unavailable"
	if err := store.MarkOutboxFailed(ctx, "q1", reason); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	items, err = store.ListOutbox(ctx)
	if err != nil {
		t.Fatalf("list after mark: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("a failed item must be kept, got %d", len(items))
	}
	if !items[0].Failed() || items[0].Failure() != reason {
		t.Errorf("failure not persisted, failed=%v reason=%q", items[0].Failed(), items[0].Failure())
	}
}
