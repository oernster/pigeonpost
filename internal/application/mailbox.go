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

// MarkRead sets or clears the read (Seen) state of a message in the local cache.
func (s *MailboxService) MarkRead(ctx context.Context, messageID string, read bool) error {
	if err := s.mail.SetSeen(ctx, messageID, read); err != nil {
		return fmt.Errorf("mark message %q read=%t: %w", messageID, read, err)
	}
	return nil
}
