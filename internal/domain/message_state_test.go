package domain

import (
	"errors"
	"reflect"
	"testing"
)

func summaryFor(t *testing.T, uid, subject string, flags Flags) MessageSummary {
	t.Helper()
	m, err := NewMessageSummary(MessageSummaryInput{ID: "f\x1f" + uid, FolderID: "f", UID: uid, Subject: subject, Flags: flags})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	return m
}

func stateFor(t *testing.T, uid string, flags Flags, keywords ...string) MessageState {
	t.Helper()
	s, err := NewMessageState(uid, flags, keywords)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	return s
}

func TestNewMessageStateRefusesABlankUID(t *testing.T) {
	if _, err := NewMessageState("  ", NewFlags(0), nil); !errors.Is(err, ErrInvalidUID) {
		t.Errorf("error = %v, want ErrInvalidUID", err)
	}
	s := stateFor(t, " 7 ", NewFlags(0).With(FlagSeen), "$PPtag_work")
	if s.UID() != "7" || !s.Flags().Has(FlagSeen) || !reflect.DeepEqual(s.Keywords(), []string{"$PPtag_work"}) {
		t.Errorf("state = %q %v %v", s.UID(), s.Flags(), s.Keywords())
	}
}

// Only listed UIDs the cache does not hold need their summaries fetched, in listing order.
func TestNewUIDsAnswersWhatTheCacheLacks(t *testing.T) {
	cached := []MessageSummary{summaryFor(t, "1", "a", NewFlags(0)), summaryFor(t, "3", "c", NewFlags(0))}
	listing := []MessageState{stateFor(t, "1", NewFlags(0)), stateFor(t, "2", NewFlags(0)), stateFor(t, "3", NewFlags(0)), stateFor(t, "4", NewFlags(0))}
	if got := NewUIDs(cached, listing); !reflect.DeepEqual(got, []string{"2", "4"}) {
		t.Errorf("new = %v, want [2 4]", got)
	}
	if got := NewUIDs(cached, listing[:1]); got != nil {
		t.Errorf("new = %v, want none", got)
	}
}

// The merge keeps a cached message's headers with the server's current flags and keywords, takes a
// fetched summary for a new UID, drops what the server no longer lists and drops a listed message that
// was neither cached nor fetched.
func TestMergeListingFollowsTheServer(t *testing.T) {
	seen := NewFlags(0).With(FlagSeen)
	cached := []MessageSummary{
		summaryFor(t, "1", "kept", NewFlags(0)),
		summaryFor(t, "2", "gone from the server", NewFlags(0)),
	}
	listing := []MessageState{
		stateFor(t, "1", seen, "$PPtag_work"),
		stateFor(t, "3", NewFlags(0)),
		stateFor(t, "4", NewFlags(0)),
	}
	fetched := []MessageSummary{summaryFor(t, "3", "arrived", NewFlags(0))}

	merged := MergeListing(cached, listing, fetched)
	if len(merged) != 2 {
		t.Fatalf("merged = %d messages, want 2", len(merged))
	}
	if merged[0].Subject() != "kept" || !merged[0].IsRead() || !reflect.DeepEqual(merged[0].Keywords(), []string{"$PPtag_work"}) {
		t.Errorf("cached message = %q read %v keywords %v; want its headers with the server's flags and keywords",
			merged[0].Subject(), merged[0].IsRead(), merged[0].Keywords())
	}
	if merged[1].Subject() != "arrived" {
		t.Errorf("second = %q, want the fetched arrival", merged[1].Subject())
	}
}

// WithKeywords copies, so the caller's slice cannot change the summary afterwards.
func TestWithKeywordsCopies(t *testing.T) {
	keywords := []string{"$PPtag_work"}
	m := summaryFor(t, "1", "a", NewFlags(0)).WithKeywords(keywords)
	keywords[0] = "changed"
	if got := m.Keywords(); !reflect.DeepEqual(got, []string{"$PPtag_work"}) {
		t.Errorf("keywords = %v, want the copy taken", got)
	}
}
