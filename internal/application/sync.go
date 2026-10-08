package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// TagSyncer replays pending tag changes to the server and reconciles local tag assignments against the tag
// keywords the server reports on freshly fetched messages. The sync drives it so a tag assigned on another
// device (or made locally while offline) is brought into step on the next sync. It is best-effort: the sync
// ignores the errors it returns so a tag-sync hiccup never fails a mail sync.
type TagSyncer interface {
	FlushPending(ctx context.Context) error
	ReconcileFetched(ctx context.Context, messages []domain.MessageSummary) error
}

// FlagSyncer replays pending flag changes (read, starred, answered, forwarded) to the server and guards
// them against a fetch that still reports the old value, returning the fetched summaries with each
// unconfirmed local intent overlaid. The sync drives it so a flag changed locally (or one the server
// applied lazily or dropped) is neither lost to a stale fetch nor left unsynced. Like the tag syncer it
// is best-effort at the sync's call sites: a hiccup never fails a mail sync.
type FlagSyncer interface {
	FlushPending(ctx context.Context) error
	ReconcileFetched(ctx context.Context, messages []domain.MessageSummary) ([]domain.MessageSummary, error)
}

// SyncService pulls folders and message summaries from a remote source and persists them into the
// local store, applying the user's filter rules to messages as they arrive.
type SyncService struct {
	accounts AccountStore
	mail     MailStore
	source   MailSource
	rules    RuleStore
	tags     TagSyncer
	flags    FlagSyncer
	ruleExec *RuleExecutor
}

// NewSyncService constructs the service with its injected dependencies.
func NewSyncService(accounts AccountStore, mail MailStore, source MailSource, rules RuleStore, tags TagSyncer,
	flags FlagSyncer, ruleExec *RuleExecutor) *SyncService {
	return &SyncService{accounts: accounts, mail: mail, source: source, rules: rules, tags: tags, flags: flags,
		ruleExec: ruleExec}
}

// reconcileTags aligns local tag assignments with the server keywords on the fetched messages, for an IMAP
// account only: a POP3 message carries no keywords, so reconciling it would read as every tag having been
// removed. It is best-effort, so its error is deliberately ignored by the caller.
func (s *SyncService) reconcileTags(ctx context.Context, account domain.Account, messages []domain.MessageSummary) {
	if account.Protocol() == domain.ProtocolPOP3 {
		return
	}
	_ = s.tags.ReconcileFetched(ctx, messages)
}

// reconcileFlags overlays unconfirmed local flag changes onto the fetched messages, for an IMAP account
// only: a POP3 account records no pending flag ops (its flags are purely local and preserveFlags carries
// them whole). It is best-effort: on a reconcile failure the fetched messages are saved as reported and
// the intents stay in place for the next pass to guard.
func (s *SyncService) reconcileFlags(ctx context.Context, account domain.Account, messages []domain.MessageSummary) []domain.MessageSummary {
	if account.Protocol() == domain.ProtocolPOP3 {
		return messages
	}
	out, err := s.flags.ReconcileFetched(ctx, messages)
	if err != nil {
		return messages
	}
	return out
}

