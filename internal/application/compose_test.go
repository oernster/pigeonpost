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
	sent      *fakeSentSaver
	outbox    *fakeOutboxStore
	recovery  *fakeDraftRecoveryStore
}

func newComposeDeps() composeDeps {
	return composeDeps{
		accounts:  newFakeAccountStore(),
		store:     newFakeMailStore(),
		transport: &fakeMailTransport{},
		drafts:    &fakeDraftSaver{},
		sent:      &fakeSentSaver{},
		outbox:    &fakeOutboxStore{},
		recovery:  &fakeDraftRecoveryStore{},
	}
}

func (d composeDeps) service() *ComposeService {
	return NewComposeService(d.accounts, d.store, d.transport, d.drafts, d.sent, d.outbox, d.recovery,
		fakeClock{now: time.Unix(0, 0).UTC()}, func() string { return "queued-id" })
}

// sentFolder builds a Sent-kind folder for an account, used by the save-to-Sent tests.
func sentFolder(t *testing.T, accountID, path string) domain.Folder {
	t.Helper()
	folder, err := domain.NewFolder(accountID+":sent", accountID, path, domain.FolderSent, 0, 0)
	if err != nil {
		t.Fatalf("build sent folder: %v", err)
	}
	return folder
}

// withSent gives a1 a Sent folder.
func (d composeDeps) withSent(t *testing.T) composeDeps {
	d.store.folders["a1"] = append(d.store.folders["a1"], sentFolder(t, "a1", "Sent"))
	return d
}

