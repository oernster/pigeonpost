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

// SyncInboxes fetches every account's inbox folders, saves what it finds and returns the messages that
// are newly arrived (a message id not already cached) and still unread across all accounts, so the caller
// can raise a desktop notification. It does NOT suppress a first population: the caller establishes a
// baseline with an initial priming call so it does not announce an existing inbox, which lets a genuinely
// new message into a previously empty inbox still be reported. A per-account or per-folder failure is
// skipped rather than failing the pass, so one unreachable account does not silence the others.
func (s *SyncService) SyncInboxes(ctx context.Context) ([]domain.MessageSummary, error) {
	accounts, err := s.accounts.ListAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("sync: list accounts: %w", err)
	}
	rules, err := s.rules.ListRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("sync: load rules: %w", err)
	}
	var arrived []domain.MessageSummary
	for _, account := range accounts {
		folders, err := s.mail.ListFolders(ctx, account.ID())
		if err != nil {
			continue
		}
		for _, folder := range folders {
			if folder.Kind() != domain.FolderInbox {
				continue
			}
			fresh, err := s.refreshInbox(ctx, account, folder, rules)
			if err != nil {
				continue
			}
			arrived = append(arrived, fresh...)
		}
	}
	return arrived, nil
}

// refreshInbox fetches one inbox folder, applies the filter rules, saves the messages and returns the
// ones that are newly arrived (an id not already cached), excluding only those a filter rule marked read
// on arrival. Read state otherwise does not gate the result, so a message another client already marked
// read still counts. A message into an empty folder counts as new; the caller's baseline priming keeps an
// initial sync from being announced.
func (s *SyncService) refreshInbox(ctx context.Context, account domain.Account, folder domain.Folder, rules []domain.Rule) ([]domain.MessageSummary, error) {
	existing, err := s.mail.ListMessages(ctx, folder.ID())
	if err != nil {
		return nil, err
	}
	fetched, err := s.source.FetchMessages(ctx, account, folder)
	if err != nil {
		return nil, err
	}
	// Record each message's read state as the server reports it, before local rules run. A message
	// another client (Thunderbird, a phone) has already marked read is still a new arrival worth
	// announcing; only a message a filter rule marks read on arrival should be silenced.
	serverUnread := make(map[string]bool, len(fetched))
	for _, m := range fetched {
		serverUnread[m.ID()] = !m.IsRead()
	}
	messages := domain.ApplyRules(fetched, rules)
	if account.Protocol() == domain.ProtocolPOP3 {
		messages = carryOverFlags(existing, messages)
	}
	if err := s.mail.SaveMessages(ctx, folder.ID(), messages); err != nil {
		return nil, err
	}
	known := make(map[string]struct{}, len(existing))
	for _, m := range existing {
		known[m.ID()] = struct{}{}
	}
	var fresh []domain.MessageSummary
	for _, m := range messages {
		if _, seen := known[m.ID()]; seen {
			continue
		}
		// Skip only a message a filter rule silenced by marking it read on arrival (unread on the server,
		// read after the rule); announce every other new message regardless of its read state.
		if serverUnread[m.ID()] && m.IsRead() {
			continue
		}
		fresh = append(fresh, m)
	}
	return fresh, nil
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
	return carryOverFlags(existing, incoming), nil
}

// carryOverFlags copies each still-present message's stored flags onto its freshly fetched summary,
// matched by the stable message id, so a local read or star mark survives a sync that reports no
// server-side flags. A newly arrived message (an id not among the existing) keeps its fetched flags.
func carryOverFlags(existing, incoming []domain.MessageSummary) []domain.MessageSummary {
	flagsByID := make(map[string]domain.Flags, len(existing))
	for _, m := range existing {
		flagsByID[m.ID()] = m.Flags()
	}
	out := make([]domain.MessageSummary, len(incoming))
	for i, m := range incoming {
		if flags, ok := flagsByID[m.ID()]; ok {
			out[i] = m.WithFlags(flags)
		} else {
			out[i] = m
		}
	}
	return out
}
