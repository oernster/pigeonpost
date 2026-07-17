package storage

// Search-index consistency tests. Each mutation path that must keep the index and the message cache in
// agreement gets its own test, mirroring the consistency contract: insert, body cache, folder replace,
// delete, account removal, flag changes and the structural predicates that stay relational.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// searchMsg is a compact spec for one test message.
type searchMsg struct {
	id, folderID     string
	subject, snippet string
	fromDisplay      string
	fromAddress      string
	toAddress        string
	date             time.Time
	flags            domain.Flag
	hasAttachments   bool
}

func buildSearchMessage(t *testing.T, m searchMsg) domain.MessageSummary {
	t.Helper()
	if m.date.IsZero() {
		m.date = time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC)
	}
	var from domain.EmailAddress
	if m.fromAddress != "" {
		parsed, err := domain.NewEmailAddress(m.fromDisplay, m.fromAddress)
		if err != nil {
			t.Fatalf("sender: %v", err)
		}
		from = parsed
	}
	var to []domain.EmailAddress
	if m.toAddress != "" {
		parsed, err := domain.NewEmailAddress("", m.toAddress)
		if err != nil {
			t.Fatalf("recipient: %v", err)
		}
		to = []domain.EmailAddress{parsed}
	}
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: m.id, FolderID: m.folderID, UID: m.id, MessageID: "<" + m.id + "@x>", From: from, To: to,
		Subject: m.subject, Date: m.date, Size: 10, Flags: domain.NewFlags(m.flags),
		HasAttachments: m.hasAttachments, Snippet: m.snippet,
	})
	if err != nil {
		t.Fatalf("message %q: %v", m.id, err)
	}
	return msg
}

// saveSearchFolder saves one folder row so search's folder join can reach its messages.
func saveSearchFolder(t *testing.T, store *Store, folderID, accountID, path string) {
	t.Helper()
	folder, err := domain.NewFolder(folderID, accountID, path, domain.FolderInbox, 0, 0)
	if err != nil {
		t.Fatalf("folder %q: %v", folderID, err)
	}
	if err := store.SaveFolders(context.Background(), accountID, []domain.Folder{folder}); err != nil {
		t.Fatalf("save folder %q: %v", folderID, err)
	}
}

// searchFor runs raw input through the real parser and the store, returning the hits.
func searchFor(t *testing.T, store *Store, raw string) []application.SearchHit {
	t.Helper()
	hits, err := store.SearchMessages(context.Background(), domain.ParseSearchQuery(raw, time.UTC), 100)
	if err != nil {
		t.Fatalf("search %q: %v", raw, err)
	}
	return hits
}

func hitIDs(hits []application.SearchHit) []string {
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.Summary.ID())
	}
	return ids
}

func wantOnly(t *testing.T, raw string, hits []application.SearchHit, ids ...string) {
	t.Helper()
	if len(hits) != len(ids) {
		t.Fatalf("%q: hits = %v, want %v", raw, hitIDs(hits), ids)
	}
	for i, id := range ids {
		if hits[i].Summary.ID() != id {
			t.Fatalf("%q: hits = %v, want %v", raw, hitIDs(hits), ids)
		}
	}
}

