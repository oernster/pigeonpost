package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// MailboxService is the use-case boundary for reading cached folders and messages. It reads from the
// local store only, so it never blocks on the network.
type MailboxService struct {
	mail MailStore
}

// NewMailboxService constructs the service with its injected store.
func NewMailboxService(mail MailStore) *MailboxService {
	return &MailboxService{mail: mail}
}

// Folders returns the cached folders for an account.
func (s *MailboxService) Folders(ctx context.Context, accountID string) ([]domain.Folder, error) {
	folders, err := s.mail.ListFolders(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list folders for account %q: %w", accountID, err)
	}
	return folders, nil
}

// Messages returns the cached message summaries for a folder.
func (s *MailboxService) Messages(ctx context.Context, folderID string) ([]domain.MessageSummary, error) {
	messages, err := s.mail.ListMessages(ctx, folderID)
	if err != nil {
		return nil, fmt.Errorf("list messages for folder %q: %w", folderID, err)
	}
	return messages, nil
}

// UnreadTotals carries the per-account unread message counts and their sum across all accounts.
type UnreadTotals struct {
	Total     int
	ByAccount map[string]int
}

// UnreadCounts returns the unread message count for each account and the total across all accounts,
// computed from the local cache. The per-account map never contains a nil value; an account with no
// unread messages is simply absent.
func (s *MailboxService) UnreadCounts(ctx context.Context) (UnreadTotals, error) {
	byAccount, err := s.mail.UnreadByAccount(ctx)
	if err != nil {
		return UnreadTotals{}, fmt.Errorf("unread counts: %w", err)
	}
	total := 0
	for _, n := range byAccount {
		total += n
	}
	return UnreadTotals{Total: total, ByAccount: byAccount}, nil
}

// Search returns cached messages matching a free-text query, most relevant first.
func (s *MailboxService) Search(ctx context.Context, query string) ([]domain.MessageSummary, error) {
	messages, err := s.mail.SearchMessages(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search messages for %q: %w", query, err)
	}
	return messages, nil
}
