package application

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// settledValidity is the UIDVALIDITY the incremental tests' folder was last settled under.
const settledValidity uint32 = 5

// newIncrementalFixture caches messages 1 and 2 in folder f1 (settled under settledValidity) while the
// server lists 1 (now read) and 3 (new), whose summary it holds.
func newIncrementalFixture(t *testing.T) (*fakeMailStore, *fakeMailSource, *SyncService, domain.Account, domain.Folder) {
	t.Helper()
	accounts, mail, source, _, svc := newSyncFixture(t)
	mail.messages["f1"] = []domain.MessageSummary{messageWithUID(t, "m1", "f1", "1"), messageWithUID(t, "m2", "f1", "2")}
	mail.validity = map[string]uint32{"f1": settledValidity}
	source.validity = map[string]uint32{"f1": settledValidity}
	seen := domain.NewFlags(0).With(domain.FlagSeen)
	source.listing = map[string][]domain.MessageState{"f1": {
		mustState(t, "1", seen, "$PPtag_work"),
		mustState(t, "3", domain.NewFlags(0)),
	}}
	source.messagesByFolder["f1"] = []domain.MessageSummary{messageWithUID(t, "m3", "f1", "3")}
	return mail, source, svc, accounts.accounts["a1"], source.folders[0]
}

func mustState(t *testing.T, uid string, flags domain.Flags, keywords ...string) domain.MessageState {
	t.Helper()
	state, err := domain.NewMessageState(uid, flags, keywords)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	return state
}

func uidsOf(messages []domain.MessageSummary) []string {
	uids := make([]string, 0, len(messages))
	for _, m := range messages {
		uids = append(uids, m.UID())
	}
	return uids
}

// A folder settled under the numbering the server still reports is refreshed from its listing: only the
// new UID is fetched, the cached message takes the server's flags and keywords and the one the server no
// longer lists drops out. Nothing is fetched in full.
func TestFetchFolderRefreshesASettledFolderFromItsListing(t *testing.T) {
	_, source, svc, account, folder := newIncrementalFixture(t)
	fetch, err := svc.fetchFolder(context.Background(), account, folder)
	if err != nil {
		t.Fatalf("fetchFolder: %v", err)
	}
	if got := uidsOf(fetch.messages); !reflect.DeepEqual(got, []string{"1", "3"}) {
		t.Errorf("messages = %v, want [1 3]", got)
	}
	if !fetch.messages[0].IsRead() || !reflect.DeepEqual(fetch.messages[0].Keywords(), []string{"$PPtag_work"}) {
		t.Error("the cached message did not take the listing's flags and keywords")
	}
	if !reflect.DeepEqual(source.byUIDRequested, [][]string{{"3"}}) || source.fullFetches != 0 {
		t.Errorf("asked for %v with %d full fetches; want only [3] and none", source.byUIDRequested, source.fullFetches)
	}
	if fetch.renumbered || fetch.validity != settledValidity {
		t.Errorf("validity %d renumbered %v, want %d unchanged", fetch.validity, fetch.renumbered, settledValidity)
	}
}

// With nothing new on the server no summary is fetched at all.
func TestFetchFolderFetchesNoSummaryWhenNothingIsNew(t *testing.T) {
	_, source, svc, account, folder := newIncrementalFixture(t)
	source.listing["f1"] = []domain.MessageState{mustState(t, "1", domain.NewFlags(0))}
	if _, err := svc.fetchFolder(context.Background(), account, folder); err != nil {
		t.Fatalf("fetchFolder: %v", err)
	}
	if len(source.byUIDRequested) != 0 || source.fullFetches != 0 {
		t.Errorf("asked for %v with %d full fetches; want nothing", source.byUIDRequested, source.fullFetches)
	}
}

// The cached headers are trusted only under the numbering they were cached with: a folder never settled,
// one renumbered since, one renumbered between the two reads and one with no UIDVALIDITY (POP3) are all
// fetched in full.
func TestFetchFolderFetchesInFullWhereTheCacheCannotBeTrusted(t *testing.T) {
	for name, arrange := range map[string]func(*fakeMailStore, *fakeMailSource){
		"never settled":                func(m *fakeMailStore, _ *fakeMailSource) { m.validity = nil },
		"renumbered since":             func(_ *fakeMailStore, s *fakeMailSource) { s.validity["f1"] = settledValidity + 1 },
		"renumbered between the reads": func(_ *fakeMailStore, s *fakeMailSource) { s.byUIDValidity = settledValidity + 1 },
		"no UIDVALIDITY (POP3 shape)":  func(_ *fakeMailStore, s *fakeMailSource) { s.listing = nil },
	} {
		mail, source, svc, account, folder := newIncrementalFixture(t)
		arrange(mail, source)
		if _, err := svc.fetchFolder(context.Background(), account, folder); err != nil {
			t.Fatalf("%s: fetchFolder: %v", name, err)
		}
		if source.fullFetches != 1 {
			t.Errorf("%s: full fetches = %d, want 1", name, source.fullFetches)
		}
	}
}

// A failure on the listing path fails the fetch rather than quietly falling back, so it reaches the log.
func TestFetchFolderReportsAFailureOnTheListingPath(t *testing.T) {
	errBoomListing := errors.New("listing refused")
	for name, arrange := range map[string]func(*fakeMailStore, *fakeMailSource){
		"listing":         func(_ *fakeMailStore, s *fakeMailSource) { s.listingErr = errBoomListing },
		"stored validity": func(m *fakeMailStore, _ *fakeMailSource) { m.validityErr = errBoomListing },
		"cached messages": func(m *fakeMailStore, _ *fakeMailSource) { m.listMessagesErr = errBoomListing },
		"new summaries":   func(_ *fakeMailStore, s *fakeMailSource) { s.byUIDErr = errBoomListing },
	} {
		mail, source, svc, account, folder := newIncrementalFixture(t)
		arrange(mail, source)
		if _, err := svc.fetchFolder(context.Background(), account, folder); !errors.Is(err, errBoomListing) {
			t.Errorf("%s: error = %v, want the failure", name, err)
		}
		if source.fullFetches != 0 {
			t.Errorf("%s: fell back to a full fetch", name)
		}
	}
}
