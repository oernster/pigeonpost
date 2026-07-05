package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

func draftTo(t *testing.T, address string) Draft {
	t.Helper()
	to, err := domain.NewEmailAddress("", address)
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	return Draft{To: []domain.EmailAddress{to}, Subject: "Hi", Body: "Hello"}
}

// draftsFolder builds a Drafts-kind folder for an account, used by the SaveDraft and replay tests.
func draftsFolder(t *testing.T, accountID, path string) domain.Folder {
	t.Helper()
	folder, err := domain.NewFolder(accountID+":drafts", accountID, path, domain.FolderDrafts, 0, 0)
	if err != nil {
		t.Fatalf("build drafts folder: %v", err)
	}
	return folder
}

// composeDeps holds the fakes a ComposeService is built from, so tests can inspect them afterwards.
type composeDeps struct {
	accounts  *fakeAccountStore
	store     *fakeMailStore
	transport *fakeMailTransport
	drafts    *fakeDraftSaver
	outbox    *fakeOutboxStore
}

func newComposeDeps() composeDeps {
	return composeDeps{
		accounts:  newFakeAccountStore(),
		store:     newFakeMailStore(),
		transport: &fakeMailTransport{},
		drafts:    &fakeDraftSaver{},
		outbox:    &fakeOutboxStore{},
	}
}

func (d composeDeps) service() *ComposeService {
	return NewComposeService(d.accounts, d.store, d.transport, d.drafts, d.outbox,
		fakeClock{now: time.Unix(0, 0).UTC()}, func() string { return "queued-id" })
}

// withAccount registers a1 and returns the deps for chaining.
func (d composeDeps) withAccount(t *testing.T) composeDeps {
	d.accounts.accounts["a1"] = testAccount(t, "a1")
	return d
}

// withDrafts gives a1 a Drafts folder.
func (d composeDeps) withDrafts(t *testing.T) composeDeps {
	d.store.folders["a1"] = []domain.Folder{draftsFolder(t, "a1", "Drafts")}
	return d
}

func outboxItem(t *testing.T, id, accountID string, kind domain.OutboxKind) domain.OutboxItem {
	t.Helper()
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From: testAccount(t, accountID).Address(),
		To:   []domain.EmailAddress{mustComposeAddr(t, "friend@example.com")},
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	item, err := domain.NewOutboxItem(id, accountID, kind, msg, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("build outbox item: %v", err)
	}
	return item
}

func mustComposeAddr(t *testing.T, address string) domain.EmailAddress {
	t.Helper()
	a, err := domain.NewEmailAddress("", address)
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	return a
}

func TestComposeSend(t *testing.T) {
	d := newComposeDeps().withAccount(t)
	if err := d.service().Send(context.Background(), "a1", draftTo(t, "friend@example.com")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.transport.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(d.transport.sent))
	}
	if d.transport.sent[0].From().Address() != "user@example.com" {
		t.Errorf("From = %q, want the account address", d.transport.sent[0].From().Address())
	}
}

func TestComposeSendErrors(t *testing.T) {
	t.Run("account not found", func(t *testing.T) {
		d := newComposeDeps()
		if err := d.service().Send(context.Background(), "missing", draftTo(t, "f@example.com")); !errors.Is(err, ErrAccountNotFound) {
			t.Errorf("error = %v, want ErrAccountNotFound", err)
		}
	})

	t.Run("invalid draft", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		if err := d.service().Send(context.Background(), "a1", Draft{Subject: "x"}); !errors.Is(err, domain.ErrNoRecipients) {
			t.Errorf("error = %v, want ErrNoRecipients", err)
		}
	})

	t.Run("transport failure", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.transport.sendErr = errBoom
		if err := d.service().Send(context.Background(), "a1", draftTo(t, "f@example.com")); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}

func TestComposeSendOfflineQueues(t *testing.T) {
	d := newComposeDeps().withAccount(t)
	d.transport.sendErr = domain.ErrOffline
	if err := d.service().Send(context.Background(), "a1", draftTo(t, "f@example.com")); err != nil {
		t.Fatalf("offline send should queue, not fail: %v", err)
	}
	if len(d.transport.sent) != 0 {
		t.Errorf("expected nothing sent while offline, got %d", len(d.transport.sent))
	}
	if len(d.outbox.items) != 1 || d.outbox.items[0].Kind() != domain.OutboxSend {
		t.Fatalf("expected 1 queued send, got %+v", d.outbox.items)
	}
}

