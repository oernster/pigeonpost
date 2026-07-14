package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pigeonpost.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func buildAccount(t *testing.T, id string) domain.Account {
	t.Helper()
	addr, err := domain.NewEmailAddress("", "user@example.com")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	in, err := domain.NewServerConfig("imap.example.com", 993, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("incoming: %v", err)
	}
	out, err := domain.NewServerConfig("smtp.example.com", 465, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("outgoing: %v", err)
	}
	account, err := domain.NewAccount(id, "Primary", addr, domain.ProtocolIMAP, in, out, domain.AuthPassword)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	return account
}

func TestAccountRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SaveAccount(ctx, buildAccount(t, "a1")); err != nil {
		t.Fatalf("save account: %v", err)
	}

	accounts, err := store.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	got := accounts[0]
	if got.ID() != "a1" || got.Address().Address() != "user@example.com" {
		t.Errorf("unexpected account: %+v", got)
	}
	if got.Incoming().Port() != 993 || got.Outgoing().Host() != "smtp.example.com" {
		t.Errorf("server config lost: %+v", got.Incoming())
	}

	fetched, err := store.GetAccount(ctx, "a1")
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if fetched.DisplayName() != "Primary" {
		t.Errorf("DisplayName = %q", fetched.DisplayName())
	}

	if _, err := store.GetAccount(ctx, "missing"); !errors.Is(err, application.ErrAccountNotFound) {
		t.Errorf("missing account error = %v, want ErrAccountNotFound", err)
	}
}

func TestSaveAccountReplaces(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.SaveAccount(ctx, buildAccount(t, "a1")); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := store.SaveAccount(ctx, buildAccount(t, "a1")); err != nil {
		t.Fatalf("second save: %v", err)
	}
	accounts, err := store.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(accounts) != 1 {
		t.Errorf("expected replace, got %d accounts", len(accounts))
	}
}

func accountIDs(t *testing.T, store *Store) []string {
	t.Helper()
	accounts, err := store.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	ids := make([]string, len(accounts))
	for i, a := range accounts {
		ids[i] = a.ID()
	}
	return ids
}

func equalIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAccountOrdering(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	for _, id := range []string{"a1", "a2", "a3"} {
		if err := store.SaveAccount(ctx, buildAccount(t, id)); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}
	// New accounts append in insertion order.
	if got := accountIDs(t, store); !equalIDs(got, []string{"a1", "a2", "a3"}) {
		t.Fatalf("initial order = %v, want [a1 a2 a3]", got)
	}

	// A manual reorder is persisted.
	if err := store.SetAccountPositions(ctx, []string{"a3", "a1", "a2"}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if got := accountIDs(t, store); !equalIDs(got, []string{"a3", "a1", "a2"}) {
		t.Fatalf("reordered = %v, want [a3 a1 a2]", got)
	}

	// Editing an account (a re-save of the same id) keeps its position.
	if err := store.SaveAccount(ctx, buildAccount(t, "a1")); err != nil {
		t.Fatalf("re-save a1: %v", err)
	}
	if got := accountIDs(t, store); !equalIDs(got, []string{"a3", "a1", "a2"}) {
		t.Fatalf("order after re-save = %v, want [a3 a1 a2] (position kept)", got)
	}

	// A brand-new account appends at the end rather than jumping to the top.
	if err := store.SaveAccount(ctx, buildAccount(t, "a4")); err != nil {
		t.Fatalf("save a4: %v", err)
	}
	if got := accountIDs(t, store); !equalIDs(got, []string{"a3", "a1", "a2", "a4"}) {
		t.Fatalf("order after append = %v, want [a3 a1 a2 a4]", got)
	}
}

func TestDeleteAccountAndData(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SaveAccount(ctx, buildAccount(t, "a1")); err != nil {
		t.Fatalf("save account: %v", err)
	}
	inbox, _ := domain.NewFolder("f1", "a1", "INBOX", domain.FolderInbox, 1, 1)
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{inbox}); err != nil {
		t.Fatalf("save folders: %v", err)
	}
	msg := buildMessage(t, "m1", time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), true)
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg}); err != nil {
		t.Fatalf("save messages: %v", err)
	}

	if err := store.DeleteAccountData(ctx, "a1"); err != nil {
		t.Fatalf("delete account data: %v", err)
	}
	folders, _ := store.ListFolders(ctx, "a1")
	if len(folders) != 0 {
		t.Errorf("expected folders cleared, got %d", len(folders))
	}
	messages, _ := store.ListMessages(ctx, "f1")
	if len(messages) != 0 {
		t.Errorf("expected messages cleared, got %d", len(messages))
	}

	if err := store.DeleteAccount(ctx, "a1"); err != nil {
		t.Fatalf("delete account: %v", err)
	}
	accounts, _ := store.ListAccounts(ctx)
	if len(accounts) != 0 {
		t.Errorf("expected account removed, got %d", len(accounts))
	}
	// Deleting an absent account is not an error.
	if err := store.DeleteAccount(ctx, "a1"); err != nil {
		t.Errorf("delete missing account: %v", err)
	}
}

func TestFolderRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	inbox, _ := domain.NewFolder("f1", "a1", "INBOX", domain.FolderInbox, 2, 5)
	archive, _ := domain.NewFolder("f2", "a1", "INBOX/Archive", domain.FolderArchive, 0, 3)
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{inbox, archive}); err != nil {
		t.Fatalf("save folders: %v", err)
	}

	folders, err := store.ListFolders(ctx, "a1")
	if err != nil {
		t.Fatalf("list folders: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(folders))
	}
	// Unread and total are computed from the cached messages, not the stored columns, so with no
	// messages saved both are zero here regardless of the values passed to SaveFolders. The computed
	// counts have their own test below.
	if folders[0].Path() != "INBOX" || folders[0].Unread() != 0 || folders[0].Total() != 0 {
		t.Errorf("unexpected first folder: %+v", folders[0])
	}

	// Saving again replaces rather than accumulates.
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{inbox}); err != nil {
		t.Fatalf("resave folders: %v", err)
	}
	folders, _ = store.ListFolders(ctx, "a1")
	if len(folders) != 1 {
		t.Errorf("expected replace to 1 folder, got %d", len(folders))
	}
}

// buildMessageIn builds a minimal cached message in a chosen folder with a chosen read state, for the
// count tests. An unread message has the Seen bit clear.
func buildMessageIn(t *testing.T, id, folderID string, read bool) domain.MessageSummary {
	t.Helper()
	flags := domain.NewFlags(0)
	if read {
		flags = domain.NewFlags(domain.FlagSeen)
	}
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: folderID, UID: id, MessageID: "<" + id + "@x>",
		Subject: "Subject " + id, Date: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		Size: 1024, Flags: flags, Snippet: "snippet " + id,
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return msg
}

func TestFolderCountsAreComputedFromMessages(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	// The stored counts are deliberately wrong (0/0); the computed counts must ignore them and reflect
	// the cached messages: two unread of three total.
	inbox, _ := domain.NewFolder("f1", "a1", "INBOX", domain.FolderInbox, 0, 0)
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{inbox}); err != nil {
		t.Fatalf("save folders: %v", err)
	}
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{
		buildMessageIn(t, "m1", "f1", false),
		buildMessageIn(t, "m2", "f1", false),
		buildMessageIn(t, "m3", "f1", true),
	}); err != nil {
		t.Fatalf("save messages: %v", err)
	}

	folders, err := store.ListFolders(ctx, "a1")
	if err != nil {
		t.Fatalf("list folders: %v", err)
	}
	if folders[0].Unread() != 2 || folders[0].Total() != 3 {
		t.Errorf("ListFolders counts wrong: unread=%d total=%d, want 2/3",
			folders[0].Unread(), folders[0].Total())
	}

	got, err := store.GetFolder(ctx, "f1")
	if err != nil {
		t.Fatalf("get folder: %v", err)
	}
	if got.Unread() != 2 || got.Total() != 3 {
		t.Errorf("GetFolder counts wrong: unread=%d total=%d, want 2/3", got.Unread(), got.Total())
	}
}

