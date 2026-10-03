package domain

import (
	"testing"
	"time"
)

func TestEventIDForKeepsAMasterOnItsPlainUID(t *testing.T) {
	if got := EventIDFor(" series@x ", time.Time{}); got != "series@x" {
		t.Fatalf("EventIDFor(master) = %q, want the trimmed UID", got)
	}
}

func TestEventIDForGivesEachOverrideItsOwnID(t *testing.T) {
	first := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	second := first.AddDate(0, 0, 7)
	master := EventIDFor("series@x", time.Time{})
	a := EventIDFor("series@x", first)
	b := EventIDFor("series@x", second)
	if a == master || b == master || a == b {
		t.Fatalf("ids collide: master %q, first %q, second %q", master, a, b)
	}
	want := "series@x" + IDSeparator + first.Format(EventRecurrenceIDLayout)
	if a != want {
		t.Fatalf("EventIDFor(override) = %q, want %q", a, want)
	}
}

func TestEventIDForNamesOneInstantOnceWhateverItsZone(t *testing.T) {
	london, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Skipf("zone data unavailable: %v", err)
	}
	instant := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	if EventIDFor("u", instant) != EventIDFor("u", instant.In(london)) {
		t.Fatal("one instant in two zones gave two ids")
	}
}

func TestMessageIDForJoinsFolderAndUIDWithTheSeparator(t *testing.T) {
	got := MessageIDFor("acc\x1fINBOX", "42")
	want := "acc\x1fINBOX" + IDSeparator + "42"
	if got != want {
		t.Fatalf("MessageIDFor = %q, want %q", got, want)
	}
}

func TestFolderIDForJoinsAccountAndPathWithTheSeparator(t *testing.T) {
	got := FolderIDFor("acc", "Shopping.Music")
	want := "acc" + IDSeparator + "Shopping.Music"
	if got != want {
		t.Fatalf("FolderIDFor = %q, want %q", got, want)
	}
}

// The two are exact inverses: a rules file writes the halves and reads them back, so a path holding
// anything unusual (a dot hierarchy, a space, an emoji) has to survive the round trip untouched.
func TestSplitFolderIDRecoversWhatFolderIDForJoined(t *testing.T) {
	for _, path := range []string{"INBOX", "Shopping.Music", "Work/Client one", "Archive 2026"} {
		account, got := SplitFolderID(FolderIDFor("acc@example.com", path))
		if account != "acc@example.com" || got != path {
			t.Errorf("round trip of %q gave (%q, %q)", path, account, got)
		}
	}
}

// An id with no separator is not a folder id; neither is the empty string a non-move action
// carries. Both yield nothing rather than a guess, so a caller cannot read a malformed id as a folder
// sitting at the top level of some account.
func TestSplitFolderIDYieldsNothingForAnIDThatIsNotOne(t *testing.T) {
	for _, id := range []string{"", "no-separator-here"} {
		account, path := SplitFolderID(id)
		if account != "" || path != "" {
			t.Errorf("SplitFolderID(%q) = (%q, %q), want two empty strings", id, account, path)
		}
	}
}
