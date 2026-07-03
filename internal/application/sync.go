package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// SyncService pulls folders and message summaries from a remote source and persists them into the
// local store, applying the user's filter rules to messages as they arrive.
type SyncService struct {
	accounts AccountStore
	mail     MailStore
	source   MailSource
	rules    RuleStore
}

// NewSyncService constructs the service with its injected dependencies.
func NewSyncService(accounts AccountStore, mail MailStore, source MailSource, rules RuleStore) *SyncService {
	return &SyncService{accounts: accounts, mail: mail, source: source, rules: rules}
}

// SyncAccount fetches the folder list for an account, then each folder's message summaries, applies
// the user's filter rules to them and writes them into the local store. It stops at the first error
// so a partially failed sync does not silently look complete.
func (s *SyncService) SyncAccount(ctx context.Context, accountID string) error {
	account, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("sync: load account %q: %w", accountID, err)
	}

	rules, err := s.rules.ListRules(ctx)
	if err != nil {
		return fmt.Errorf("sync: load rules: %w", err)
	}

	folders, err := s.source.FetchFolders(ctx, account)
	if err != nil {
		return fmt.Errorf("sync: fetch folders: %w", err)
	}
	if err := s.mail.SaveFolders(ctx, accountID, folders); err != nil {
		return fmt.Errorf("sync: save folders: %w", err)
	}

	for _, folder := range folders {
		messages, err := s.source.FetchMessages(ctx, account, folder)
		if err != nil {
			return fmt.Errorf("sync: fetch messages for %q: %w", folder.Path(), err)
		}
		// Filter rules mark-read or flag matching messages as they arrive. The actions only set
		// flags, so re-applying on every sync is stable.
		messages = domain.ApplyRules(messages, rules)
		if err := s.mail.SaveMessages(ctx, folder.ID(), messages); err != nil {
			return fmt.Errorf("sync: save messages for %q: %w", folder.Path(), err)
		}
	}
	return nil
}
