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
// act on mail the user had already filed by hand. Within the Inbox, rules act only on arrivals.
//
// A folder never baselined or one the server has renumbered is rebaselined instead (see
// folderFetch.knownFor): every message in it counts as already held, so no rule acts on it that pass.
//
// The returned marks are the flags the rules set on arriving mail of an account with server flags; the
// caller records them once the messages are saved (see recordRuleMarks).
//
// The returned error reports a rule that could not be carried out. The messages come back regardless,
// so the caller saves what it has before deciding what to do with the error.
func (s *SyncService) applyRules(ctx context.Context, account domain.Account, folder domain.Folder,
	fetch folderFetch, rules []domain.Rule) ([]domain.MessageSummary, []ruleMark, error) {
	if len(rules) == 0 || folder.Kind() != domain.FolderInbox {
		return fetch.messages, nil, nil
	}
	known, err := s.knownIDs(ctx, folder.ID())
	if err != nil {
		return fetch.messages, nil, err
	}
	baselined, err := s.folderBaselined(ctx, folder)
	if err != nil {
		return fetch.messages, nil, err
	}
	return s.applyRulesKnown(ctx, account, folder, fetch.messages, fetch.knownFor(known, baselined), baselined, rules)
}

// applyRulesKnown is applyRules for a caller that has already listed the folder's cached messages and
// read its baseline, so neither is read twice.
func (s *SyncService) applyRulesKnown(ctx context.Context, account domain.Account, folder domain.Folder,
	fetched []domain.MessageSummary, known map[string]struct{}, baselined bool,
	rules []domain.Rule) ([]domain.MessageSummary, []ruleMark, error) {
	if len(rules) == 0 || folder.Kind() != domain.FolderInbox {
		return fetched, nil, nil
	}
	ruled, err := s.ruleExec.Apply(ctx, account, folder, fetched, known, baselined, rules)
	// A POP3 account has no server flags to land a mark on; preserveFlags keeps its marks locally.
	if !hasServerFlags(account) {
		return ruled, nil, err
	}
	return ruled, ruleMarksOf(fetched, ruled), err
}

// ruleMark is one flag the rules set, with the ids of the messages they set it on.
type ruleMark struct {
	flag domain.Flag
	ids  []string
}

// ruleMarksOf returns the flags the rules added to the messages they kept: each bit a message's ruled
// copy holds that its fetched copy lacked. Rules only ever set flags (mark read, flag), so this is
// exactly what they changed. The marks come in the order each flag is first met.
func ruleMarksOf(fetched, ruled []domain.MessageSummary) []ruleMark {
	before := make(map[string]domain.Flag, len(fetched))
	for _, m := range fetched {
		before[m.ID()] = m.Flags().Raw()
	}
	var marks []ruleMark
	at := make(map[domain.Flag]int)
	for _, m := range ruled {
		added := m.Flags().Raw() &^ before[m.ID()]
		for flag := domain.Flag(1); added != 0; flag <<= 1 {
			if added&flag == 0 {
				continue
			}
			added &^= flag
			i, ok := at[flag]
			if !ok {
				i = len(marks)
				at[flag] = i
				marks = append(marks, ruleMark{flag: flag})
			}
			marks[i].ids = append(marks[i].ids, m.ID())
		}
	}
	return marks
}

// recordRuleMarks records each flag a rule set on arriving mail as a pending intent, exactly as a user's
// mark is, so the next sync replays it to the server (FlagSyncService.FlushPending) and guards it until
// the server agrees (ReconcileFetched). Without it a rule's mark changed only the cache and the next
// sync brought the server's state back. It runs after the save, because the store records an intent
// only against a cached message.
func (s *SyncService) recordRuleMarks(ctx context.Context, marks []ruleMark) error {
	for _, m := range marks {
		if err := s.mail.SetFlagMany(ctx, m.ids, m.flag, true, true); err != nil {
			return err
		}
	}
	return nil
}

// folderBaselined reads whether a folder has had its baseline pass. A mark that cannot be read fails
// the caller rather than defaulting either way: defaulting to baselined would hand a newly added
// account's backlog to the rules and the arrival notice; the other way would silence a real arrival.
func (s *SyncService) folderBaselined(ctx context.Context, folder domain.Folder) (bool, error) {
	baselined, err := s.mail.FolderBaselined(ctx, folder.ID())
	if err != nil {
		return false, fmt.Errorf("sync: read baseline for %q: %w", folder.ID(), err)
	}
	return baselined, nil
}

// markBaselined records that a folder's contents are now cached, so the next pass may act destructively
// on what arrives after them. It is called only once the fetched messages are saved: a folder marked
// ahead of a failed save would have its whole backlog read as arrivals next time.
//
// Only the Inbox is marked, because it is the only folder the rules run on. The mark is idempotent, so
// every sync calling it costs one no-op insert.
func (s *SyncService) markBaselined(ctx context.Context, folder domain.Folder) error {
	if folder.Kind() != domain.FolderInbox {
		return nil
	}
	return s.mail.MarkFolderBaselined(ctx, folder.ID())
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
