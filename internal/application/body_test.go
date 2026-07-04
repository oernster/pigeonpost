package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func newBodyService() (*MessageBodyService, *fakeMailStore, *fakeAccountStore, *fakeMailSource) {
	store := newFakeMailStore()
	accounts := newFakeAccountStore()
	source := &fakeMailSource{}
	return NewMessageBodyService(store, accounts, source), store, accounts, source
}

// seedMessageLocation wires up a message m1 in folder f1 owned by account a1 so a fetch can resolve.
func seedMessageLocation(t *testing.T, store *fakeMailStore, accounts *fakeAccountStore) {
	t.Helper()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	store.messages["f1"] = []domain.MessageSummary{testMessage(t, "m1", "f1")}
}

func TestMessageBodyCacheHit(t *testing.T) {
	svc, store, _, source := newBodyService()
	cached, _ := domain.NewMessageBody("m1", "hello", "")
	store.bodies["m1"] = cached
	source.fetchBodyErr = errBoom // must not be reached on a cache hit

	body, err := svc.Body(context.Background(), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body.Plain() != "hello" {
		t.Errorf("Plain = %q, want the cached hello", body.Plain())
	}
}

func TestMessageBodyCacheLookupError(t *testing.T) {
	svc, store, _, _ := newBodyService()
	store.getBodyErr = errBoom
	if _, err := svc.Body(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Body error = %v, want wrapped boom", err)
	}
}

func TestMessageBodyFetchesAndCaches(t *testing.T) {
	svc, store, accounts, source := newBodyService()
	seedMessageLocation(t, store, accounts)
	source.bodyPlain = "full body"
	source.bodyHTML = "<p>full body</p>"

	body, err := svc.Body(context.Background(), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body.Plain() != "full body" || body.HTML() != "<p>full body</p>" {
		t.Errorf("fetched body wrong: %+v", body)
	}
	if _, ok := store.bodies["m1"]; !ok {
		t.Error("fetched body was not cached")
	}
}

func TestMessageBodyGetMessageError(t *testing.T) {
	svc, store, _, _ := newBodyService()
	store.getMessageErr = errBoom
	if _, err := svc.Body(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Body error = %v, want wrapped boom", err)
	}
}

func TestMessageBodyGetFolderError(t *testing.T) {
	svc, store, accounts, _ := newBodyService()
	seedMessageLocation(t, store, accounts)
	store.getFolderErr = errBoom
	if _, err := svc.Body(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Body error = %v, want wrapped boom", err)
	}
}

func TestMessageBodyGetAccountError(t *testing.T) {
	svc, store, accounts, _ := newBodyService()
	seedMessageLocation(t, store, accounts)
	accounts.getErr = errBoom
	if _, err := svc.Body(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Body error = %v, want wrapped boom", err)
	}
}

func TestMessageBodyFetchError(t *testing.T) {
	svc, store, accounts, source := newBodyService()
	seedMessageLocation(t, store, accounts)
	source.fetchBodyErr = errBoom
	if _, err := svc.Body(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Body error = %v, want wrapped boom", err)
	}
}

func TestMessageBodyBuildError(t *testing.T) {
	svc, store, accounts, _ := newBodyService()
	// Force GetMessage to succeed for an empty id so building the body fails validation.
	forced := testMessage(t, "m1", "f1")
	store.forcedMessage = &forced
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	accounts.accounts["a1"] = testAccount(t, "a1")

	if _, err := svc.Body(context.Background(), ""); !errors.Is(err, domain.ErrEmptyMessageID) {
		t.Errorf("Body error = %v, want ErrEmptyMessageID", err)
	}
}

func TestMessageBodySaveError(t *testing.T) {
	svc, store, accounts, _ := newBodyService()
	seedMessageLocation(t, store, accounts)
	store.saveBodyErr = errBoom
	if _, err := svc.Body(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Body error = %v, want wrapped boom", err)
	}
}

func TestMessageRawFetches(t *testing.T) {
	svc, store, accounts, source := newBodyService()
	seedMessageLocation(t, store, accounts)
	source.raw = []byte("From: a@example.com\r\nSubject: Hi\r\n\r\nBody")

	raw, err := svc.Raw(context.Background(), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != "From: a@example.com\r\nSubject: Hi\r\n\r\nBody" {
		t.Errorf("raw = %q, want the fetched bytes", raw)
	}
}

func TestMessageRawGetMessageError(t *testing.T) {
	svc, store, _, _ := newBodyService()
	store.getMessageErr = errBoom
	if _, err := svc.Raw(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Raw error = %v, want wrapped boom", err)
	}
}

func TestMessageRawGetFolderError(t *testing.T) {
	svc, store, accounts, _ := newBodyService()
	seedMessageLocation(t, store, accounts)
	store.getFolderErr = errBoom
	if _, err := svc.Raw(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Raw error = %v, want wrapped boom", err)
	}
}

func TestMessageRawGetAccountError(t *testing.T) {
	svc, store, accounts, _ := newBodyService()
	seedMessageLocation(t, store, accounts)
	accounts.getErr = errBoom
	if _, err := svc.Raw(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Raw error = %v, want wrapped boom", err)
	}
}

func TestMessageRawFetchError(t *testing.T) {
	svc, store, accounts, source := newBodyService()
	seedMessageLocation(t, store, accounts)
	source.fetchRawErr = errBoom
	if _, err := svc.Raw(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("Raw error = %v, want wrapped boom", err)
	}
}

func TestMessageRawMessage(t *testing.T) {
	svc, store, accounts, source := newBodyService()
	accounts.accounts["a1"] = testAccount(t, "a1")
	store.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	withSubject, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: "m1", FolderID: "f1", UID: "1", Size: 10, Flags: domain.NewFlags(0), Subject: "Report",
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	store.messages["f1"] = []domain.MessageSummary{withSubject}
	source.raw = []byte("raw bytes")

	got, err := svc.RawMessage(context.Background(), "m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got.Raw) != "raw bytes" || got.Subject != "Report" {
		t.Errorf("RawMessage = %+v, want raw bytes with subject Report", got)
	}
}

func TestMessageRawMessageError(t *testing.T) {
	svc, store, _, _ := newBodyService()
	store.getMessageErr = errBoom
	if _, err := svc.RawMessage(context.Background(), "m1"); !errors.Is(err, errBoom) {
		t.Errorf("RawMessage error = %v, want wrapped boom", err)
	}
}
