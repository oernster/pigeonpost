package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// TagSyncService rounds user tags onto the server as IMAP keywords and keeps the local tag assignments in
// step with them. Assigning or removing a tag is applied locally at once and, for an IMAP account, recorded
// as a pending intent; a later sync replays those intents to the server (FlushPending) and reconciles the
// local assignments against the keywords the server reports (ReconcileFetched). A POP3 account has no
// server-side keywords, so its tags stay purely local and no intent is recorded.
type TagSyncService struct {
	tags     TagStore
	store    MailStore
	accounts AccountStore
	remote   MailActions
}

// NewTagSyncService constructs the service with its injected dependencies.
func NewTagSyncService(tags TagStore, store MailStore, accounts AccountStore, remote MailActions) *TagSyncService {
	return &TagSyncService{tags: tags, store: store, accounts: accounts, remote: remote}
}

// Assign attaches a tag to a message and, for an IMAP account, records the intent so a later sync rounds it
// onto the server. The local change and the pending intent are written together in one transaction, so a
// tag can never end up applied locally with no intent recorded (which a later reconcile would mistake for a
// server-cleared tag and delete). Assigning a tag already present is a no-op locally.
func (s *TagSyncService) Assign(ctx context.Context, messageID, tagID string) error {
	recordPending, err := s.recordsPending(ctx, messageID)
	if err != nil {
		return fmt.Errorf("assign tag %q to message %q: %w", tagID, messageID, err)
	}
	if err := s.tags.AssignMessageTag(ctx, messageID, tagID, recordPending); err != nil {
		return fmt.Errorf("assign tag %q to message %q: %w", tagID, messageID, err)
	}
	return nil
}

// Unassign detaches a tag from a message and, for an IMAP account, records the intent so a later sync
// removes the keyword on the server. The local change and the intent are written together in one transaction.
func (s *TagSyncService) Unassign(ctx context.Context, messageID, tagID string) error {
	recordPending, err := s.recordsPending(ctx, messageID)
	if err != nil {
		return fmt.Errorf("unassign tag %q from message %q: %w", tagID, messageID, err)
	}
	if err := s.tags.UnassignMessageTag(ctx, messageID, tagID, recordPending); err != nil {
		return fmt.Errorf("unassign tag %q from message %q: %w", tagID, messageID, err)
	}
	return nil
}

// recordsPending reports whether a tag change to the given message should record a pending intent to sync to
// the server: true only for an IMAP account. A POP3 account has no server-side keywords, so its tags stay
// local and no intent is recorded. Resolving the account first also means a lookup failure fails the change
// cleanly before anything is written, rather than leaving a local tag with no intent for a reconcile to
// mistake for a server-cleared one.
func (s *TagSyncService) recordsPending(ctx context.Context, messageID string) (bool, error) {
	_, _, account, err := resolveMessageContext(ctx, s.store, s.accounts, messageID)
	if err != nil {
		return false, err
	}
	return account.Protocol() != domain.ProtocolPOP3, nil
}

// ReconcileFetched aligns the local tag assignments of each freshly fetched message with the tag keywords the
// server reported on it. A keyword present on the server for a known tag is assigned locally; a known tag
// whose keyword is absent is unassigned locally; a pending intent overrides that until the server agrees with
// it, at which point the intent is cleared. It is best-effort, so the sync calls it for IMAP folders only
// (a POP3 message carries no keywords, which would otherwise read as every tag having been removed) and
// ignores the error it returns.
func (s *TagSyncService) ReconcileFetched(ctx context.Context, messages []domain.MessageSummary) error {
	if len(messages) == 0 {
		return nil
	}
	tags, err := s.tags.ListTags(ctx)
	if err != nil {
		return fmt.Errorf("reconcile tags: list tags: %w", err)
	}
	tagByKeyword := make(map[string]string, len(tags))
	for _, tag := range tags {
		tagByKeyword[tag.Keyword()] = tag.ID()
	}
	for _, msg := range messages {
		if err := s.reconcileMessage(ctx, msg, tagByKeyword); err != nil {
			return err
		}
	}
	return nil
}