func TestUnreadByAccount(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	// Three accounts: a1 has 2 unread across two folders, a2 has 1, a3 has none (so it is absent from
	// the returned map).
	a1inbox, _ := domain.NewFolder("f1", "a1", "INBOX", domain.FolderInbox, 0, 0)
	a1arch, _ := domain.NewFolder("f2", "a1", "Archive", domain.FolderArchive, 0, 0)
	a2inbox, _ := domain.NewFolder("f3", "a2", "INBOX", domain.FolderInbox, 0, 0)
	a3inbox, _ := domain.NewFolder("f4", "a3", "INBOX", domain.FolderInbox, 0, 0)
	for accountID, folders := range map[string][]domain.Folder{
		"a1": {a1inbox, a1arch},
		"a2": {a2inbox},
		"a3": {a3inbox},
	} {
		if err := store.SaveFolders(ctx, accountID, folders); err != nil {
			t.Fatalf("save %s folders: %v", accountID, err)
		}
	}
	mustSave := func(folderID string, msgs ...domain.MessageSummary) {
		if err := store.SaveMessages(ctx, folderID, msgs); err != nil {
			t.Fatalf("save messages in %q: %v", folderID, err)
		}
	}
	mustSave("f1", buildMessageIn(t, "m1", "f1", false), buildMessageIn(t, "m2", "f1", true))
	mustSave("f2", buildMessageIn(t, "m3", "f2", false))
	mustSave("f3", buildMessageIn(t, "m4", "f3", false))
	mustSave("f4", buildMessageIn(t, "m5", "f4", true))

	counts, err := store.UnreadByAccount(ctx)
	if err != nil {
		t.Fatalf("unread by account: %v", err)
	}
	if counts["a1"] != 2 {
		t.Errorf("a1 unread = %d, want 2", counts["a1"])
	}
	if counts["a2"] != 1 {
		t.Errorf("a2 unread = %d, want 1", counts["a2"])
	}
	if _, ok := counts["a3"]; ok {
		t.Errorf("a3 should be absent from the map, got %d", counts["a3"])
	}
}

func buildMessage(t *testing.T, id string, when time.Time, withSender bool) domain.MessageSummary {
	t.Helper()
	var from domain.EmailAddress
	if withSender {
		var err error
		from, err = domain.NewEmailAddress("Alice", "alice@example.com")
		if err != nil {
			t.Fatalf("sender: %v", err)
		}
	}
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: "f1", UID: "1", MessageID: "<" + id + "@x>", From: from,
		Subject: "Subject " + id, Date: when, Size: 1024, Flags: domain.NewFlags(domain.FlagSeen),
		HasAttachments: withSender, Snippet: "snippet " + id,
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return msg
}

func TestMessageRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	older := buildMessage(t, "m1", time.Date(2026, time.July, 1, 8, 0, 0, 0, time.UTC), true)
	newer := buildMessage(t, "m2", time.Date(2026, time.July, 2, 8, 0, 0, 0, time.UTC), false)
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{older, newer}); err != nil {
		t.Fatalf("save messages: %v", err)
	}

	messages, err := store.ListMessages(ctx, "f1")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	// Newest first.
	if messages[0].ID() != "m2" {
		t.Errorf("ordering wrong: first = %q, want m2", messages[0].ID())
	}
	if messages[1].From().Address() != "alice@example.com" {
		t.Errorf("sender lost: %q", messages[1].From().Address())
	}
	if !messages[0].From().IsZero() {
		t.Errorf("expected zero sender for m2, got %q", messages[0].From().Address())
	}
	if !messages[0].IsRead() {
		t.Error("Seen flag lost in round trip")
	}
}

