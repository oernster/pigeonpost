package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

func testDatedMessage(t *testing.T, id, folderID string, date time.Time) domain.MessageSummary {
	t.Helper()
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: folderID, UID: "1", Date: date, Size: 10, Flags: domain.NewFlags(0),
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	return msg
}

// newUnifiedFixture builds two accounts, each with an inbox, plus a sent folder that must never appear
// in the unified list. Dates: m1(d1) and m3(d3) and mB(d4) in a1's inbox; m2(d2) and mA(d4) in a2's
// inbox, so the merged order exercises both the cross-account interleave and the (date, id) tie-break.
func newUnifiedFixture(t *testing.T) (*fakeAccountStore, *fakeMailStore, *UnifiedMailboxService) {
	t.Helper()
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	accounts.accounts["a2"] = testAccount(t, "a2")

	mail := newFakeMailStore()
	sent, err := domain.NewFolder("fs", "a1", "Sent", domain.FolderSent, 0, 0)
	if err != nil {
		t.Fatalf("build sent folder: %v", err)
	}
	mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX"), sent}
	mail.folders["a2"] = []domain.Folder{testFolder(t, "f2", "a2", "INBOX")}

	d1 := time.Date(2026, time.July, 1, 8, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, time.July, 2, 8, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, time.July, 3, 8, 0, 0, 0, time.UTC)
	d4 := time.Date(2026, time.July, 4, 8, 0, 0, 0, time.UTC)
	d5 := time.Date(2026, time.July, 5, 8, 0, 0, 0, time.UTC)
	mail.messages["f1"] = []domain.MessageSummary{
		testDatedMessage(t, "m1", "f1", d1),
		testDatedMessage(t, "m3", "f1", d3),
		testDatedMessage(t, "mB", "f1", d4),
	}
	mail.messages["f2"] = []domain.MessageSummary{
		testDatedMessage(t, "m2", "f2", d2),
		testDatedMessage(t, "mA", "f2", d4),
	}
	// The sent folder holds the newest message of all; it must still never surface in the unified list.
	mail.messages["fs"] = []domain.MessageSummary{testDatedMessage(t, "mS", "fs", d5)}

	return accounts, mail, NewUnifiedMailboxService(accounts, mail, fakeClock{now: time.Unix(0, 0).UTC()})
}

func unifiedIDs(merged []UnifiedMessage) []string {
	out := make([]string, len(merged))
	for i, m := range merged {
		out[i] = m.Summary.ID()
	}
	return out
}

func assertIDs(t *testing.T, got []UnifiedMessage, want ...string) {
	t.Helper()
	ids := unifiedIDs(got)
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v, want %v", ids, want)
		}
	}
}

func TestUnifiedMessagesMergesInboxesNewestFirst(t *testing.T) {
	_, _, service := newUnifiedFixture(t)

	merged, err := service.Messages(context.Background())
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	// Newest first; mB and mA share d4, so the id tie-break (descending) puts mB before mA; the sent
	// folder's mS is absent.
	assertIDs(t, merged, "mB", "mA", "m3", "m2", "m1")

	wantAccounts := map[string]string{"mB": "a1", "mA": "a2", "m3": "a1", "m2": "a2", "m1": "a1"}
	for _, m := range merged {
		if m.AccountID != wantAccounts[m.Summary.ID()] {
			t.Errorf("message %q stamped with account %q, want %q", m.Summary.ID(), m.AccountID, wantAccounts[m.Summary.ID()])
		}
	}
}

func TestUnifiedMessagesPageWalksWithoutSkipOrRepeat(t *testing.T) {
	_, _, service := newUnifiedFixture(t)
	ctx := context.Background()

	first, err := service.MessagesPage(ctx, false, 0, "", 2, false)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	assertIDs(t, first, "mB", "mA")

	last := first[len(first)-1].Summary
	second, err := service.MessagesPage(ctx, true, last.Date().UnixMilli(), last.ID(), 2, false)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	assertIDs(t, second, "m3", "m2")

	last = second[len(second)-1].Summary
	third, err := service.MessagesPage(ctx, true, last.Date().UnixMilli(), last.ID(), 2, false)
	if err != nil {
		t.Fatalf("third page: %v", err)
	}
	assertIDs(t, third, "m1")

	last = third[len(third)-1].Summary
	fourth, err := service.MessagesPage(ctx, true, last.Date().UnixMilli(), last.ID(), 2, false)
	if err != nil {
		t.Fatalf("fourth page: %v", err)
	}
	if len(fourth) != 0 {
		t.Fatalf("fourth page = %v, want empty", unifiedIDs(fourth))
	}
}

func TestUnifiedMessagesPageAscending(t *testing.T) {
	_, _, service := newUnifiedFixture(t)

	// A limit beyond the total returns everything, oldest first with the id tie-break reversed.
	page, err := service.MessagesPage(context.Background(), false, 0, "", 10, true)
	if err != nil {
		t.Fatalf("ascending page: %v", err)
	}
	assertIDs(t, page, "m1", "m2", "m3", "mA", "mB")
}

func TestUnifiedMessagesErrors(t *testing.T) {
	ctx := context.Background()

	accounts, _, service := newUnifiedFixture(t)
	accounts.listErr = errors.New("boom")
	if _, err := service.Messages(ctx); err == nil {
		t.Error("expected an account-list error from Messages")
	}
	if _, err := service.MessagesPage(ctx, false, 0, "", 2, false); err == nil {
		t.Error("expected an account-list error from MessagesPage")
	}

	_, mail, service := newUnifiedFixture(t)
	mail.listFoldersErr = errors.New("boom")
	if _, err := service.Messages(ctx); err == nil {
		t.Error("expected a folder-list error from Messages")
	}

	_, mail, service = newUnifiedFixture(t)
	mail.listMessagesErr = errors.New("boom")
	if _, err := service.Messages(ctx); err == nil {
		t.Error("expected a message-list error from Messages")
	}

	_, mail, service = newUnifiedFixture(t)
	mail.listPageErr = errors.New("boom")
	if _, err := service.MessagesPage(ctx, false, 0, "", 2, false); err == nil {
		t.Error("expected a page error from MessagesPage")
	}
}
