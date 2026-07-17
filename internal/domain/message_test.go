package domain

import (
	"errors"
	"testing"
	"time"
)

func TestFlags(t *testing.T) {
	f := NewFlags(FlagSeen | FlagFlagged)
	if !f.Has(FlagSeen) || !f.Has(FlagFlagged) {
		t.Error("expected Seen and Flagged set")
	}
	if f.Has(FlagDraft) {
		t.Error("Draft should not be set")
	}
	if !f.IsSeen() {
		t.Error("IsSeen should be true")
	}
	if f.Raw() != (FlagSeen | FlagFlagged) {
		t.Errorf("Raw = %d", f.Raw())
	}

	answered := f.With(FlagAnswered)
	if !answered.Has(FlagAnswered) {
		t.Error("With should set Answered")
	}
	if f.Has(FlagAnswered) {
		t.Error("With must not mutate the original")
	}

	unseen := f.Without(FlagSeen)
	if unseen.IsSeen() {
		t.Error("Without should clear Seen")
	}
	if !f.IsSeen() {
		t.Error("Without must not mutate the original")
	}

	forwarded := f.With(FlagForwarded)
	if !forwarded.Has(FlagForwarded) {
		t.Error("With should set Forwarded")
	}
	if f.Has(FlagForwarded) {
		t.Error("With must not mutate the original for Forwarded")
	}
}

func TestMessageSummaryFlagAccessors(t *testing.T) {
	both, err := NewMessageSummary(MessageSummaryInput{
		ID: "m1", FolderID: "inbox", UID: "1", Flags: NewFlags(FlagAnswered | FlagForwarded),
	})
	if err != nil {
		t.Fatalf("NewMessageSummary: %v", err)
	}
	if !both.IsAnswered() {
		t.Error("IsAnswered should be true when \\Answered is set")
	}
	if !both.IsForwarded() {
		t.Error("IsForwarded should be true when $Forwarded is set")
	}
	if both.IsFlagged() {
		t.Error("IsFlagged should be false when only answered and forwarded are set")
	}

	plain, err := NewMessageSummary(MessageSummaryInput{ID: "m2", FolderID: "inbox", UID: "2", Flags: NewFlags(FlagSeen)})
	if err != nil {
		t.Fatalf("NewMessageSummary: %v", err)
	}
	if plain.IsAnswered() || plain.IsForwarded() {
		t.Error("a seen-only message must be neither answered nor forwarded")
	}
}

func TestNewTag(t *testing.T) {
	colour, _ := NewColour("#ff8800")
	tag, err := NewTag(" t1 ", "  Important  ", colour, " $PPtag_x ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag.ID() != "t1" {
		t.Errorf("ID = %q", tag.ID())
	}
	if tag.Name() != "Important" {
		t.Errorf("Name = %q", tag.Name())
	}
	if tag.Colour().Hex() != "#ff8800" {
		t.Errorf("Colour = %q", tag.Colour().Hex())
	}
	if tag.Keyword() != "$PPtag_x" {
		t.Errorf("Keyword = %q", tag.Keyword())
	}
}

func TestNewTagInvalid(t *testing.T) {
	colour, _ := NewColour("#ffffff")
	if _, err := NewTag("  ", "name", colour, "$PPtag_x"); !errors.Is(err, ErrEmptyTagID) {
		t.Errorf("empty id error = %v", err)
	}
	if _, err := NewTag("id", "  ", colour, "$PPtag_x"); !errors.Is(err, ErrEmptyTagName) {
		t.Errorf("empty name error = %v", err)
	}
	if _, err := NewTag("id", "name", colour, "  "); !errors.Is(err, ErrEmptyTagKeyword) {
		t.Errorf("empty keyword error = %v", err)
	}
}