func TestSearchIndexesNewMail(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveSearchFolder(t, store, "f1", "a1", "INBOX")
	msg := buildSearchMessage(t, searchMsg{id: "m1", folderID: "f1", subject: "Quarterly report",
		snippet: "the numbers are in", fromDisplay: "Bob Smith", fromAddress: "bob@example.com",
		toAddress: "alice@example.com"})
	other := buildSearchMessage(t, searchMsg{id: "m2", folderID: "f1", subject: "Lunch plans",
		snippet: "pizza on friday"})
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg, other}); err != nil {
		t.Fatalf("save: %v", err)
	}

	wantOnly(t, "quart", searchFor(t, store, "quart"), "m1")       // subject prefix
	wantOnly(t, "pizza", searchFor(t, store, "pizza"), "m2")       // snippet
	wantOnly(t, "bob", searchFor(t, store, "bob"), "m1")           // sender, bare term
	wantOnly(t, "alice", searchFor(t, store, "alice"), "m1")       // recipient, bare term
	wantOnly(t, "from:bob", searchFor(t, store, "from:bob"), "m1") // sender, field term
	wantOnly(t, "to:alice", searchFor(t, store, "to:alice"), "m1") // recipient, field term
	wantOnly(t, "subject:lunch", searchFor(t, store, "subject:lunch"), "m2")
	if hits := searchFor(t, store, "from:lunch"); len(hits) != 0 {
		t.Errorf("a from: term must not match subject text: %v", hitIDs(hits))
	}

	// The matched term comes back wrapped in the match markers.
	hits := searchFor(t, store, "quarterly")
	if !strings.Contains(hits[0].Snippet, application.SearchMatchStart+"Quarterly"+application.SearchMatchEnd) {
		t.Errorf("snippet %q lacks marked match", hits[0].Snippet)
	}
}

func TestSearchBodyCoversCachedBodies(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveSearchFolder(t, store, "f1", "a1", "INBOX")
	msg := buildSearchMessage(t, searchMsg{id: "m1", folderID: "f1", subject: "Hello", snippet: "opening words"})
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg}); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Before the body is cached, deep body text is not findable.
	if hits := searchFor(t, store, "xylophone"); len(hits) != 0 {
		t.Fatalf("uncached body matched: %v", hitIDs(hits))
	}

	body, err := domain.NewMessageBody("m1", "deep in the text a xylophone is mentioned", "")
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	if err := store.SaveMessageBody(ctx, body); err != nil {
		t.Fatalf("save body: %v", err)
	}

	hits := searchFor(t, store, "xylophone")
	wantOnly(t, "xylophone", hits, "m1")
	if !strings.Contains(hits[0].Snippet, application.SearchMatchStart+"xylophone"+application.SearchMatchEnd) {
		t.Errorf("body snippet %q lacks marked match", hits[0].Snippet)
	}

	// Re-caching the body (a re-open) does not duplicate the hit.
	if err := store.SaveMessageBody(ctx, body); err != nil {
		t.Fatalf("resave body: %v", err)
	}
	wantOnly(t, "xylophone resave", searchFor(t, store, "xylophone"), "m1")
}

func TestSchemaV42ClearsBodiesAndRebuildsSearchIndex(t *testing.T) {
	// schemaV42 clears the cached bodies (so each re-parses with the href-normalising HTML preparation)
	// and rebuilds the search index from message_searchable_text so the index never matches body text
	// the cache no longer holds. Run against a populated store, exactly as the migration runner executes
	// it on an upgraded database; run twice to pin the idempotent re-run the crash-window rule requires.
	store := openTestStore(t)
	ctx := context.Background()
	saveSearchFolder(t, store, "f1", "a1", "INBOX")
	msg := buildSearchMessage(t, searchMsg{id: "m1", folderID: "f1", subject: "Password reset", snippet: "reset requested"})
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg}); err != nil {
		t.Fatalf("save: %v", err)
	}
	body, err := domain.NewMessageBody("m1", "click the tamarind button", "<p>click the tamarind button</p>")
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	if err := store.SaveMessageBody(ctx, body); err != nil {
		t.Fatalf("save body: %v", err)
	}
	wantOnly(t, "tamarind", searchFor(t, store, "tamarind"), "m1")

	for run := 1; run <= 2; run++ {
		if _, err := store.db.ExecContext(ctx, schemaV42); err != nil {
			t.Fatalf("apply schemaV42 (run %d): %v", run, err)
		}
	}

	var cached int
	if err := store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM message_body;").Scan(&cached); err != nil {
		t.Fatalf("count bodies: %v", err)
	}
	if cached != 0 {
		t.Errorf("cached bodies after schemaV42 = %d, want 0", cached)
	}
	if hits := searchFor(t, store, "tamarind"); len(hits) != 0 {
		t.Errorf("cleared body text must not stay searchable: %v", hitIDs(hits))
	}
	// Header-derived text survives the rebuild, exactly as for a message never opened.
	wantOnly(t, "subject after clear", searchFor(t, store, "password"), "m1")
}

