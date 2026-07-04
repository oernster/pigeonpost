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
		messages, err = s.preserveFlags(ctx, account, folder, messages)
		if err != nil {
			return fmt.Errorf("sync: preserve flags for %q: %w", folder.Path(), err)
		}
		if err := s.mail.SaveMessages(ctx, folder.ID(), messages); err != nil {
			return fmt.Errorf("sync: save messages for %q: %w", folder.Path(), err)
		}
	}
	return nil
}

// SyncFolder refreshes a single folder's message summaries from the server, applies the filter rules
// and writes them into the local store. It is the light path taken when a folder is opened or on the
// periodic refresh, avoiding a full account sync of every mailbox.
func (s *SyncService) SyncFolder(ctx context.Context, folderID string) error {
	folder, err := s.mail.GetFolder(ctx, folderID)
	if err != nil {
		return fmt.Errorf("sync: load folder %q: %w", folderID, err)
	}
	account, err := s.accounts.GetAccount(ctx, folder.AccountID())
	if err != nil {
		return fmt.Errorf("sync: load account %q: %w", folder.AccountID(), err)
	}
	rules, err := s.rules.ListRules(ctx)
	if err != nil {
		return fmt.Errorf("sync: load rules: %w", err)
	}
	messages, err := s.source.FetchMessages(ctx, account, folder)
	if err != nil {
		return fmt.Errorf("sync: fetch messages for %q: %w", folder.Path(), err)
	}
	messages = domain.ApplyRules(messages, rules)
	messages, err = s.preserveFlags(ctx, account, folder, messages)
	if err != nil {
		return fmt.Errorf("sync: preserve flags for %q: %w", folder.Path(), err)
	}
	if err := s.mail.SaveMessages(ctx, folder.ID(), messages); err != nil {
		return fmt.Errorf("sync: save messages for %q: %w", folder.Path(), err)
	}
	return nil
}

// preserveFlags carries a POP3 message's local read and starred state across a sync. POP3 has no
// server-side flags, so a fetch always reports every message as unread; without this, marking a message
// read would be undone on the next sync. Flags are matched by the stable message id, so messages still
// present keep their local flags while newly arrived messages keep the fetched state. IMAP mirrors its
// flags from the server, so its messages are returned unchanged.
func (s *SyncService) preserveFlags(ctx context.Context, account domain.Account, folder domain.Folder, incoming []domain.MessageSummary) ([]domain.MessageSummary, error) {
	if account.Protocol() != domain.ProtocolPOP3 {
		return incoming, nil
	}
	existing, err := s.mail.ListMessages(ctx, folder.ID())
	if err != nil {
		return nil, err
	}
	flagsByID := make(map[string]domain.Flags, len(existing))
	for _, m := range existing {
		flagsByID[m.ID()] = m.Flags()
	}
	preserved := make([]domain.MessageSummary, len(incoming))
	for i, m := range incoming {
		if flags, ok := flagsByID[m.ID()]; ok {
			preserved[i] = m.WithFlags(flags)
		} else {
			preserved[i] = m
		}
	}
	return preserved, nil
}
