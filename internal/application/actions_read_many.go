package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// A bulk mark-read is split in two so the unread badges can follow the list at once. MarkReadMany
// writes the cache (the badges' source) for the whole selection in one transaction and returns;
// PushReadMany then lands the change on the server with one connection per folder. The caller runs
// the push after refreshing the badges. The per-message MarkRead did both halves for every message in
// turn, a login each, so the badges waited on the whole selection's round trips.

// hasServerFlags reports whether an account keeps read state on the server. POP3 has no server flags,
// so its read state is purely local: no pending intent is recorded and nothing is pushed.
func hasServerFlags(account domain.Account) bool {
	return account.Protocol() != domain.ProtocolPOP3
}

// MarkReadMany sets or clears the read state of several messages in the cache, recording the pending
// intent for the server-flagged accounts in the same transaction as the change, exactly as MarkRead
// does for one. It does not touch the server; PushReadMany does. It returns the ids written, so the
// caller knows which rows now hold the new state.
func (s *MessageActionService) MarkReadMany(ctx context.Context, messageIDs []string, read bool) ([]string, error) {
	batches, errs := s.batchByFolder(ctx, messageIDs, batchRules{})
	var pending, local []string
	for _, b := range batches {
		if hasServerFlags(b.account) {
			pending = append(pending, b.ids...)
		} else {
			local = append(local, b.ids...)
		}
	}
	written := make([]string, 0, len(pending)+len(local))
	for _, group := range []struct {
		ids           []string
		recordPending bool
	}{{pending, true}, {local, false}} {
		if len(group.ids) == 0 {
			continue
		}
		if err := s.store.SetFlagMany(ctx, group.ids, domain.FlagSeen, read, group.recordPending); err != nil {
			errs = append(errs, fmt.Errorf("set cached read state on %d messages: %w", len(group.ids), err))
			continue
		}
		written = append(written, group.ids...)
	}
	return written, errors.Join(errs...)
}

// PushReadMany lands a bulk read-state change on the server, one connection per folder. Like the push
// in MarkRead it is best effort: MarkReadMany already recorded the intent, so a folder whose push fails
// (offline, say) is replayed by the next sync. The error names each failed folder for the caller to log.
func (s *MessageActionService) PushReadMany(ctx context.Context, messageIDs []string, read bool) error {
	batches, errs := s.batchByFolder(ctx, messageIDs, batchRules{})
	for _, b := range batches {
		if !hasServerFlags(b.account) {
			continue
		}
		if err := s.remote.SetSeenMany(ctx, b.account, b.folder, b.uids, read); err != nil {
			errs = append(errs, fmt.Errorf("set read state on %d messages in %q on server: %w", len(b.uids), b.folder.ID(), err))
		}
	}
	return errors.Join(errs...)
}
