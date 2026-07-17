package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// FlagSyncService keeps locally made flag changes (read, starred, answered, forwarded) in step with the
// server. A mark action applies the change to the cache at once and records it as a pending intent; a
// later sync replays those intents to the server (FlushPending) and guards them against a fetch that
// still reports the old value (ReconcileFetched), because some servers (Outlook.com among them) accept a
// flag STORE and then report the stale flag on the next fetch, or drop the STORE outright. Without the
// guard the sync would faithfully write that stale view over the cache, un-reading a message the user
// just viewed. An intent is cleared only once a fetch shows the server agreeing with it, mirroring
// TagSyncService.
type FlagSyncService struct {
	store    MailStore
	accounts AccountStore
	remote   MailActions
}

// NewFlagSyncService constructs the service with its injected dependencies.
func NewFlagSyncService(store MailStore, accounts AccountStore, remote MailActions) *FlagSyncService {
	return &FlagSyncService{store: store, accounts: accounts, remote: remote}
}

// FlushPending replays every pending flag intent to the server, so a flag changed while offline (or one
// the server dropped) reaches the server on a later sync. It is best-effort: a push that fails leaves
// the intent to be retried, and the intent is cleared only once a reconcile sees the server agree with
// it. An intent for a message that no longer exists is skipped; the store sweeps those rows when the
// message goes.
func (s *FlagSyncService) FlushPending(ctx context.Context) error {
	ops, err := s.store.ListPendingFlagOps(ctx)
	if err != nil {
		return fmt.Errorf("flush pending flags: list: %w", err)
	}
	for _, op := range ops {
		s.pushPending(ctx, op)
	}
	return nil
}

// pushPending replays one pending intent to the server, best-effort. The message's context is resolved
// at push time, so the STORE targets the message's current UID even when it has changed since the intent
// was recorded. A flag with no server counterpart is skipped.
func (s *FlagSyncService) pushPending(ctx context.Context, op domain.PendingFlagOp) {
	msg, folder, account, err := resolveMessageContext(ctx, s.store, s.accounts, op.MessageID())
	if err != nil {
		return
	}
	switch op.Flag() {
	case domain.FlagSeen:
		_ = s.remote.SetSeen(ctx, account, folder, msg.UID(), op.Value())
	case domain.FlagFlagged:
		_ = s.remote.SetFlagged(ctx, account, folder, msg.UID(), op.Value())
	case domain.FlagAnswered:
		_ = s.remote.SetAnswered(ctx, account, folder, msg.UID(), op.Value())
	case domain.FlagForwarded:
		_ = s.remote.SetForwarded(ctx, account, folder, msg.UID(), op.Value())
	}
}

// ReconcileFetched overlays the pending flag intents onto freshly fetched message summaries, so a save
// of the fetched set cannot regress an unconfirmed local change. For each fetched message with a pending
// intent: when the server already agrees with the intent it is confirmed and cleared; while it disagrees
// the fetched flags are rewritten to the intended value, guarding the local state until a flush lands
// it. Messages without intents pass through unchanged, and the common no-intents case costs one query.
func (s *FlagSyncService) ReconcileFetched(ctx context.Context, messages []domain.MessageSummary) ([]domain.MessageSummary, error) {
	if len(messages) == 0 {
		return messages, nil
	}
	ops, err := s.store.ListPendingFlagOps(ctx)
	if err != nil {
		return nil, fmt.Errorf("reconcile flags: list pending: %w", err)
	}
	if len(ops) == 0 {
		return messages, nil
	}
	byMessage := make(map[string]map[domain.Flag]bool)
	for _, op := range ops {
		if byMessage[op.MessageID()] == nil {
			byMessage[op.MessageID()] = make(map[domain.Flag]bool)
		}
		byMessage[op.MessageID()][op.Flag()] = op.Value()
	}
	out := make([]domain.MessageSummary, len(messages))
	for i, msg := range messages {
		out[i] = msg
		pending, hasPending := byMessage[msg.ID()]
		if !hasPending {
			continue
		}
		flags := msg.Flags()
		changed := false
		for flag, want := range pending {
			if flags.Has(flag) == want {
				if err := s.store.ClearPendingFlagOp(ctx, msg.ID(), flag); err != nil {
					return nil, fmt.Errorf("reconcile flags: confirm %q: %w", msg.ID(), err)
				}
				continue
			}
			if want {
				flags = flags.With(flag)
			} else {
				flags = flags.Without(flag)
			}
			changed = true
		}
		if changed {
			out[i] = msg.WithFlags(flags)
		}
	}
	return out, nil
}