func TestTagKeyword(t *testing.T) {
	// KeywordForName is the prefix followed by the hex of the trimmed name, so it is a valid IMAP atom.
	if got := KeywordForName("Important"); got != "$PPtag_496d706f7274616e74" {
		t.Errorf("KeywordForName = %q", got)
	}
	// A tag with the same name and case on another device maps to the same keyword, so the assignment
	// syncs; matching is case-sensitive, so a different case maps to a different keyword (dropping the
	// lower-casing keeps the Go derivation identical to the migration's lower(hex(name)) for every name,
	// including non-ASCII ones SQLite's ASCII-only lower() cannot fold).
	if KeywordForName("IMPORTANT") == KeywordForName("Important") {
		t.Error("different case must map to different keywords")
	}
	if KeywordForName("Work") == KeywordForName("Important") {
		t.Error("different names must not share a keyword")
	}
	// A tag returns its stored keyword verbatim; it stays frozen when the tag is rebuilt with a new name,
	// so a rename does not recompute the keyword (which would orphan the tag's server assignments).
	colour, _ := NewColour("#ff8800")
	tag, _ := NewTag("t1", "Important", colour, KeywordForName("Important"))
	if tag.Keyword() != "$PPtag_496d706f7274616e74" {
		t.Errorf("Tag.Keyword = %q", tag.Keyword())
	}
	renamed, _ := NewTag("t1", "Blocked", colour, tag.Keyword())
	if renamed.Keyword() != tag.Keyword() {
		t.Error("keyword must stay frozen, not be recomputed from the new name")
	}
}

func TestIsTagKeyword(t *testing.T) {
	if !IsTagKeyword("$PPtag_696d706f7274616e74") {
		t.Error("a PigeonPost tag keyword should be recognised")
	}
	for _, kw := range []string{"\\Seen", "$Forwarded", "$Label1", "PPtag_x", ""} {
		if IsTagKeyword(kw) {
			t.Errorf("%q must not be recognised as a tag keyword", kw)
		}
	}
}

func TestPendingTagOp(t *testing.T) {
	assign := NewPendingTagOp("m1", "t1", true)
	if assign.MessageID() != "m1" || assign.TagID() != "t1" || !assign.Assigned() {
		t.Errorf("assign op = %+v", assign)
	}
	remove := NewPendingTagOp("m2", "t2", false)
	if remove.MessageID() != "m2" || remove.TagID() != "t2" || remove.Assigned() {
		t.Errorf("remove op = %+v", remove)
	}
}

func TestPendingFlagOp(t *testing.T) {
	set := NewPendingFlagOp("m1", FlagSeen, true)
	if set.MessageID() != "m1" || set.Flag() != FlagSeen || !set.Value() {
		t.Errorf("set op = %+v", set)
	}
	clear := NewPendingFlagOp("m2", FlagFlagged, false)
	if clear.MessageID() != "m2" || clear.Flag() != FlagFlagged || clear.Value() {
		t.Errorf("clear op = %+v", clear)
	}
}

func validMessageInput() MessageSummaryInput {
	from, _ := NewEmailAddress("Sender", "sender@example.com")
	to, _ := NewEmailAddress("", "me@example.com")
	cc, _ := NewEmailAddress("", "team@example.com")
	return MessageSummaryInput{
		ID:             "m1",
		FolderID:       "f1",
		UID:            "42",
		MessageID:      "<abc@example.com>",
		From:           from,
		To:             []EmailAddress{to},
		Cc:             []EmailAddress{cc},
		Subject:        "Hello",
		Date:           time.Date(2026, time.July, 3, 9, 0, 0, 0, time.UTC),
		Size:           2048,
		Flags:          NewFlags(FlagSeen),
		HasAttachments: true,
		Snippet:        "preview text",
	}
}