func TestListMessagesPage(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	d1 := time.Date(2026, time.July, 1, 8, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, time.July, 2, 8, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, time.July, 3, 8, 0, 0, 0, time.UTC)
	// mA and mB share the newest timestamp, so the (date, id) tie-break must order them by id.
	msgs := []domain.MessageSummary{
		buildMessage(t, "m1", d1, true),
		buildMessage(t, "m2", d2, true),
		buildMessage(t, "mA", d3, true),
		buildMessage(t, "mB", d3, true),
	}
	if err := store.SaveMessages(ctx, "f1", msgs); err != nil {
		t.Fatalf("save messages: %v", err)
	}
	ids := func(page []domain.MessageSummary) []string {
		out := make([]string, len(page))
		for i, m := range page {
			out[i] = m.ID()
		}
		return out
	}

	// First page, newest first, limit two: the two newest share d3, so id DESC gives mB then mA.
	first, err := store.ListMessagesPage(ctx, "f1", false, 0, "", 2, false)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if got := ids(first); len(got) != 2 || got[0] != "mB" || got[1] != "mA" {
		t.Fatalf("first page = %v, want [mB mA]", got)
	}

	// Second page resumes strictly after the first page's last row (mA at d3): m2 then m1.
	last := first[len(first)-1]
	second, err := store.ListMessagesPage(ctx, "f1", true, last.Date().UnixMilli(), last.ID(), 2, false)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if got := ids(second); len(got) != 2 || got[0] != "m2" || got[1] != "m1" {
		t.Fatalf("second page = %v, want [m2 m1]", got)
	}

	// A third page past the end is empty, ending the walk.
	lastTwo := second[len(second)-1]
	third, err := store.ListMessagesPage(ctx, "f1", true, lastTwo.Date().UnixMilli(), lastTwo.ID(), 2, false)
	if err != nil {
		t.Fatalf("third page: %v", err)
	}
	if len(third) != 0 {
		t.Fatalf("third page = %v, want empty", ids(third))
	}

	// Ascending walks oldest first: m1, m2, then mA before mB (id ASC on the shared d3).
	asc, err := store.ListMessagesPage(ctx, "f1", false, 0, "", 10, true)
	if err != nil {
		t.Fatalf("ascending page: %v", err)
	}
	if got := ids(asc); len(got) != 4 || got[0] != "m1" || got[1] != "m2" || got[2] != "mA" || got[3] != "mB" {
		t.Fatalf("ascending page = %v, want [m1 m2 mA mB]", got)
	}
}

func TestSetSeen(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	msg := buildMessage(t, "m1", time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), true)
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg}); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := store.SetSeen(ctx, "m1", false); err != nil {
		t.Fatalf("clear seen: %v", err)
	}
	msgs, _ := store.ListMessages(ctx, "f1")
	if msgs[0].IsRead() {
		t.Error("expected unread after clearing seen")
	}

	if err := store.SetSeen(ctx, "m1", true); err != nil {
		t.Fatalf("set seen: %v", err)
	}
	msgs, _ = store.ListMessages(ctx, "f1")
	if !msgs[0].IsRead() {
		t.Error("expected read after setting seen")
	}

	if err := store.SetSeen(ctx, "missing", true); err == nil {
		t.Error("expected error for a missing message")
	}
}