// reconcileMessage reconciles one message's local tag assignments against the tag keywords the server
// reported on it, keyed through tagByKeyword (keyword to known tag id).
func (s *TagSyncService) reconcileMessage(ctx context.Context, msg domain.MessageSummary, tagByKeyword map[string]string) error {
	onServer := make(map[string]bool)
	for _, keyword := range msg.Keywords() {
		if tagID, ok := tagByKeyword[keyword]; ok {
			onServer[tagID] = true
		}
	}
	localTags, err := s.tags.TagsForMessage(ctx, msg.ID())
	if err != nil {
		return fmt.Errorf("reconcile tags for message %q: %w", msg.ID(), err)
	}
	onLocal := make(map[string]bool, len(localTags))
	for _, tag := range localTags {
		onLocal[tag.ID()] = true
	}
	pending, err := s.tags.PendingTagOps(ctx, msg.ID())
	if err != nil {
		return fmt.Errorf("reconcile pending for message %q: %w", msg.ID(), err)
	}

	involved := map[string]struct{}{}
	for tagID := range onServer {
		involved[tagID] = struct{}{}
	}
	for tagID := range onLocal {
		involved[tagID] = struct{}{}
	}
	for tagID := range pending {
		involved[tagID] = struct{}{}
	}

	for tagID := range involved {
		if intent, isPending := pending[tagID]; isPending {
			// While the server disagrees with the pending intent it is left in place, guarding the local
			// state until a flush lands it; once the server agrees the intent is confirmed and cleared.
			if intent == onServer[tagID] {
				if err := s.tags.ClearPendingTagOp(ctx, msg.ID(), tagID); err != nil {
					return fmt.Errorf("clear pending tag %q for message %q: %w", tagID, msg.ID(), err)
				}
			}
			continue
		}
		switch {
		case onServer[tagID] && !onLocal[tagID]:
			if err := s.tags.AddMessageTag(ctx, msg.ID(), tagID); err != nil {
				return fmt.Errorf("apply server tag %q to message %q: %w", tagID, msg.ID(), err)
			}
		case !onServer[tagID] && onLocal[tagID]:
			if err := s.tags.RemoveMessageTag(ctx, msg.ID(), tagID); err != nil {
				return fmt.Errorf("remove server-cleared tag %q from message %q: %w", tagID, msg.ID(), err)
			}
		}
	}
	return nil
}

// FlushPending replays every pending tag intent to the server, so a tag assigned or removed while offline
// reaches the server on a later sync. It is best-effort: a push that fails (the server is offline or rejects
// the keyword) leaves the intent to be retried, and the intent is cleared only once a reconcile sees the
// server agree with it. An intent for a message or tag that no longer exists is skipped.
func (s *TagSyncService) FlushPending(ctx context.Context) error {
	ops, err := s.tags.ListPendingTagOps(ctx)
	if err != nil {
		return fmt.Errorf("flush pending tags: list: %w", err)
	}
	if len(ops) == 0 {
		return nil
	}
	tags, err := s.tags.ListTags(ctx)
	if err != nil {
		return fmt.Errorf("flush pending tags: list tags: %w", err)
	}
	keywordByTag := make(map[string]string, len(tags))
	for _, tag := range tags {
		keywordByTag[tag.ID()] = tag.Keyword()
	}
	for _, op := range ops {
		s.pushPending(ctx, op, keywordByTag)
	}
	return nil
}

// pushPending replays one pending intent to the server, best-effort. A push failure (offline or a rejecting
// server) leaves the intent in place to be retried; an intent whose message or tag no longer exists is
// skipped.
func (s *TagSyncService) pushPending(ctx context.Context, op domain.PendingTagOp, keywordByTag map[string]string) {
	keyword, ok := keywordByTag[op.TagID()]
	if !ok {
		return
	}
	msg, folder, account, err := resolveMessageContext(ctx, s.store, s.accounts, op.MessageID())
	if err != nil {
		return
	}
	_ = s.remote.SetKeyword(ctx, account, folder, msg.UID(), keyword, op.Assigned())
}
