package imap

import (
	"context"
	"strings"
	"testing"
)

// containsAny reports whether line holds any of the given fragments, compared without case.
func containsAny(line string, fragments ...string) bool {
	upper := strings.ToUpper(line)
	for _, fragment := range fragments {
		if strings.Contains(upper, strings.ToUpper(fragment)) {
			return true
		}
	}
	return false
}

// The listing asks for UIDs and flags only (no envelope, no body structure) and maps each message's flags
// as a full fetch does: the scripted server's one message, UID 7, reported \Seen.
func TestFetchListingAsksForFlagsOnly(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands})
	listing, validity, err := fakeSource().FetchListing(context.Background(), fakeAccount(t, host, port), fakeFolder(t))
	if err != nil {
		t.Fatalf("FetchListing: %v", err)
	}
	if len(listing) != 1 || listing[0].UID() != "7" || !listing[0].Flags().IsSeen() || validity != defaultUIDValidity {
		t.Fatalf("listing %+v under %d, want UID 7 seen under %d", listing, validity, defaultUIDValidity)
	}
	fetches := commands.matching("FETCH")
	if len(fetches) != 1 || containsAny(fetches[0], "ENVELOPE", "BODYSTRUCTURE") {
		t.Errorf("fetches = %v, want one asking for UID and FLAGS only", fetches)
	}
}

// An empty folder lists nothing yet still reports its UIDVALIDITY.
func TestFetchListingOfAnEmptyFolderReportsItsValidity(t *testing.T) {
	t.Parallel()
	host, port := listenFake(t, script{empty: true, uidValidity: 9})
	listing, validity, err := fakeSource().FetchListing(context.Background(), fakeAccount(t, host, port), fakeFolder(t))
	if err != nil || len(listing) != 0 || validity != 9 {
		t.Errorf("listing %v under %d, err %v; want nothing under 9", listing, validity, err)
	}
}

// A refused sign-in reaches the caller.
func TestFetchListingReportsARefusedSignIn(t *testing.T) {
	t.Parallel()
	host, port := listenFake(t, script{loginRefusal: "no"})
	if _, _, err := fakeSource().FetchListing(context.Background(), fakeAccount(t, host, port), fakeFolder(t)); err == nil {
		t.Error("a refused sign-in answered no error")
	}
}

// The by-UID fetch asks by UID and builds the summaries it is given; even with an unreadable body
// structure it falls back and still answers them.
func TestFetchMessagesByUIDFetchesOnlyThoseUIDs(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands, bodyStructure: unreadableBodyStructure})
	messages, validity, err := fakeSource().FetchMessagesByUID(context.Background(), fakeAccount(t, host, port), fakeFolder(t), []string{"7"})
	if err != nil {
		t.Fatalf("FetchMessagesByUID: %v", err)
	}
	if len(messages) != 1 || messages[0].UID() != "7" || validity != defaultUIDValidity {
		t.Fatalf("messages %d under %d, want UID 7 under %d", len(messages), validity, defaultUIDValidity)
	}
	for _, fetch := range commands.matching("UID FETCH") {
		if !containsAny(fetch, "UID FETCH 7 ") {
			t.Errorf("fetch %q, want it to ask for UID 7 alone", fetch)
		}
	}
}

// A malformed UID fails the fetch before anything is sent.
func TestFetchMessagesByUIDRefusesAMalformedUID(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands})
	if _, _, err := fakeSource().FetchMessagesByUID(context.Background(), fakeAccount(t, host, port), fakeFolder(t), []string{"x"}); err == nil {
		t.Error("a malformed UID answered no error")
	}
	if logins := len(commands.matching("LOGIN")); logins != 0 {
		t.Errorf("logins = %d, want none for a malformed UID", logins)
	}
}
