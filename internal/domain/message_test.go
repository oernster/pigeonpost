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
}

func TestNewTag(t *testing.T) {
	colour, _ := NewColour("#ff8800")
	tag, err := NewTag(" t1 ", "  Important  ", colour)
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
}

func TestNewTagInvalid(t *testing.T) {
	colour, _ := NewColour("#ffffff")
	if _, err := NewTag("  ", "name", colour); !errors.Is(err, ErrEmptyTagID) {
		t.Errorf("empty id error = %v", err)
	}
	if _, err := NewTag("id", "  ", colour); !errors.Is(err, ErrEmptyTagName) {
		t.Errorf("empty name error = %v", err)
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