func TestComposeSendOfflineEnqueueErrors(t *testing.T) {
	t.Run("bad item id", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.transport.sendErr = domain.ErrOffline
		svc := NewComposeService(d.accounts, d.store, d.transport, d.drafts, d.outbox,
			fakeClock{now: time.Unix(0, 0).UTC()}, func() string { return "" })
		if err := svc.Send(context.Background(), "a1", draftTo(t, "f@example.com")); !errors.Is(err, domain.ErrEmptyOutboxID) {
			t.Errorf("error = %v, want ErrEmptyOutboxID", err)
		}
	})

	t.Run("store failure", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.transport.sendErr = domain.ErrOffline
		d.outbox.enqueueErr = errBoom
		if err := d.service().Send(context.Background(), "a1", draftTo(t, "f@example.com")); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}

func TestComposeSaveDraft(t *testing.T) {
	d := newComposeDeps().withAccount(t).withDrafts(t)
	if err := d.service().SaveDraft(context.Background(), "a1", draftTo(t, "friend@example.com")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.drafts.saved) != 1 {
		t.Fatalf("expected 1 saved draft, got %d", len(d.drafts.saved))
	}
	if d.drafts.paths[0] != "Drafts" {
		t.Errorf("draft path = %q, want Drafts", d.drafts.paths[0])
	}
}

func TestComposeSaveDraftIncomplete(t *testing.T) {
	// A draft with no recipients and an empty body is valid: the user is still composing it.
	d := newComposeDeps().withAccount(t).withDrafts(t)
	if err := d.service().SaveDraft(context.Background(), "a1", Draft{Subject: "later"}); err != nil {
		t.Fatalf("expected incomplete draft to save, got %v", err)
	}
	if len(d.drafts.saved) != 1 {
		t.Fatalf("expected 1 saved draft, got %d", len(d.drafts.saved))
	}
}

func TestComposeSaveDraftOfflineQueues(t *testing.T) {
	d := newComposeDeps().withAccount(t).withDrafts(t)
	d.drafts.saveErr = domain.ErrOffline
	if err := d.service().SaveDraft(context.Background(), "a1", draftTo(t, "f@example.com")); err != nil {
		t.Fatalf("offline draft should queue, not fail: %v", err)
	}
	if len(d.outbox.items) != 1 || d.outbox.items[0].Kind() != domain.OutboxDraft {
		t.Fatalf("expected 1 queued draft, got %+v", d.outbox.items)
	}
}

