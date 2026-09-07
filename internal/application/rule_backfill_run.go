package application

import (
	"context"
	"errors"
	"fmt"
)

// Run applies the rule to the stored mail and reports what it actually did, which is not always what
// Preview said it would: a server may refuse a batch; mail may have arrived or moved between the
// two calls. The returned counts are therefore taken from the work that succeeded, never from the plan.
//
// The order is flags, then moves, then destroys. Flags go first so a message the rule both marks and
// moves carries its new state into the destination (an IMAP move preserves flags); destroys go last
// so a failure earlier in the run has not already deleted the evidence of it. A batch the server
// refuses leaves its messages where they are and contributes an error, so a partial failure is never
// silent.
func (s *RuleBackfillService) Run(ctx context.Context, ruleID string) (RuleBackfillCounts, error) {
	plan, err := s.plan(ctx, ruleID)
	if plan == nil {
		return RuleBackfillCounts{}, err
	}
	var errs []error
	if err != nil {
		errs = append(errs, err)
	}
	done := RuleBackfillCounts{Scanned: plan.counts.Scanned}
	done.MarkRead = s.applyFlags(ctx, plan.markRead, s.actions.MarkRead, "mark read", &errs)
	done.Flag = s.applyFlags(ctx, plan.flag, s.actions.MarkFlagged, "flag", &errs)
	done.Move = s.applyMoves(ctx, plan, &errs)
	done.Destroy = s.applyDestroys(ctx, plan, &errs)
	return done, errors.Join(errs...)
}

// applyFlags sets one flag across a set of messages and returns how many took it. Each message is its
// own call because that is the shape of the underlying action: the flag write lands in the cache with
// its pending intent before the server is asked, so a message whose server push fails still shows the
// change locally and is replayed by the next sync.
func (s *RuleBackfillService) applyFlags(ctx context.Context, messageIDs []string,
	set func(context.Context, string, bool) error, what string, errs *[]error) int {
	changed := 0
	for _, id := range messageIDs {
		if err := set(ctx, id, true); err != nil {
			*errs = append(*errs, fmt.Errorf("rules: backfill: %s %q: %w", what, id, err))
			continue
		}
		changed++
	}
	return changed
}

// applyMoves relocates each destination's batch in one server round trip and returns how many messages
// left their folder. The count comes from the ids the move reports as moved, so a destination whose
// batch is refused contributes nothing to it.
func (s *RuleBackfillService) applyMoves(ctx context.Context, plan *backfillPlan, errs *[]error) int {
	moved := 0
	for _, destFolderID := range plan.moveOrder {
		ids := plan.moves[destFolderID]
		done, _, err := s.actions.MoveMany(ctx, ids, destFolderID)
		moved += len(done)
		if err != nil {
			*errs = append(*errs, fmt.Errorf("rules: backfill: move %d message(s) to %q: %w", len(ids), destFolderID, err))
		}
	}
	return moved
}

// applyDestroys deletes the rule's destroy set outright and returns how many were removed. The deletion
// is permanent, with no Trash hop, because that is what the destroy action means everywhere else: a
// rule that destroys mail is documented as irreversible and a backfill must not quietly soften it into
// something recoverable.
func (s *RuleBackfillService) applyDestroys(ctx context.Context, plan *backfillPlan, errs *[]error) int {
	if len(plan.destroy) == 0 {
		return 0
	}
	deleted, _, err := s.actions.DeleteMany(ctx, plan.destroy, true)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("rules: backfill: destroy %d message(s): %w", len(plan.destroy), err))
	}
	return len(deleted)
}