func TestSearchAttachmentFilenames(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveSearchFolder(t, store, "f1", "a1", "INBOX")
	msg := buildSearchMessage(t, searchMsg{id: "m1", folderID: "f1", subject: "Invoice attached", hasAttachments: true})
	plain := buildSearchMessage(t, searchMsg{id: "m2", folderID: "f1", subject: "No attachments here"})
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg, plain}); err != nil {
		t.Fatalf("save: %v", err)
	}
	attachment, err := domain.NewAttachment("invoice-march.pdf", "application/pdf", []byte("%PDF"))
	if err != nil {
		t.Fatalf("attachment: %v", err)
	}
	body, err := domain.NewMessageBody("m1", "see attached", "")
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	if err := store.SaveMessageBody(ctx, body.WithAttachments([]domain.Attachment{attachment})); err != nil {
		t.Fatalf("save body: %v", err)
	}

	wantOnly(t, "filename:", searchFor(t, store, "filename:invoice-march"), "m1")
	wantOnly(t, "bare filename", searchFor(t, store, "invoice-march"), "m1")
	wantOnly(t, "has:attachment", searchFor(t, store, "has:attachment"), "m1")
	if hits := searchFor(t, store, "filename:attachments"); len(hits) != 0 {
		t.Errorf("filename: must not match subject text: %v", hitIDs(hits))
	}
}

func TestSearchDeleteRemovesFromIndex(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveSearchFolder(t, store, "f1", "a1", "INBOX")
	msg := buildSearchMessage(t, searchMsg{id: "m1", folderID: "f1", subject: "Doomed message"})
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg}); err != nil {
		t.Fatalf("save: %v", err)
	}
	body, err := domain.NewMessageBody("m1", "body of the doomed", "")
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	if err := store.SaveMessageBody(ctx, body); err != nil {
		t.Fatalf("save body: %v", err)
	}
	if err := store.DeleteMessage(ctx, "m1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if hits := searchFor(t, store, "doomed"); len(hits) != 0 {
		t.Errorf("deleted message still matches: %v", hitIDs(hits))
	}
}

func TestSearchFolderReplaceKeepsIndexInStep(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveSearchFolder(t, store, "f1", "a1", "INBOX")
	kept := buildSearchMessage(t, searchMsg{id: "m1", folderID: "f1", subject: "Kept message"})
	expunged := buildSearchMessage(t, searchMsg{id: "m2", folderID: "f1", subject: "Expunged message"})
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{kept, expunged}); err != nil {
		t.Fatalf("save: %v", err)
	}
	body, err := domain.NewMessageBody("m1", "a body with a walrus in it", "")
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	if err := store.SaveMessageBody(ctx, body); err != nil {
		t.Fatalf("save body: %v", err)
	}

	// The re-sync drops m2 on the server side and keeps m1.
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{kept}); err != nil {
		t.Fatalf("resync: %v", err)
	}
	wantOnly(t, "kept", searchFor(t, store, "kept"), "m1")
	if hits := searchFor(t, store, "expunged"); len(hits) != 0 {
		t.Errorf("expunged message still matches: %v", hitIDs(hits))
	}
	// The kept message's cached body survives the replace and stays searchable: a re-sync never narrows
	// what search covers.
	wantOnly(t, "walrus", searchFor(t, store, "walrus"), "m1")
}

func TestSearchAccountRemoval(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveSearchFolder(t, store, "f1", "a1", "INBOX")
	saveSearchFolder(t, store, "f2", "a2", "INBOX")
	mine := buildSearchMessage(t, searchMsg{id: "m1", folderID: "f1", subject: "Shared word zebra"})
	theirs := buildSearchMessage(t, searchMsg{id: "m2", folderID: "f2", subject: "Shared word zebra"})
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{mine}); err != nil {
		t.Fatalf("save f1: %v", err)
	}
	if err := store.SaveMessages(ctx, "f2", []domain.MessageSummary{theirs}); err != nil {
		t.Fatalf("save f2: %v", err)
	}
	if err := store.DeleteAccountData(ctx, "a1"); err != nil {
		t.Fatalf("delete account data: %v", err)
	}
	wantOnly(t, "zebra", searchFor(t, store, "zebra"), "m2")
}