// SyncAccount fetches the folder list for an account, then each folder's message summaries, applies
// the user's filter rules to them and writes them into the local store. It stops at the first error
// so a partially failed sync does not silently look complete.
func (s *SyncService) SyncAccount(ctx context.Context, accountID string) error {
	account, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("sync: load account %q: %w", accountID, err)
	}
	// One connection for the whole sync: the folder list, the flag replay and every folder's fetch share
	// it, where each used to log in on its own (about 51 logins for StartMail's 49 folders).
	ctx, end := s.source.BeginSession(ctx, account)
	defer end()

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

	// Replay any tag and flag changes that have not yet reached the server before reading folders back,
	// so the fetch reflects them and the reconciles can confirm them. Best-effort: a failure never fails
	// the sync.
	_ = s.tags.FlushPending(ctx)
	_ = s.flags.FlushPending(ctx)

	for _, folder := range folders {
		fetch, err := s.fetchFolder(ctx, account, folder)
		if err != nil {
			return fmt.Errorf("sync: fetch messages for %q: %w", folder.Path(), err)
		}
		// Align local tag assignments with the server keywords on the fetched messages, before the rules
		// run, so the reconcile sees the keywords exactly as the server reported them.
		s.reconcileTags(ctx, account, fetch.messages)
		// Filter rules act on arriving inbox mail: they set flags, move messages into another folder
		// or destroy them outright. A rule that could not be carried out is reported after the save
		// below, so what did work is still stored.
		messages, marks, ruleErr := s.applyRules(ctx, account, folder, fetch, rules)
		messages, err = s.preserveFlags(ctx, account, folder, messages)
		if err != nil {
			return fmt.Errorf("sync: preserve flags for %q: %w", folder.Path(), err)
		}
		// Overlay unconfirmed local flag changes last, so nothing between here and the save can regress
		// a flag the user changed but the server has not yet confirmed.
		messages = s.reconcileFlags(ctx, account, messages)
		if err := s.mail.SaveMessages(ctx, folder.ID(), messages); err != nil {
			return fmt.Errorf("sync: save messages for %q: %w", folder.Path(), err)
		}
		// Only now that the folder's contents are cached are its baseline, UIDVALIDITY and the rules'
		// marks established.
		if err := s.settleFolder(ctx, folder, fetch.validity, marks); err != nil {
			return fmt.Errorf("sync: settle %q: %w", folder.Path(), err)
		}
		if ruleErr != nil {
			return fmt.Errorf("sync: apply rules to %q: %w", folder.Path(), ruleErr)
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
	// Replay unsynced tag and flag changes before the fetch, so the fetch reflects them and the
	// reconciles below can confirm them. Best-effort: a failure never fails the sync.
	_ = s.tags.FlushPending(ctx)
	_ = s.flags.FlushPending(ctx)
	fetch, err := s.fetchFolder(ctx, account, folder)
	if err != nil {
		return fmt.Errorf("sync: fetch messages for %q: %w", folder.Path(), err)
	}
	s.reconcileTags(ctx, account, fetch.messages)
	messages, marks, ruleErr := s.applyRules(ctx, account, folder, fetch, rules)
	messages, err = s.preserveFlags(ctx, account, folder, messages)
	if err != nil {
		return fmt.Errorf("sync: preserve flags for %q: %w", folder.Path(), err)
	}
	// Overlay unconfirmed local flag changes last, so nothing between here and the save can regress a
	// flag the user changed but the server has not yet confirmed.
	messages = s.reconcileFlags(ctx, account, messages)
	if err := s.mail.SaveMessages(ctx, folder.ID(), messages); err != nil {
		return fmt.Errorf("sync: save messages for %q: %w", folder.Path(), err)
	}
	// Only now that the folder's contents are cached are its baseline, UIDVALIDITY and the rules' marks
	// established.
	if err := s.settleFolder(ctx, folder, fetch.validity, marks); err != nil {
		return fmt.Errorf("sync: settle %q: %w", folder.Path(), err)
	}
	if ruleErr != nil {
		return fmt.Errorf("sync: apply rules to %q: %w", folder.Path(), ruleErr)
	}
	return nil
}

// SyncInboxes fetches every account's inbox folders, saves what it finds and returns the messages that
// are newly arrived (a message id not already cached) across all accounts, less any a filter rule marked
// read on arrival (see refreshInbox), so the caller can raise a desktop notification. An inbox never
// baselined is a first population and announces nothing (see folderFetch.knownFor), so a newly added
// account's existing mail is not reported whichever pass reaches it first; an inbox that has been
// baselined and then emptied still reports its next genuine arrival. A per-account or per-folder failure is
// skipped rather than failing the pass, so one unreachable account does not silence the others; every
// one skipped is joined into the error, beside the arrivals the rest of the pass found, so a caller can
// record a failure that would otherwise go unseen for as long as the account stays broken.
func (s *SyncService) SyncInboxes(ctx context.Context) ([]domain.MessageSummary, error) {
	return s.syncInboxesOf(ctx, func(domain.Account) bool { return true })
}

// SyncAccountInbox is SyncInboxes for the one account accountID names: what an IDLE push for that account
// calls. A push says only that account's inbox changed; syncing every account on it would connect to every
// server each time one of them announced mail. An id that names no account syncs nothing.
func (s *SyncService) SyncAccountInbox(ctx context.Context, accountID string) ([]domain.MessageSummary, error) {
	return s.syncInboxesOf(ctx, func(account domain.Account) bool { return account.ID() == accountID })
}

// syncInboxesOf runs one inbox pass over the accounts include accepts, as SyncInboxes describes.
func (s *SyncService) syncInboxesOf(ctx context.Context, include func(domain.Account) bool) ([]domain.MessageSummary, error) {
	accounts, err := s.accounts.ListAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("sync: list accounts: %w", err)
	}
	rules, err := s.rules.ListRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("sync: load rules: %w", err)
	}
	// Replay unsynced tag and flag changes once for this pass before reading the inboxes. Best-effort: a
	// replay that fails does not stop the pass; it is reported with the pass's other skips.
	var skipped []error
	if err := s.tags.FlushPending(ctx); err != nil {
		skipped = append(skipped, err)
	}
	if err := s.flags.FlushPending(ctx); err != nil {
		skipped = append(skipped, err)
	}
	var arrived []domain.MessageSummary
	for _, account := range accounts {
		if !include(account) {
			continue
		}
		folders, err := s.mail.ListFolders(ctx, account.ID())
		if err != nil {
			skipped = append(skipped, fmt.Errorf("sync: inbox of %s: %w", account.ID(), err))
			continue
		}
		for _, folder := range folders {
			if folder.Kind() != domain.FolderInbox {
				continue
			}
			fresh, err := s.refreshInbox(ctx, account, folder, rules)
			if err != nil {
				skipped = append(skipped, fmt.Errorf("sync: inbox of %s: %w", account.ID(), err))
				continue
			}
			arrived = append(arrived, fresh...)
		}
	}
	return arrived, errors.Join(skipped...)
}