func TestSetAnswered(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	msg := buildMessage(t, "m1", time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), true)
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg}); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := store.SetAnswered(ctx, "m1", true); err != nil {
		t.Fatalf("set answered: %v", err)
	}
	msgs, _ := store.ListMessages(ctx, "f1")
	if !msgs[0].IsAnswered() {
		t.Error("expected answered after setting it")
	}
	if !msgs[0].IsRead() {
		t.Error("setting answered must not disturb the seen flag")
	}

	if err := store.SetAnswered(ctx, "m1", false); err != nil {
		t.Fatalf("clear answered: %v", err)
	}
	msgs, _ = store.ListMessages(ctx, "f1")
	if msgs[0].IsAnswered() {
		t.Error("expected not answered after clearing it")
	}

	if err := store.SetAnswered(ctx, "missing", true); err == nil {
		t.Error("expected error for a missing message")
	}
}

func TestSetForwarded(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	msg := buildMessage(t, "m1", time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), true)
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg}); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := store.SetForwarded(ctx, "m1", true); err != nil {
		t.Fatalf("set forwarded: %v", err)
	}
	// The forwarded bit is new; asserting it survives a SaveMessages then ListMessages round-trip confirms the
	// existing integer flags column carries it with no schema change.
	msgs, _ := store.ListMessages(ctx, "f1")
	if !msgs[0].IsForwarded() {
		t.Error("expected forwarded after setting it")
	}

	if err := store.SetForwarded(ctx, "m1", false); err != nil {
		t.Fatalf("clear forwarded: %v", err)
	}
	msgs, _ = store.ListMessages(ctx, "f1")
	if msgs[0].IsForwarded() {
		t.Error("expected not forwarded after clearing it")
	}
}

func buildTag(t *testing.T, id, name, hex string) domain.Tag {
	t.Helper()
	colour, err := domain.NewColour(hex)
	if err != nil {
		t.Fatalf("colour: %v", err)
	}
	tag, err := domain.NewTag(id, name, colour, domain.KeywordForName(name))
	if err != nil {
		t.Fatalf("tag: %v", err)
	}
	return tag
}

func TestTagRoundTripAndAssignment(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SaveTag(ctx, buildTag(t, "t1", "Work", "#3366ff")); err != nil {
		t.Fatalf("save tag: %v", err)
	}
	if err := store.SaveTag(ctx, buildTag(t, "t2", "Personal", "#ff8800")); err != nil {
		t.Fatalf("save tag: %v", err)
	}

	tags, err := store.ListTags(ctx)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(tags) != 2 || tags[0].Name() != "Personal" {
		t.Fatalf("expected 2 tags ordered by name, got %+v", tags)
	}

	if err := store.AddMessageTag(ctx, "m1", "t1"); err != nil {
		t.Fatalf("attach: %v", err)
	}
	// Re-attaching is a no-op, not an error.
	if err := store.AddMessageTag(ctx, "m1", "t1"); err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	if err := store.AddMessageTag(ctx, "m1", "t2"); err != nil {
		t.Fatalf("attach: %v", err)
	}

	forMsg, err := store.TagsForMessage(ctx, "m1")
	if err != nil {
		t.Fatalf("tags for message: %v", err)
	}
	if len(forMsg) != 2 {
		t.Fatalf("expected 2 tags on m1, got %d", len(forMsg))
	}

	colours, err := store.TagColoursForMessages(ctx, []string{"m1", "m2"})
	if err != nil {
		t.Fatalf("tag colours for messages: %v", err)
	}
	got := colours["m1"]
	if len(got) != 2 {
		t.Fatalf("expected 2 tag colours on m1, got %+v", got)
	}
	inColours := map[string]bool{got[0]: true, got[1]: true}
	if !inColours["#3366ff"] || !inColours["#ff8800"] {
		t.Fatalf("expected m1 colours to include #3366ff and #ff8800, got %+v", got)
	}
	if _, ok := colours["m2"]; ok {
		t.Errorf("expected no colours entry for untagged m2")
	}
	if empty, emptyErr := store.TagColoursForMessages(ctx, nil); emptyErr != nil || len(empty) != 0 {
		t.Errorf("empty ids: colours %+v, err %v", empty, emptyErr)
	}

	if err := store.RemoveMessageTag(ctx, "m1", "t1"); err != nil {
		t.Fatalf("detach: %v", err)
	}
	forMsg, _ = store.TagsForMessage(ctx, "m1")
	if len(forMsg) != 1 || forMsg[0].ID() != "t2" {
		t.Fatalf("expected only t2 left, got %+v", forMsg)
	}

	// Deleting a tag detaches it everywhere.
	if err := store.DeleteTag(ctx, "t2"); err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	forMsg, _ = store.TagsForMessage(ctx, "m1")
	if len(forMsg) != 0 {
		t.Errorf("expected no tags after deleting t2, got %d", len(forMsg))
	}
	tags, _ = store.ListTags(ctx)
	if len(tags) != 1 {
		t.Errorf("expected 1 tag remaining, got %d", len(tags))
	}
}

