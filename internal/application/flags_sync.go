package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// FlagSyncService keeps locally made flag changes (read, starred, answered, forwarded) in step with the
// server. A mark action applies the change to the cache at once and records it as a pending intent; a
// later sync replays those intents to the server (FlushPending) and guards them against a fetch that
// still reports the old value (ReconcileFetched), because some servers (Outlook.com among them) accept a
// flag STORE and then report the stale flag on the next fetch; some drop the STORE outright. Without the
// guard the sync would faithfully write that stale view over the cache, un-reading a message the user
// just viewed. An intent is cleared only once the server is seen agreeing with it, whether in a fetch
// or in the read-back that follows the replay's own push.
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
// the server dropped) reaches the server on a later sync. The intents are pushed in batches, one per
// folder, flag and value, each over one connection (see MailActions.PushFlag); an intent the server
// settles is cleared at once. Pushing each intent on a connection of its own while clearing none until
// the folder happened to be fetched turned 14785 mark-read intents in folders no background pass fetches
// into a login every few seconds; StartMail blocked the address for it on 2026-10-07.
//
// It is best-effort: a batch that fails leaves its intents to be retried and is joined into the error,
// so the caller can record it. An intent the server keeps disagreeing with stays too, so a reconcile can
// still guard the local value. An intent for a message that no longer exists is skipped; the store
// sweeps those rows when the message goes.
func (s *FlagSyncService) FlushPending(ctx context.Context) error {
	ops, err := s.store.ListPendingFlagOps(ctx)
	if err != nil {
		return fmt.Errorf("flush pending flags: list: %w", err)
	}
	intents := make([]pendingIntent[domain.Flag], 0, len(ops))
	for _, op := range ops {
		intents = append(intents, pendingIntent[domain.Flag]{messageID: op.MessageID(), key: op.Flag(), set: op.Value()})
	}
	var failures []error
	for _, batch := range batchPending(ctx, s.store, s.accounts, intents) {
		settled, err := s.remote.PushFlag(ctx, batch.account, batch.folder, batch.uids, batch.key, batch.set)
		if err != nil {
			failures = append(failures, fmt.Errorf("flush pending flags: %d in %q: %w", len(batch.uids), batch.folder.Path(), err))
			continue
		}
		for _, uid := range settled {
			if err := s.store.ClearPendingFlagOp(ctx, batch.messageIDs[uid], batch.key); err != nil {
				failures = append(failures, fmt.Errorf("flush pending flags: clear %q: %w", batch.messageIDs[uid], err))
			}
		}
	}
	return errors.Join(failures...)
}

// ReconcileFetched overlays the pending flag intents onto freshly fetched message summaries, so a save
// of the fetched set cannot regress an unconfirmed local change. For each fetched message with a pending
// intent: when the server already agrees with the intent it is confirmed and cleared; while it disagrees
// the fetched flags are rewritten to the intended value, guarding the local state until a flush lands
// it. Messages without intents pass through unchanged; the common no-intents case costs one query.
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