// gmailAccount registers a1 as a Gmail account (outgoing smtp.gmail.com), which saves sent mail
// server-side. It returns the deps for chaining.
func (d composeDeps) gmailAccount(t *testing.T) composeDeps {
	t.Helper()
	addr, err := domain.NewEmailAddress("", "user@gmail.com")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	in, err := domain.NewServerConfig("imap.gmail.com", 993, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("incoming: %v", err)
	}
	out, err := domain.NewServerConfig("smtp.gmail.com", 587, domain.SecurityStartTLS)
	if err != nil {
		t.Fatalf("outgoing: %v", err)
	}
	account, err := domain.NewAccount("a1", "Gmail", addr, domain.ProtocolIMAP, in, out, domain.AuthPassword)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	d.accounts.accounts["a1"] = account
	return d
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

func TestComposeSendSavesToSent(t *testing.T) {
	d := newComposeDeps().withAccount(t).withSent(t)
	if err := d.service().Send(context.Background(), "a1", draftTo(t, "f@example.com")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.transport.sent) != 1 {
		t.Errorf("expected the message to be sent, got %d", len(d.transport.sent))
	}
	if len(d.sent.saved) != 1 || d.sent.paths[0] != "Sent" {
		t.Errorf("expected a copy saved to Sent, got %v paths %v", d.sent.saved, d.sent.paths)
	}
}

func TestComposeSendSkipsSentForGmail(t *testing.T) {
	// Gmail saves sent mail server-side, so the client must not append its own copy.
	d := newComposeDeps().gmailAccount(t).withSent(t)
	if err := d.service().Send(context.Background(), "a1", draftTo(t, "f@example.com")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.sent.saved) != 0 {
		t.Errorf("Gmail send must not append to Sent, got %v", d.sent.saved)
	}
}

func TestComposeSendNoSentFolderSkips(t *testing.T) {
	d := newComposeDeps().withAccount(t) // no Sent folder configured
	if err := d.service().Send(context.Background(), "a1", draftTo(t, "f@example.com")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.sent.saved) != 0 {
		t.Errorf("no Sent folder means no copy, got %v", d.sent.saved)
	}
}

func TestComposeSendSentListErrorSkips(t *testing.T) {
	d := newComposeDeps().withAccount(t).withSent(t)
	d.store.listFoldersErr = errBoom
	// A folder-list error must not fail the send: the message is already delivered.
	if err := d.service().Send(context.Background(), "a1", draftTo(t, "f@example.com")); err != nil {
		t.Fatalf("send must succeed despite a folder-list error: %v", err)
	}
	if len(d.sent.saved) != 0 {
		t.Errorf("no Sent copy when the folder list cannot be read, got %v", d.sent.saved)
	}
}

func TestComposeSendSentAppendErrorSwallowed(t *testing.T) {
	d := newComposeDeps().withAccount(t).withSent(t)
	d.sent.saveErr = errBoom
	// The append to Sent failed; the message was already delivered, so Send still succeeds.
	if err := d.service().Send(context.Background(), "a1", draftTo(t, "f@example.com")); err != nil {
		t.Fatalf("send must succeed despite a Sent-append error: %v", err)
	}
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

	t.Run("unknown sender", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		draft := draftTo(t, "f@example.com")
		draft.From = "stranger@nowhere.example"
		if err := d.service().Send(context.Background(), "a1", draft); !errors.Is(err, ErrUnknownSender) {
			t.Errorf("error = %v, want ErrUnknownSender", err)
		}
		if len(d.transport.sent) != 0 {
			t.Error("a message from an unowned address must not be sent")
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
		svc := NewComposeService(d.accounts, d.store, d.transport, d.drafts, d.sent, d.outbox, d.recovery,
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

	t.Run("unknown sender", func(t *testing.T) {
		d := newComposeDeps().withAccount(t).withDrafts(t)
		draft := draftTo(t, "f@example.com")
		draft.From = "stranger@nowhere.example"
		if err := d.service().SaveDraft(context.Background(), "a1", draft); !errors.Is(err, ErrUnknownSender) {
			t.Errorf("error = %v, want ErrUnknownSender", err)
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
	d.outbox.items = []domain.OutboxItem{outboxItem(t, "q1", "a1", domain.OutboxSend)}
	cancelled, err := d.service().CancelOutbox(context.Background(), "q1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cancelled {
		t.Error("cancelling a queued item must report it was stopped")
	}
	if len(d.outbox.deleted) != 1 || d.outbox.deleted[0] != "q1" {
		t.Errorf("deleted = %v, want [q1]", d.outbox.deleted)
	}

	// Cancelling again reports the item was already gone: an undo that lost the race must say so.
	cancelled, err = d.service().CancelOutbox(context.Background(), "q1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cancelled {
		t.Error("cancelling a missing item must report the message had already left")
	}

	d.outbox.deleteErr = errBoom
	if _, err := d.service().CancelOutbox(context.Background(), "q1"); !errors.Is(err, errBoom) {
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

func composeSnapshot() DraftSnapshot {
	return DraftSnapshot{
		AccountID: "a1",
		To:        "friend@example.com, half@examp",
		Cc:        "cc@example.com",
		Subject:   "Half written",
		BodyHTML:  "<p>still typing</p>",
	}
}

func TestComposeSaveDraftRecovery(t *testing.T) {
	t.Run("saves a stamped snapshot", func(t *testing.T) {
		d := newComposeDeps()
		if err := d.service().SaveDraftRecovery(context.Background(), composeSnapshot()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !d.recovery.present {
			t.Fatal("snapshot was not stored")
		}
		got := d.recovery.snapshot
		if got.AccountID() != "a1" || got.To() != "friend@example.com, half@examp" ||
			got.Subject() != "Half written" || got.BodyHTML() != "<p>still typing</p>" {
			t.Errorf("stored snapshot mismatch: %+v", got)
		}
		if !got.SavedAt().Equal(time.Unix(0, 0).UTC()) {
			t.Errorf("savedAt = %v, want the injected clock time", got.SavedAt())
		}
	})

	t.Run("rejects a snapshot without an account", func(t *testing.T) {
		d := newComposeDeps()
		snap := composeSnapshot()
		snap.AccountID = ""
		if err := d.service().SaveDraftRecovery(context.Background(), snap); !errors.Is(err, domain.ErrEmptyAccountID) {
			t.Errorf("error = %v, want ErrEmptyAccountID", err)
		}
	})

	t.Run("wraps a store failure", func(t *testing.T) {
		d := newComposeDeps()
		d.recovery.saveErr = errBoom
		if err := d.service().SaveDraftRecovery(context.Background(), composeSnapshot()); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}

func TestComposeDraftRecovery(t *testing.T) {
	t.Run("returns the stored snapshot", func(t *testing.T) {
		d := newComposeDeps()
		if err := d.service().SaveDraftRecovery(context.Background(), composeSnapshot()); err != nil {
			t.Fatalf("save: %v", err)
		}
		got, ok, err := d.service().DraftRecovery(context.Background())
		if err != nil || !ok {
			t.Fatalf("get: ok=%v err=%v", ok, err)
		}
		if got.Subject() != "Half written" {
			t.Errorf("subject = %q", got.Subject())
		}
	})

	t.Run("reports an empty slot", func(t *testing.T) {
		d := newComposeDeps()
		if _, ok, err := d.service().DraftRecovery(context.Background()); err != nil || ok {
			t.Errorf("ok=%v err=%v, want ok=false err=nil", ok, err)
		}
	})

	t.Run("wraps a store failure", func(t *testing.T) {
		d := newComposeDeps()
		d.recovery.getErr = errBoom
		if _, _, err := d.service().DraftRecovery(context.Background()); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}

func TestComposeClearDraftRecovery(t *testing.T) {
	t.Run("clears the slot", func(t *testing.T) {
		d := newComposeDeps()
		if err := d.service().ClearDraftRecovery(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !d.recovery.cleared {
			t.Error("clear was not called on the store")
		}
	})

	t.Run("wraps a store failure", func(t *testing.T) {
		d := newComposeDeps()
		d.recovery.clearErr = errBoom
		if err := d.service().ClearDraftRecovery(context.Background()); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}

// heldItem builds a queued send carrying an undo-send hold ending at holdUntil.
func heldItem(t *testing.T, id string, holdUntil time.Time) domain.OutboxItem {
	t.Helper()
	return outboxItem(t, id, "a1", domain.OutboxSend).WithHoldUntil(holdUntil)
}

func TestComposeHoldSend(t *testing.T) {
	epoch := time.Unix(0, 0).UTC()

	t.Run("queues the send behind the window and returns its id", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		id, err := d.service().HoldSend(context.Background(), "a1", draftTo(t, "f@example.com"), 10*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "queued-id" {
			t.Errorf("id = %q, want queued-id", id)
		}
		if len(d.transport.sent) != 0 {
			t.Error("a held send must not reach the transport yet")
		}
		if len(d.outbox.items) != 1 {
			t.Fatalf("queued items = %d, want 1", len(d.outbox.items))
		}
		item := d.outbox.items[0]
		if item.Kind() != domain.OutboxSend || !item.HoldUntil().Equal(epoch.Add(10*time.Second)) {
			t.Errorf("queued item wrong: kind=%v hold=%v", item.Kind(), item.HoldUntil())
		}
	})

	t.Run("a zero window sends immediately", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		id, err := d.service().HoldSend(context.Background(), "a1", draftTo(t, "f@example.com"), 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "" {
			t.Errorf("id = %q, want empty for an immediate send", id)
		}
		if len(d.transport.sent) != 1 || len(d.outbox.items) != 0 {
			t.Errorf("expected a direct send, got sent=%d queued=%d", len(d.transport.sent), len(d.outbox.items))
		}
	})

	t.Run("wraps an unknown account", func(t *testing.T) {
		d := newComposeDeps()
		if _, err := d.service().HoldSend(context.Background(), "missing", draftTo(t, "f@example.com"), 10*time.Second); err == nil {
			t.Error("expected an error for an unknown account")
		}
	})

	t.Run("wraps an enqueue failure", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.outbox.enqueueErr = errBoom
		if _, err := d.service().HoldSend(context.Background(), "a1", draftTo(t, "f@example.com"), 10*time.Second); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("wraps an id generator yielding no id", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		svc := NewComposeService(d.accounts, d.store, d.transport, d.drafts, d.sent, d.outbox, d.recovery,
			fakeClock{now: epoch}, func() string { return "" })
		if _, err := svc.HoldSend(context.Background(), "a1", draftTo(t, "f@example.com"), 10*time.Second); !errors.Is(err, domain.ErrEmptyOutboxID) {
			t.Errorf("error = %v, want ErrEmptyOutboxID", err)
		}
	})
}

func TestComposeReplayDueHeld(t *testing.T) {
	epoch := time.Unix(0, 0).UTC()

	t.Run("sends only the due held items and keeps the Sent copy", func(t *testing.T) {
		d := newComposeDeps().withAccount(t).withSent(t)
		d.outbox.items = []domain.OutboxItem{
			heldItem(t, "q-due", epoch),
			heldItem(t, "q-future", epoch.Add(time.Minute)),
			outboxItem(t, "q-plain", "a1", domain.OutboxSend),
			heldItem(t, "q-failed", epoch).WithFailure("rejected"),
		}
		sent, err := d.service().ReplayDueHeld(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sent != 1 || len(d.transport.sent) != 1 {
			t.Errorf("sent = %d (transport %d), want exactly the due item", sent, len(d.transport.sent))
		}
		if len(d.sent.saved) != 1 {
			t.Errorf("a delivered held send must keep its Sent copy, saved = %d", len(d.sent.saved))
		}
		if len(d.outbox.deleted) != 1 || d.outbox.deleted[0] != "q-due" {
			t.Errorf("deleted = %v, want [q-due]", d.outbox.deleted)
		}
	})

	t.Run("an offline attempt clears the hold instead of retrying", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.transport.sendErr = domain.ErrOffline
		d.outbox.items = []domain.OutboxItem{heldItem(t, "q-due", epoch)}
		sent, err := d.service().ReplayDueHeld(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sent != 0 {
			t.Errorf("sent = %d, want 0 while offline", sent)
		}
		if len(d.outbox.clearedHolds) != 1 || d.outbox.clearedHolds[0] != "q-due" {
			t.Errorf("clearedHolds = %v, want [q-due]", d.outbox.clearedHolds)
		}
		if len(d.outbox.deleted) != 0 {
			t.Error("an undelivered item must stay queued")
		}
	})

	t.Run("wraps a clear-hold failure", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.transport.sendErr = domain.ErrOffline
		d.outbox.clearHoldErr = errBoom
		d.outbox.items = []domain.OutboxItem{heldItem(t, "q-due", epoch)}
		if _, err := d.service().ReplayDueHeld(context.Background()); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("stamps a permanent failure on the item", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.transport.sendErr = errBoom
		d.outbox.items = []domain.OutboxItem{heldItem(t, "q-due", epoch)}
		_, err := d.service().ReplayDueHeld(context.Background())
		if !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want the collected failure", err)
		}
		if d.outbox.failed["q-due"] == "" {
			t.Error("the failure must be stamped on the item")
		}
		if len(d.outbox.deleted) != 0 {
			t.Error("a failed item must stay in the outbox")
		}
	})

	t.Run("wraps a mark-failed failure", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.transport.sendErr = errBoom
		d.outbox.markErr = errBoom
		d.outbox.items = []domain.OutboxItem{heldItem(t, "q-due", epoch)}
		if _, err := d.service().ReplayDueHeld(context.Background()); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("wraps a delete failure", func(t *testing.T) {
		d := newComposeDeps().withAccount(t)
		d.outbox.deleteErr = errBoom
		d.outbox.items = []domain.OutboxItem{heldItem(t, "q-due", epoch)}
		if _, err := d.service().ReplayDueHeld(context.Background()); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})

	t.Run("wraps a list failure", func(t *testing.T) {
		d := newComposeDeps()
		d.outbox.listErr = errBoom
		if _, err := d.service().ReplayDueHeld(context.Background()); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}

func TestComposeNextHold(t *testing.T) {
	epoch := time.Unix(0, 0).UTC()

	t.Run("returns the earliest hold", func(t *testing.T) {
		d := newComposeDeps()
		d.outbox.items = []domain.OutboxItem{
			heldItem(t, "q-later", epoch.Add(time.Minute)),
			heldItem(t, "q-sooner", epoch.Add(10*time.Second)),
		}
		next, ok, err := d.service().NextHold(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || !next.Equal(epoch.Add(10*time.Second)) {
			t.Errorf("next = %v ok=%v, want the sooner hold", next, ok)
		}
	})

	t.Run("reports no hold", func(t *testing.T) {
		d := newComposeDeps()
		if _, ok, err := d.service().NextHold(context.Background()); err != nil || ok {
			t.Errorf("ok=%v err=%v, want no hold and no error", ok, err)
		}
	})

	t.Run("wraps a store failure", func(t *testing.T) {
		d := newComposeDeps()
		d.outbox.nextHoldErr = errBoom
		if _, _, err := d.service().NextHold(context.Background()); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}
