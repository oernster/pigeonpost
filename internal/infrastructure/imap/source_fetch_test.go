package imap

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// unreadableBodyStructure is a multipart/mixed whose second child is a message/rfc822 part carrying NIL
// where its envelope belongs. The client's grammar requires an envelope there, so reading it fails and
// the whole FETCH is lost with it. This is the shape behind the reported sync failure on Gmail's All
// Mail; it is written out here so the failure is reproducible offline.
const unreadableBodyStructure = `(("TEXT" "PLAIN" ("CHARSET" "UTF-8") NIL NIL "7BIT" 10 1 NIL NIL NIL NIL)` +
	`("MESSAGE" "RFC822" NIL NIL NIL "7BIT" 100 NIL NIL NIL NIL) "MIXED" NIL NIL NIL NIL)`

// TestIsBodyStructureErrorRejectsOthers proves the detector does not claim an ordinary failure, which
// would turn a real error into a silent second attempt.
func TestIsBodyStructureErrorRejectsOthers(t *testing.T) {
	t.Parallel()
	if isBodyStructureError(nil) {
		t.Fatal("nil reported as a body structure failure")
	}
	if isBodyStructureError(domain.ErrOffline) {
		t.Fatal("an offline error reported as a body structure failure")
	}
}

// TestFetchMessagesFallsBackWhenBodyStructureUnreadable serves a structure the client cannot read and
// asserts the folder still yields its summaries, with the paperclip reported as absent rather than the
// whole fetch being lost. It also proves the detector matches an error the library itself produced,
// so a library upgrade that renamed the grammar tokens fails here rather than in the field.
func TestFetchMessagesFallsBackWhenBodyStructureUnreadable(t *testing.T) {
	t.Parallel()
	host, port := listenFake(t, script{bodyStructure: unreadableBodyStructure})
	source := fakeSource()

	// The structure alone is what the client cannot read, so the fetch that asks for it must fail.
	_, err := source.fetchSummaries(context.Background(), fakeAccount(t, host, port), fakeFolder(t), true)
	if err == nil {
		t.Fatal("expected the fetch asking for the body structure to fail")
	}
	if !isBodyStructureError(err) {
		t.Fatalf("detector missed the library's own error: %v", err)
	}
	// The same failure must reach the interface as a sentinel, so the reader is not shown the grammar
	// production that ran out.
	if !errors.Is(err, domain.ErrUnreadableResponse) {
		t.Fatalf("an unreadable reply was not marked for the interface: %v", err)
	}

	messages, err := source.FetchMessages(context.Background(), fakeAccount(t, host, port), fakeFolder(t))
	if err != nil {
		t.Fatalf("FetchMessages after fallback: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("summaries: want 1, got %d", len(messages))
	}
	if messages[0].UID() != "7" {
		t.Fatalf("uid: want 7, got %q", messages[0].UID())
	}
	if messages[0].HasAttachments() {
		t.Fatal("paperclip claimed on a fallback fetch that never read a structure")
	}
}

// TestFetchMessagesKeepsStructureWhenReadable proves the fallback is not taken on a structure the client
// can read, so the paperclip still reaches the list.
func TestFetchMessagesKeepsStructureWhenReadable(t *testing.T) {
	t.Parallel()
	readable := `(("TEXT" "PLAIN" ("CHARSET" "UTF-8") NIL NIL "7BIT" 10 1 NIL NIL NIL NIL)` +
		`("APPLICATION" "PDF" ("NAME" "report.pdf") NIL NIL "BASE64" 200 NIL ("ATTACHMENT" ("FILENAME" "report.pdf")) NIL NIL)` +
		` "MIXED" NIL NIL NIL NIL)`
	host, port := listenFake(t, script{bodyStructure: readable})
	source := fakeSource()

	messages, err := source.FetchMessages(context.Background(), fakeAccount(t, host, port), fakeFolder(t))
	if err != nil {
		t.Fatalf("FetchMessages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("summaries: want 1, got %d", len(messages))
	}
	if !messages[0].HasAttachments() {
		t.Fatal("paperclip missing on a message whose structure carries an attachment")
	}
}

// TestMarkUnreadableLeavesOtherFailuresAlone proves the marking is narrow: an ordinary failure keeps its
// own detail rather than being replaced by the general sentence.
func TestMarkUnreadableLeavesOtherFailuresAlone(t *testing.T) {
	t.Parallel()
	if markUnreadable(nil) != nil {
		t.Fatal("nil was marked")
	}
	ordinary := errors.New("imap: NO mailbox does not exist")
	if errors.Is(markUnreadable(ordinary), domain.ErrUnreadableResponse) {
		t.Fatal("an ordinary refusal was marked as an unreadable reply")
	}
}