func TestNewMessageSummary(t *testing.T) {
	m, err := NewMessageSummary(validMessageInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ID() != "m1" {
		t.Errorf("ID = %q", m.ID())
	}
	if m.FolderID() != "f1" {
		t.Errorf("FolderID = %q", m.FolderID())
	}
	if m.UID() != "42" {
		t.Errorf("UID = %q", m.UID())
	}
	if m.MessageID() != "<abc@example.com>" {
		t.Errorf("MessageID = %q", m.MessageID())
	}
	if m.From().Address() != "sender@example.com" {
		t.Errorf("From = %q", m.From().Address())
	}
	if m.Subject() != "Hello" {
		t.Errorf("Subject = %q", m.Subject())
	}
	if !m.Date().Equal(time.Date(2026, time.July, 3, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("Date = %v", m.Date())
	}
	if m.Size() != 2048 {
		t.Errorf("Size = %d", m.Size())
	}
	if !m.HasAttachments() {
		t.Error("HasAttachments should be true")
	}
	if m.Snippet() != "preview text" {
		t.Errorf("Snippet = %q", m.Snippet())
	}
	if !m.IsRead() {
		t.Error("IsRead should be true when Seen is set")
	}
	if m.Flags().Raw() != NewFlags(FlagSeen).Raw() {
		t.Errorf("Flags = %d", m.Flags().Raw())
	}
}

func TestNewMessageSummaryInvalid(t *testing.T) {
	cases := map[string]struct {
		mutate func(*MessageSummaryInput)
		want   error
	}{
		"empty id":      {func(in *MessageSummaryInput) { in.ID = "  " }, ErrEmptyMessageID},
		"empty folder":  {func(in *MessageSummaryInput) { in.FolderID = "  " }, ErrEmptyFolderID},
		"empty uid":     {func(in *MessageSummaryInput) { in.UID = "  " }, ErrInvalidUID},
		"negative size": {func(in *MessageSummaryInput) { in.Size = -1 }, ErrNegativeSize},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := validMessageInput()
			tc.mutate(&in)
			if _, err := NewMessageSummary(in); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestMessageSummaryRecipients(t *testing.T) {
	m, err := NewMessageSummary(validMessageInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.To()) != 1 || m.To()[0].Address() != "me@example.com" {
		t.Errorf("To = %+v", m.To())
	}
	if len(m.Cc()) != 1 || m.Cc()[0].Address() != "team@example.com" {
		t.Errorf("Cc = %+v", m.Cc())
	}
	// The getters must return copies, not the internal slices.
	got := m.To()
	got[0] = EmailAddress{}
	if m.To()[0].IsZero() {
		t.Error("To() must return a copy, not the internal slice")
	}
	cc := m.Cc()
	cc[0] = EmailAddress{}
	if m.Cc()[0].IsZero() {
		t.Error("Cc() must return a copy, not the internal slice")
	}
}

func TestMessageSummaryWithFlags(t *testing.T) {
	m, _ := NewMessageSummary(validMessageInput())
	unread := m.WithFlags(m.Flags().Without(FlagSeen))
	if unread.IsRead() {
		t.Error("expected unread after clearing Seen")
	}
	if !m.IsRead() {
		t.Error("WithFlags must not mutate the original")
	}

	if m.IsFlagged() {
		t.Error("expected not flagged by default")
	}
	flagged := m.WithFlags(m.Flags().With(FlagFlagged))
	if !flagged.IsFlagged() {
		t.Error("expected flagged after setting FlagFlagged")
	}
}

func TestMessageSummaryKeywords(t *testing.T) {
	in := validMessageInput()
	in.Keywords = []string{"$PPtag_abc", "$PPtag_def"}
	m, err := NewMessageSummary(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.Keywords(); len(got) != 2 || got[0] != "$PPtag_abc" || got[1] != "$PPtag_def" {
		t.Errorf("Keywords = %v", got)
	}
	// Mutating the input slice after construction must not reach into the summary.
	in.Keywords[0] = "mutated"
	if m.Keywords()[0] != "$PPtag_abc" {
		t.Error("NewMessageSummary must copy the input keywords")
	}
	// The getter returns a copy, not the internal slice.
	got := m.Keywords()
	got[0] = "mutated"
	if m.Keywords()[0] != "$PPtag_abc" {
		t.Error("Keywords() must return a copy, not the internal slice")
	}
	// A summary built without keywords carries none.
	plain, _ := NewMessageSummary(validMessageInput())
	if len(plain.Keywords()) != 0 {
		t.Errorf("expected no keywords, got %v", plain.Keywords())
	}
}