func TestComposeSaveDraftErrors(t *testing.T) {
	t.Run("account not found", func(t *testing.T) {
		d := newComposeDeps()
		if err := d.service().SaveDraft(context.Background(), "missing", draftTo(t, "f@example.com")); !errors.Is(err, ErrAccountNotFound) {
			t.Errorf("error = %v, want ErrAccountNotFound", err)
		}
	})

	t.Run("no drafts folder", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.store.folders["a1"] = []domain.Folder{testFolder(t, "a1:inbox", "a1", "INBOX")}
		if err := d.service().SaveDraft(context.Background(), "a1", draftTo(t, "f@example.com")); !errors.Is(err, ErrNoDraftsFolder) {
			t.Errorf("error = %v, want ErrNoDraftsFolder", err)
		}
	})

	t.Run("list folders failure", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.store.listFoldersErr = errBoom
		if err := d.service().SaveDraft(context.Background(), "a1", draftTo(t, "f@example.com")); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("invalid draft", func(t *testing.T) {
		d := newComposeDeps().withAccount(t).withDrafts(t)
		bad := Draft{To: []domain.EmailAddress{{}}}
		if err := d.service().SaveDraft(context.Background(), "a1", bad); !errors.Is(err, domain.ErrNoRecipients) {
			t.Errorf("error = %v, want ErrNoRecipients", err)
		}
	})

	t.Run("append failure", func(t *testing.T) {
		d := newComposeDeps().withAccount(t).withDrafts(t)
		d.drafts.saveErr = errBoom
		if err := d.service().SaveDraft(context.Background(), "a1", draftTo(t, "f@example.com")); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}

func TestComposePendingOutbox(t *testing.T) {
	d := newComposeDeps().withAccount(t)
	d.outbox.items = []domain.OutboxItem{outboxItem(t, "q1", "a1", domain.OutboxSend)}
	n, err := d.service().PendingOutbox(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("pending = %d, want 1", n)
	}

	d.outbox.listErr = errBoom
	if _, err := d.service().PendingOutbox(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestComposeOutboxItems(t *testing.T) {
	d := newComposeDeps().withAccount(t)
	d.outbox.items = []domain.OutboxItem{outboxItem(t, "q1", "a1", domain.OutboxSend)}
	items, err := d.service().OutboxItems(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].ID() != "q1" {
		t.Errorf("items = %+v, want the queued q1", items)
	}

	d.outbox.listErr = errBoom
	if _, err := d.service().OutboxItems(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestComposeCancelOutbox(t *testing.T) {
	d := newComposeDeps().withAccount(t)
	if err := d.service().CancelOutbox(context.Background(), "q1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.outbox.deleted) != 1 || d.outbox.deleted[0] != "q1" {
		t.Errorf("deleted = %v, want [q1]", d.outbox.deleted)
	}

	d.outbox.deleteErr = errBoom
	if err := d.service().CancelOutbox(context.Background(), "q1"); !errors.Is(err, errBoom) {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestComposeReplayOutbox(t *testing.T) {
	t.Run("sends and drafts", func(t *testing.T) {
		d := newComposeDeps().withAccount(t).withDrafts(t)
		d.outbox.items = []domain.OutboxItem{
			outboxItem(t, "q1", "a1", domain.OutboxSend),
			outboxItem(t, "q2", "a1", domain.OutboxDraft),
		}
		n, err := d.service().ReplayOutbox(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 2 {
			t.Errorf("replayed = %d, want 2", n)
		}
		if len(d.transport.sent) != 1 || len(d.drafts.saved) != 1 {
			t.Errorf("expected 1 send and 1 draft, got %d/%d", len(d.transport.sent), len(d.drafts.saved))
		}
		if len(d.outbox.items) != 0 {
			t.Errorf("expected queue drained, got %d", len(d.outbox.items))
		}
	})

	t.Run("still offline stops and keeps items", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.transport.sendErr = domain.ErrOffline
		d.outbox.items = []domain.OutboxItem{outboxItem(t, "q1", "a1", domain.OutboxSend)}
		n, err := d.service().ReplayOutbox(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Errorf("replayed = %d, want 0 while offline", n)
		}
		if len(d.outbox.items) != 1 {
			t.Errorf("expected item kept while offline, got %d", len(d.outbox.items))
		}
	})

	t.Run("permanent failure marks item and keeps it", func(t *testing.T) {
		d := newComposeDeps() // no account registered: the item's account is gone
		d.outbox.items = []domain.OutboxItem{outboxItem(t, "q1", "a1", domain.OutboxSend)}
		n, err := d.service().ReplayOutbox(context.Background())
		if err == nil {
			t.Fatal("expected an error reporting the failed item")
		}
		if n != 0 {
			t.Errorf("replayed = %d, want 0", n)
		}
		if len(d.outbox.deleted) != 0 {
			t.Errorf("a permanently failed item must not be dropped, deleted=%v", d.outbox.deleted)
		}
		if _, ok := d.outbox.failed["q1"]; !ok {
			t.Errorf("expected the undeliverable item marked failed, failed=%v", d.outbox.failed)
		}
		if len(d.outbox.items) != 1 || !d.outbox.items[0].Failed() {
			t.Errorf("expected the item kept and flagged failed, items=%+v", d.outbox.items)
		}
	})

	t.Run("already failed item is skipped", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.outbox.items = []domain.OutboxItem{outboxItem(t, "q1", "a1", domain.OutboxSend).WithFailure("gone")}
		n, err := d.service().ReplayOutbox(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Errorf("replayed = %d, want 0 (a failed item is not retried)", n)
		}
		if len(d.transport.sent) != 0 {
			t.Errorf("a failed item must not be re-sent, sent=%d", len(d.transport.sent))
		}
		if len(d.outbox.items) != 1 {
			t.Errorf("expected the failed item kept, items=%+v", d.outbox.items)
		}
	})

	t.Run("mark-failed error is reported", func(t *testing.T) {
		d := newComposeDeps() // account gone, so the send fails permanently
		d.outbox.items = []domain.OutboxItem{outboxItem(t, "q1", "a1", domain.OutboxSend)}
		d.outbox.markErr = errBoom
		if _, err := d.service().ReplayOutbox(context.Background()); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("draft replay without drafts folder marks item failed", func(t *testing.T) {
		d := newComposeDeps().withAccount(t) // account exists but no Drafts folder
		d.outbox.items = []domain.OutboxItem{outboxItem(t, "q1", "a1", domain.OutboxDraft)}
		n, err := d.service().ReplayOutbox(context.Background())
		if !errors.Is(err, ErrNoDraftsFolder) {
			t.Errorf("error = %v, want ErrNoDraftsFolder", err)
		}
		if n != 0 {
			t.Errorf("replayed = %d, want 0", n)
		}
		if _, ok := d.outbox.failed["q1"]; !ok {
			t.Errorf("expected the draft marked failed, failed=%v", d.outbox.failed)
		}
	})

	t.Run("list failure", func(t *testing.T) {
		d := newComposeDeps()
		d.outbox.listErr = errBoom
		if _, err := d.service().ReplayOutbox(context.Background()); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("delete failure after success", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.outbox.items = []domain.OutboxItem{outboxItem(t, "q1", "a1", domain.OutboxSend)}
		d.outbox.deleteErr = errBoom
		if _, err := d.service().ReplayOutbox(context.Background()); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}