// refreshInbox fetches one inbox folder, applies the filter rules, saves the messages and returns the
// ones that are newly arrived (an id not already cached), excluding only those a filter rule marked read
// on arrival. Read state otherwise does not gate the result, so a message another client already marked
// read still counts. A message into an emptied folder counts as new; one never baselined announces
// nothing on its first pass.
func (s *SyncService) refreshInbox(ctx context.Context, account domain.Account, folder domain.Folder, rules []domain.Rule) ([]domain.MessageSummary, error) {
	existing, err := s.mail.ListMessages(ctx, folder.ID())
	if err != nil {
		return nil, err
	}
	baselined, err := s.folderBaselined(ctx, folder)
	if err != nil {
		return nil, err
	}
	fetch, err := s.fetchFolder(ctx, account, folder)
	if err != nil {
		return nil, err
	}
	fetched := fetch.messages
	// On an inbox never baselined or renumbered every fetched message counts as already held, so the
	// rules act on none of it and none of it is announced as new mail.
	known := fetch.knownFor(knownSetOf(existing), baselined)
	// Align local tag assignments with the server keywords before the rules run (IMAP only). Best-effort.
	s.reconcileTags(ctx, account, fetched)
	// Record each message's read state as the server reports it, before local rules run. A message
	// another client (Thunderbird, a phone) has already marked read is still a new arrival worth
	// announcing; only a message a filter rule marks read on arrival should be silenced.
	serverUnread := make(map[string]bool, len(fetched))
	for _, m := range fetched {
		serverUnread[m.ID()] = !m.IsRead()
	}
	// Rules run here too, so mail arriving on the background pass is filtered exactly as it is on an
	// explicit sync. A rule that could not be carried out is best-effort at this call site, in keeping
	// with the rest of the pass: it must not silence the other accounts.
	messages, marks, _ := s.applyRulesKnown(ctx, account, folder, fetched, known, baselined, rules)
	if account.Protocol() == domain.ProtocolPOP3 {
		messages = carryOverFlags(existing, messages)
	}
	// Overlay unconfirmed local flag changes last, so this save cannot regress a flag the user changed
	// but the server has not yet confirmed (the immediate un-read revert on servers with lazy STOREs).
	messages = s.reconcileFlags(ctx, account, messages)
	if err := s.mail.SaveMessages(ctx, folder.ID(), messages); err != nil {
		return nil, err
	}
	// Only now that the folder's contents are cached are its baseline, UIDVALIDITY and the rules' marks
	// established. Best-effort here, in keeping with the rest of this pass: an unmarked folder simply gets
	// another protected pass; an unrecorded UIDVALIDITY means another rebaseline; an unrecorded mark stays
	// local until the next sync brings the server's state back.
	_ = s.settleFolder(ctx, folder, fetch.validity, marks)
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