func TestPendingTagOps(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	// Record intents: assign t1 and remove t2 on m1, assign t1 on m2.
	for _, op := range []struct {
		messageID, tagID string
		assigned         bool
	}{{"m1", "t1", true}, {"m1", "t2", false}, {"m2", "t1", true}} {
		if err := store.SetPendingTagOp(ctx, op.messageID, op.tagID, op.assigned); err != nil {
			t.Fatalf("set pending %s/%s: %v", op.messageID, op.tagID, err)
		}
	}

	pending, err := store.PendingTagOps(ctx, "m1")
	if err != nil {
		t.Fatalf("pending tag ops: %v", err)
	}
	if len(pending) != 2 || pending["t1"] != true || pending["t2"] != false {
		t.Fatalf("pending for m1 = %+v", pending)
	}

	// Setting the same pair again replaces the intent.
	if err := store.SetPendingTagOp(ctx, "m1", "t1", false); err != nil {
		t.Fatalf("replace pending: %v", err)
	}
	if pending, _ = store.PendingTagOps(ctx, "m1"); pending["t1"] != false {
		t.Fatalf("expected replaced intent false, got %+v", pending)
	}

	all, err := store.ListPendingTagOps(ctx)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 pending ops, got %d", len(all))
	}

	// Clearing one pair leaves the rest.
	if err := store.ClearPendingTagOp(ctx, "m1", "t1"); err != nil {
		t.Fatalf("clear pending: %v", err)
	}
	pending, _ = store.PendingTagOps(ctx, "m1")
	if _, ok := pending["t1"]; ok || len(pending) != 1 {
		t.Fatalf("expected only t2 pending left for m1, got %+v", pending)
	}

	// Deleting a tag clears its pending ops everywhere (here m2/t1).
	if err := store.SaveTag(ctx, buildTag(t, "t1", "Work", "#3366ff")); err != nil {
		t.Fatalf("save tag: %v", err)
	}
	if err := store.DeleteTag(ctx, "t1"); err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	all, _ = store.ListPendingTagOps(ctx)
	for _, op := range all {
		if op.TagID() == "t1" {
			t.Errorf("expected t1 pending cleared by DeleteTag, got %+v", op)
		}
	}

	// Deleting a message clears its pending ops.
	if err := store.DeleteMessage(ctx, "m1"); err != nil {
		t.Fatalf("delete message: %v", err)
	}
	if pending, _ = store.PendingTagOps(ctx, "m1"); len(pending) != 0 {
		t.Errorf("expected no pending for m1 after delete, got %+v", pending)
	}
}

