package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// applyRules runs the filter rules over one folder's freshly fetched messages and returns what should
// be saved locally. It is the sync's single entry point into the rule engine, so every sync path scopes
// rules the same way.
//
// Rules run on the Inbox only. Mail arrives there; it is the one folder where acting on a message
// is what the user asked for: a rule that moved or destroyed messages in Sent, Archive or Trash would
// act on mail the user had already filed by hand. Within the Inbox, rules act only on arrivals, their
// destructive actions held back on a folder's first sight (see RuleExecutor.Apply).
//
// The returned error reports a rule that could not be carried out. The messages come back regardless,
// so the caller saves what it has before deciding what to do with the error.
func (s *SyncService) applyRules(ctx context.Context, account domain.Account, folder domain.Folder,
	fetched []domain.MessageSummary, rules []domain.Rule) ([]domain.MessageSummary, error) {
	if len(rules) == 0 || folder.Kind() != domain.FolderInbox {
		return fetched, nil
	}
	known, err := s.knownIDs(ctx, folder.ID())
	if err != nil {
		return fetched, err
	}
	return s.ruleExec.Apply(ctx, account, folder, fetched, known, rules)
}

// applyRulesKnown is applyRules for a caller that has already listed the folder's cached messages, so
// the folder is not read twice.
func (s *SyncService) applyRulesKnown(ctx context.Context, account domain.Account, folder domain.Folder,
	fetched []domain.MessageSummary, known map[string]struct{}, rules []domain.Rule) ([]domain.MessageSummary, error) {
	if len(rules) == 0 || folder.Kind() != domain.FolderInbox {
		return fetched, nil
	}
	return s.ruleExec.Apply(ctx, account, folder, fetched, known, rules)
}

// knownIDs reads the ids the local store already holds for a folder, the set that separates an arrival
// from a message seen before.
func (s *SyncService) knownIDs(ctx context.Context, folderID string) (map[string]struct{}, error) {
	ids, err := s.mail.MessageIDs(ctx, folderID)
	if err != nil {
		return nil, fmt.Errorf("sync: read known messages for %q: %w", folderID, err)
	}
	known := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		known[id] = struct{}{}
	}
	return known, nil
}

// knownSetOf builds the known-id set from message summaries a caller already holds.
func knownSetOf(messages []domain.MessageSummary) map[string]struct{} {
	known := make(map[string]struct{}, len(messages))
	for _, m := range messages {
		known[m.ID()] = struct{}{}
	}
	return known
}
