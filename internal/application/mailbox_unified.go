package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/oernster/pigeonpost/internal/domain"
)

// UnifiedMailboxService is the read-side aggregation behind the unified mailbox view: one combined,
// date-ordered list drawn from every account's inbox. It reads only the local cache (through the same
// MailStore the per-folder views use), so the unified list needs no storage change and never blocks on
// the network; the merge and its ordering policy live here, in gated application code.
type UnifiedMailboxService struct {
	accounts AccountStore
	mail     MailStore
}

// NewUnifiedMailboxService constructs the service with its injected stores.
func NewUnifiedMailboxService(accounts AccountStore, mail MailStore) *UnifiedMailboxService {
	return &UnifiedMailboxService{accounts: accounts, mail: mail}
}

// UnifiedMessage pairs a cached message summary with the id of the account whose inbox holds it, so the
// combined list can label each row with its account.
type UnifiedMessage struct {
	Summary   domain.MessageSummary
	AccountID string
}

// inboxes returns every account's inbox folders, in account order.
func (s *UnifiedMailboxService) inboxes(ctx context.Context) ([]domain.Folder, error) {
	accounts, err := s.accounts.ListAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("unified: list accounts: %w", err)
	}
	var inboxes []domain.Folder
	for _, account := range accounts {
		folders, err := s.mail.ListFolders(ctx, account.ID())
		if err != nil {
			return nil, fmt.Errorf("unified: list folders for account %q: %w", account.ID(), err)
		}
		for _, folder := range folders {
			if folder.Kind() == domain.FolderInbox {
				inboxes = append(inboxes, folder)
			}
		}
	}
	return inboxes, nil
}

// Messages returns every inbox's cached messages merged into one list in the newest-first (date, id)
// order. The conversation view needs the whole combined set to thread it, exactly as a single folder's
// conversation view loads the whole folder.
func (s *UnifiedMailboxService) Messages(ctx context.Context) ([]UnifiedMessage, error) {
	inboxes, err := s.inboxes(ctx)
	if err != nil {
		return nil, err
	}
	var merged []UnifiedMessage
	for _, folder := range inboxes {
		messages, err := s.mail.ListMessages(ctx, folder.ID())
		if err != nil {
			return nil, fmt.Errorf("unified: list messages for folder %q: %w", folder.ID(), err)
		}
		for _, message := range messages {
			merged = append(merged, UnifiedMessage{Summary: message, AccountID: folder.AccountID()})
		}
	}
	sortUnified(merged, false)
	return merged, nil
}

// MessagesPage returns one keyset page of the combined inbox list. It fans the same cursor out to each
// inbox folder (each returns its first limit rows strictly after the cursor, in the store's (date, id)
// order), merges the results in that same order and keeps the first limit rows. Every kept row is
// within some folder's returned page, so the walk never skips or repeats a row; the cursor mechanics
// are exactly the per-folder ones the flat folder view already uses.
func (s *UnifiedMailboxService) MessagesPage(ctx context.Context, hasCursor bool, cursorDateMs int64, cursorID string, limit int, ascending bool) ([]UnifiedMessage, error) {
	inboxes, err := s.inboxes(ctx)
	if err != nil {
		return nil, err
	}
	var merged []UnifiedMessage
	for _, folder := range inboxes {
		page, err := s.mail.ListMessagesPage(ctx, folder.ID(), hasCursor, cursorDateMs, cursorID, limit, ascending)
		if err != nil {
			return nil, fmt.Errorf("unified: page messages for folder %q: %w", folder.ID(), err)
		}
		for _, message := range page {
			merged = append(merged, UnifiedMessage{Summary: message, AccountID: folder.AccountID()})
		}
	}
	sortUnified(merged, ascending)
	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}
	return merged, nil
}

// sortUnified orders the merged rows by (date, id) in the store's paging order: newest first with id
// descending on a shared timestamp, or the exact reverse when ascending. Matching the store's order
// keeps the fanned-out cursor walk total, so no row is skipped or repeated across pages.
func sortUnified(messages []UnifiedMessage, ascending bool) {
	sort.SliceStable(messages, func(i, j int) bool {
		a, b := messages[i].Summary, messages[j].Summary
		dateA, dateB := a.Date().UnixMilli(), b.Date().UnixMilli()
		if dateA != dateB {
			if ascending {
				return dateA < dateB
			}
			return dateA > dateB
		}
		if ascending {
			return a.ID() < b.ID()
		}
		return a.ID() > b.ID()
	})
}