func TestAssignUnassignMessageTagAtomic(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.SaveTag(ctx, buildTag(t, "t1", "Work", "#3366ff")); err != nil {
		t.Fatalf("save tag: %v", err)
	}

	// Assign with recordPending=true writes the link and the pending intent together.
	if err := store.AssignMessageTag(ctx, "m1", "t1", true); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if tags, _ := store.TagsForMessage(ctx, "m1"); len(tags) != 1 || tags[0].ID() != "t1" {
		t.Fatalf("expected t1 on m1, got %+v", tags)
	}
	if p, _ := store.PendingTagOps(ctx, "m1"); !p["t1"] {
		t.Errorf("expected a pending assign, got %v", p)
	}

	// recordPending=false (a POP3 account) writes only the link.
	if err := store.AssignMessageTag(ctx, "m2", "t1", false); err != nil {
		t.Fatalf("assign local-only: %v", err)
	}
	if p, _ := store.PendingTagOps(ctx, "m2"); len(p) != 0 {
		t.Errorf("a local-only assign must record no pending, got %v", p)
	}

	// Unassign with recordPending=true removes the link and records the removal intent.
	if err := store.UnassignMessageTag(ctx, "m1", "t1", true); err != nil {
		t.Fatalf("unassign: %v", err)
	}
	if tags, _ := store.TagsForMessage(ctx, "m1"); len(tags) != 0 {
		t.Errorf("expected t1 detached, got %+v", tags)
	}
	if p, _ := store.PendingTagOps(ctx, "m1"); p["t1"] != false {
		t.Errorf("expected a pending unassign (false), got %v", p)
	}
	// recordPending=false unassign writes only the removal.
	if err := store.UnassignMessageTag(ctx, "m2", "t1", false); err != nil {
		t.Fatalf("unassign local-only: %v", err)
	}
	if p, _ := store.PendingTagOps(ctx, "m2"); len(p) != 0 {
		t.Errorf("a local-only unassign must record no pending, got %v", p)
	}
}

func TestSaveMessagesSweepsOrphanedPendingTags(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	// Cache a message in f1 and record a pending tag op on it: this one must survive a folder replace.
	kept := buildMessageIn(t, "m-kept", "f1", false)
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{kept}); err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if err := store.SetPendingTagOp(ctx, "m-kept", "t1", true); err != nil {
		t.Fatalf("set pending kept: %v", err)
	}
	// A pending op for a message that is no longer cached (expunged on the server) must be swept.
	if err := store.SetPendingTagOp(ctx, "m-gone", "t1", true); err != nil {
		t.Fatalf("set pending gone: %v", err)
	}

	// A folder replace sweeps the orphaned pending row but keeps the one whose message is still cached.
	if err := store.SaveMessages(ctx, "f2", nil); err != nil {
		t.Fatalf("save messages f2: %v", err)
	}
	if p, _ := store.PendingTagOps(ctx, "m-kept"); !p["t1"] {
		t.Errorf("a cached message's pending op must survive, got %v", p)
	}
	if p, _ := store.PendingTagOps(ctx, "m-gone"); len(p) != 0 {
		t.Errorf("an orphaned pending op must be swept, got %v", p)
	}
}

func TestMessageBodyCache(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if _, err := store.GetMessageBody(ctx, "m1"); !errors.Is(err, application.ErrBodyNotCached) {
		t.Errorf("expected ErrBodyNotCached, got %v", err)
	}

	body, _ := domain.NewMessageBody("m1", "plain text", "<p>html</p>")
	if err := store.SaveMessageBody(ctx, body); err != nil {
		t.Fatalf("save body: %v", err)
	}
	got, err := store.GetMessageBody(ctx, "m1")
	if err != nil {
		t.Fatalf("get body: %v", err)
	}
	if got.Plain() != "plain text" || got.HTML() != "<p>html</p>" {
		t.Errorf("round-tripped body wrong: %+v", got)
	}
	if got.HasInvite() {
		t.Errorf("a body saved with no invite should round-trip without one")
	}

	invite := []byte("BEGIN:VCALENDAR\r\nMETHOD:REQUEST\r\nEND:VCALENDAR\r\n")
	if err := store.SaveMessageBody(ctx, body.WithInvite(invite)); err != nil {
		t.Fatalf("save body with invite: %v", err)
	}
	withInvite, err := store.GetMessageBody(ctx, "m1")
	if err != nil {
		t.Fatalf("get body with invite: %v", err)
	}
	if !withInvite.HasInvite() || string(withInvite.Invite()) != string(invite) {
		t.Errorf("invite did not round-trip through storage: %q", withInvite.Invite())
	}
}

func TestMessageBodyAttachmentsRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	body, _ := domain.NewMessageBody("m1", "see attached", "")
	pdf, _ := domain.NewAttachment("report.pdf", "application/pdf", []byte("PDF"))
	txt, _ := domain.NewAttachment("notes.txt", "text/plain", []byte("hello"))
	if err := store.SaveMessageBody(ctx, body.WithAttachments([]domain.Attachment{pdf, txt})); err != nil {
		t.Fatalf("save body with attachments: %v", err)
	}
	got, err := store.GetMessageBody(ctx, "m1")
	if err != nil {
		t.Fatalf("get body: %v", err)
	}
	atts := got.Attachments()
	if len(atts) != 2 {
		t.Fatalf("got %d attachments, want 2", len(atts))
	}
	// Order is preserved and content round-trips.
	if atts[0].Filename() != "report.pdf" || string(atts[0].Content()) != "PDF" {
		t.Errorf("first attachment = %+v", atts[0])
	}
	if atts[1].Filename() != "notes.txt" || atts[1].ContentType() != "text/plain" {
		t.Errorf("second attachment = %+v", atts[1])
	}

	// Re-saving with a smaller set replaces the previous attachments rather than appending.
	if err := store.SaveMessageBody(ctx, body.WithAttachments([]domain.Attachment{txt})); err != nil {
		t.Fatalf("re-save body: %v", err)
	}
	reGot, err := store.GetMessageBody(ctx, "m1")
	if err != nil {
		t.Fatalf("get body after re-save: %v", err)
	}
	if len(reGot.Attachments()) != 1 || reGot.Attachments()[0].Filename() != "notes.txt" {
		t.Errorf("re-saved attachments = %+v, want just notes.txt", reGot.Attachments())
	}

	// Deleting the message clears its cached attachments.
	if err := store.DeleteMessage(ctx, "m1"); err != nil {
		t.Fatalf("delete message: %v", err)
	}
	if _, err := store.GetMessageBody(ctx, "m1"); !errors.Is(err, application.ErrBodyNotCached) {
		t.Errorf("after delete: %v, want ErrBodyNotCached", err)
	}
}

func TestGetMessageAndFolder(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	inbox, _ := domain.NewFolder("f1", "a1", "INBOX", domain.FolderInbox, 0, 1)
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{inbox}); err != nil {
		t.Fatalf("save folders: %v", err)
	}
	msg := buildMessage(t, "m1", time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), true)
	if err := store.SaveMessages(ctx, "f1", []domain.MessageSummary{msg}); err != nil {
		t.Fatalf("save messages: %v", err)
	}

	gotMsg, err := store.GetMessage(ctx, "m1")
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if gotMsg.FolderID() != "f1" || gotMsg.UID() != "1" {
		t.Errorf("message wrong: %+v", gotMsg)
	}
	gotFolder, err := store.GetFolder(ctx, "f1")
	if err != nil {
		t.Fatalf("get folder: %v", err)
	}
	if gotFolder.AccountID() != "a1" || gotFolder.Path() != "INBOX" {
		t.Errorf("folder wrong: %+v", gotFolder)
	}

	if _, err := store.GetFolder(ctx, "missing"); err == nil {
		t.Error("expected error for a missing folder")
	}
}

func TestReopenPersistsAndMigratesOnce(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "persist.db")

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := store.SaveAccount(ctx, buildAccount(t, "a1")); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	accounts, err := reopened.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("list after reopen: %v", err)
	}
	if len(accounts) != 1 {
		t.Errorf("expected persisted account after reopen, got %d", len(accounts))
	}
}
