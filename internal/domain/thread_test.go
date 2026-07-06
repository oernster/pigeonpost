package domain

import (
	"testing"
	"time"
)

func threadMessage(t *testing.T, id, subject string, day int, seen bool) MessageSummary {
	t.Helper()
	flags := NewFlags(0)
	if seen {
		flags = NewFlags(FlagSeen)
	}
	msg, err := NewMessageSummary(MessageSummaryInput{
		ID:       id,
		FolderID: "f1",
		UID:      id,
		Subject:  subject,
		Date:     time.Date(2026, time.July, day, 9, 0, 0, 0, time.UTC),
		Flags:    flags,
	})
	if err != nil {
		t.Fatalf("build summary %q: %v", id, err)
	}
	return msg
}

func TestGroupThreadsBySubject(t *testing.T) {
	messages := []MessageSummary{
		threadMessage(t, "a1", "Lunch plans", 1, true),
		threadMessage(t, "b1", "Invoice", 2, false),
		threadMessage(t, "a2", "Re: Lunch plans", 3, false),
		threadMessage(t, "a3", "RE: Fwd: lunch   plans", 4, true),
	}
	threads := GroupThreads(messages)
	if len(threads) != 2 {
		t.Fatalf("got %d threads, want 2", len(threads))
	}
	// The Lunch thread has the most recent message (day 4), so it sorts first.
	lunch := threads[0]
	if lunch.Count() != 3 {
		t.Errorf("lunch thread count = %d, want 3", lunch.Count())
	}
	// Messages are oldest first, and the display subject is the latest message's raw subject.
	if lunch.Messages()[0].ID() != "a1" || lunch.Latest().ID() != "a3" {
		t.Errorf("lunch ordering wrong: first=%s latest=%s", lunch.Messages()[0].ID(), lunch.Latest().ID())
	}
	if lunch.Subject() != "RE: Fwd: lunch   plans" {
		t.Errorf("lunch display subject = %q, want the latest raw subject", lunch.Subject())
	}
	if lunch.UnreadCount() != 1 {
		t.Errorf("lunch unread = %d, want 1 (a2)", lunch.UnreadCount())
	}
	if threads[1].Count() != 1 || threads[1].Latest().ID() != "b1" {
		t.Errorf("second thread = %+v, want the single Invoice message", threads[1])
	}
}

func TestGroupThreadsEmptyInput(t *testing.T) {
	if got := GroupThreads(nil); len(got) != 0 {
		t.Errorf("GroupThreads(nil) = %d threads, want 0", len(got))
	}
}

func TestNormaliseSubject(t *testing.T) {
	cases := map[string]string{
		"Re: Hello":        "hello",
		"RE: FWD: Hello":   "hello",
		"Re[2]: Hello":     "hello",
		"Fw(3): Hello":     "hello",
		"  Hello  world  ": "hello world",
		"Re:":              "",
		"":                 "",
		"Re":               "re",          // a bare prefix with no colon is not a reply marker
		"Rescue plan":      "rescue plan", // "Re" is only a prefix when followed by a colon
	}
	for in, want := range cases {
		if got := normaliseSubject(in); got != want {
			t.Errorf("normaliseSubject(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSkipReplyCountMalformed(t *testing.T) {
	// A bracket that never closes, or holds a non-digit, is left in place rather than stripped.
	if normaliseSubject("Re[x]: Hello") != "re[x]: hello" {
		t.Errorf("a non-numeric count should not be stripped: %q", normaliseSubject("Re[x]: Hello"))
	}
	if normaliseSubject("Re[2: Hello") != "re[2: hello" {
		t.Errorf("an unclosed count should not be stripped: %q", normaliseSubject("Re[2: Hello"))
	}
}