func TestSearchStatusPredicatesReflectCurrentFlags(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveSearchFolder(t, store, "f1", "a1", "INBOX")
	msg := buildSearchMessage(t, searchMsg{id: "m1", folderID: "f1", subject: "Flagged reading"})
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg}); err != nil {
		t.Fatalf("save: %v", err)
	}

	wantOnly(t, "is:unread", searchFor(t, store, "reading is:unread"), "m1")
	if hits := searchFor(t, store, "reading is:read"); len(hits) != 0 {
		t.Errorf("unread message matched is:read: %v", hitIDs(hits))
	}
	if err := store.SetFlag(ctx, "m1", domain.FlagSeen, true, false); err != nil {
		t.Fatalf("set seen: %v", err)
	}
	wantOnly(t, "is:read", searchFor(t, store, "reading is:read"), "m1")
	if hits := searchFor(t, store, "reading is:unread"); len(hits) != 0 {
		t.Errorf("read message matched is:unread: %v", hitIDs(hits))
	}

	if hits := searchFor(t, store, "reading is:flagged"); len(hits) != 0 {
		t.Errorf("unflagged message matched is:flagged: %v", hitIDs(hits))
	}
	if err := store.SetFlag(ctx, "m1", domain.FlagFlagged, true, false); err != nil {
		t.Fatalf("set flagged: %v", err)
	}
	wantOnly(t, "is:flagged", searchFor(t, store, "reading is:flagged"), "m1")
}

func TestSearchScopesAndDates(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.SaveAccount(ctx, buildAccount(t, "a1")); err != nil {
		t.Fatalf("save account: %v", err)
	}
	saveSearchFolder(t, store, "f1", "a1", "INBOX")
	saveSearchFolder(t, store, "f2", "a2", "Archive")
	january := buildSearchMessage(t, searchMsg{id: "m1", folderID: "f1", subject: "Meeting notes",
		date: time.Date(2026, time.January, 15, 9, 0, 0, 0, time.UTC)})
	june := buildSearchMessage(t, searchMsg{id: "m2", folderID: "f2", subject: "Meeting notes",
		date: time.Date(2026, time.June, 15, 9, 0, 0, 0, time.UTC)})
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{january}); err != nil {
		t.Fatalf("save f1: %v", err)
	}
	if err := store.SaveMessages(ctx, "f2", []domain.MessageSummary{june}); err != nil {
		t.Fatalf("save f2: %v", err)
	}

	wantOnly(t, "in:inbox", searchFor(t, store, "meeting in:inbox"), "m1")
	wantOnly(t, "in:archive", searchFor(t, store, "meeting in:archive"), "m2")
	// account: matches the account's display name (buildAccount names a1 "Primary") case-insensitively.
	wantOnly(t, "account:primary", searchFor(t, store, "meeting account:primary"), "m1")
	wantOnly(t, "before:", searchFor(t, store, "meeting before:2026-02-01"), "m1")
	wantOnly(t, "after:", searchFor(t, store, "meeting after:2026-02-01"), "m2")
	wantOnly(t, "on:", searchFor(t, store, "meeting on:2026-06-15"), "m2")

	// The UI scope selectors travel as exact ids alongside the parsed text.
	scoped := domain.ParseSearchQuery("meeting", time.UTC).WithFolderScope("f2")
	hits, err := store.SearchMessages(ctx, scoped, 100)
	if err != nil {
		t.Fatalf("scoped search: %v", err)
	}
	wantOnly(t, "folder scope", hits, "m2")
	scoped = domain.ParseSearchQuery("meeting", time.UTC).WithAccountScope("a1")
	hits, err = store.SearchMessages(ctx, scoped, 100)
	if err != nil {
		t.Fatalf("account-scoped search: %v", err)
	}
	wantOnly(t, "account scope", hits, "m1")
}

func TestSearchStructuralOnlyQuery(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveSearchFolder(t, store, "f1", "a1", "INBOX")
	older := buildSearchMessage(t, searchMsg{id: "m1", folderID: "f1", subject: "Older", hasAttachments: true,
		date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)})
	newer := buildSearchMessage(t, searchMsg{id: "m2", folderID: "f1", subject: "Newer", hasAttachments: true,
		date: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)})
	plain := buildSearchMessage(t, searchMsg{id: "m3", folderID: "f1", subject: "No attachment"})
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{older, newer, plain}); err != nil {
		t.Fatalf("save: %v", err)
	}
	hits := searchFor(t, store, "has:attachment")
	wantOnly(t, "has:attachment", hits, "m2", "m1") // no text: newest first
	if hits[0].Snippet != "" {
		t.Errorf("structural-only query must carry no snippet, got %q", hits[0].Snippet)
	}
}

func TestSearchGrammarSemantics(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveSearchFolder(t, store, "f1", "a1", "INBOX")
	one := buildSearchMessage(t, searchMsg{id: "m1", folderID: "f1", subject: "green tea ceremony"})
	two := buildSearchMessage(t, searchMsg{id: "m2", folderID: "f1", subject: "tea green harvest"})
	three := buildSearchMessage(t, searchMsg{id: "m3", folderID: "f1", subject: "coffee tasting"})
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{one, two, three}); err != nil {
		t.Fatalf("save: %v", err)
	}

	// A quoted phrase is exact: only the message with the words adjacent and in order matches.
	wantOnly(t, "phrase", searchFor(t, store, `"green tea"`), "m1")
	// Bare terms AND together regardless of order.
	if got := hitIDs(searchFor(t, store, "green tea")); len(got) != 2 {
		t.Errorf("green tea = %v, want both tea messages", got)
	}
	// OR widens.
	if got := hitIDs(searchFor(t, store, "ceremony OR coffee")); len(got) != 2 {
		t.Errorf("OR = %v, want m1 and m3", got)
	}
	// Negation excludes.
	wantOnly(t, "negation", searchFor(t, store, "tea -ceremony"), "m2")
	// Negation without positive text still works (relational base plus NOT IN).
	got := hitIDs(searchFor(t, store, "-tea"))
	if len(got) != 1 || got[0] != "m3" {
		t.Errorf("-tea = %v, want only m3", got)
	}
}

func TestSearchRankingBoostsSubject(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveSearchFolder(t, store, "f1", "a1", "INBOX")
	inBody := buildSearchMessage(t, searchMsg{id: "m1", folderID: "f1", subject: "Weekly minutes",
		date: time.Date(2026, time.July, 2, 0, 0, 0, 0, time.UTC)})
	inSubject := buildSearchMessage(t, searchMsg{id: "m2", folderID: "f1", subject: "Penguin colony report",
		date: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)})
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{inBody, inSubject}); err != nil {
		t.Fatalf("save: %v", err)
	}
	body, err := domain.NewMessageBody("m1", "the penguin was mentioned once deep in the body", "")
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	if err := store.SaveMessageBody(ctx, body); err != nil {
		t.Fatalf("save body: %v", err)
	}
	// The subject match outranks the body-only match even though the body message is newer.
	wantOnly(t, "ranking", searchFor(t, store, "penguin"), "m2", "m1")
}

func TestSearchLimitCapsResults(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	saveSearchFolder(t, store, "f1", "a1", "INBOX")
	msgs := []domain.MessageSummary{
		buildSearchMessage(t, searchMsg{id: "m1", folderID: "f1", subject: "cap test one"}),
		buildSearchMessage(t, searchMsg{id: "m2", folderID: "f1", subject: "cap test two"}),
		buildSearchMessage(t, searchMsg{id: "m3", folderID: "f1", subject: "cap test three"}),
	}
	if err := store.SaveMessages(ctx, "f1", msgs); err != nil {
		t.Fatalf("save: %v", err)
	}
	hits, err := store.SearchMessages(ctx, domain.ParseSearchQuery("cap", time.UTC), 2)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Errorf("limit ignored: %d hits", len(hits))
	}
}
