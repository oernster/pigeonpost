package application

import (
	"context"
	"errors"
	"fmt"
)

// actionBatchSize bounds how many messages one server round trip carries. A rule that files a mailbox
// routinely matches thousands; sending them as one command has two costs. The progress bar cannot
// move until the whole lot comes back (a move of 2414 messages sat at its opening reading for the
// entire operation, which reads as a hang), nor can a cancel take effect until then. It also
// keeps a single IMAP UID set to a length every server accepts. It matches the message list's own page
// size, which is the same trade of round trips against responsiveness.
const actionBatchSize = 200

// batched splits ids into runs of at most actionBatchSize, the unit of one server call, one progress
// step and one cancellation check. A short slice yields one batch; an empty one yields none.
func batched(ids []string) [][]string {
	var out [][]string
	for start := 0; start < len(ids); start += actionBatchSize {
		end := start + actionBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		out = append(out, ids[start:end])
	}
	return out
}

// Run applies the rule to the stored mail and reports what it actually did, which is not always what
// Preview said it would: a server may refuse a batch; mail may have arrived or moved between the
// two calls. The returned counts are therefore taken from the work that succeeded, never from the plan.
//
// The order is flags, then moves, then destroys. Flags go first so a message the rule both marks and
// moves carries its new state into the destination (an IMAP move preserves flags); destroys go last
// so a failure earlier in the run has not already deleted the evidence of it. A batch the server
// refuses leaves its messages where they are and contributes an error, so a partial failure is never
// silent.
//
// Cancelling the context stops the run between batches and is not an error: the counts come back with
// Cancelled set, reporting the work that had already landed. A backfill's work is irreversible, so a
// cancel that pretended nothing had happened would be a lie about the mailbox.
func (s *RuleBackfillService) Run(ctx context.Context, ruleID string, to RuleBackfillReport) (RuleBackfillCounts, error) {
	plan, err := s.plan(ctx, ruleID, to)
	if plan == nil {
		return RuleBackfillCounts{}, err
	}
	var errs []error
	if err != nil {
		errs = append(errs, err)
	}
	run := &backfillRun{ctx: ctx, to: to, total: plan.actionCount()}
	report(to, RuleBackfillApplying, 0, run.total)
	done := RuleBackfillCounts{Scanned: plan.counts.Scanned}
	done.MarkRead = s.applyFlags(ctx, plan.markRead, s.actions.MarkRead, "mark read", run, &errs)
	done.Flag = s.applyFlags(ctx, plan.flag, s.actions.MarkFlagged, "flag", run, &errs)
	done.Move = s.applyMoves(ctx, plan, run, &errs)
	done.Destroy = s.applyDestroys(ctx, plan, run, &errs)
	done.Cancelled = plan.cancelled || run.stopped()
	report(to, RuleBackfillFinished, run.done, run.total)
	return done, errors.Join(errs...)
}

// actionCount is how many messages the plan will act on, the total the applying phase counts against.
// It counts attempts rather than successes, so the bar does not shorten when a batch is refused.
func (p *backfillPlan) actionCount() int {
	moves := 0
	for _, ids := range p.moves {
		moves += len(ids)
	}
	return len(p.markRead) + len(p.flag) + moves + len(p.destroy)
}

// backfillRun carries the applying phase's running position, so each step advances one counter rather
// than each recomputing where the run has got to. It also holds the context every step checks, which is
// what makes a cancel take effect part way through rather than at the end.
type backfillRun struct {
	ctx   context.Context
	to    RuleBackfillReport
	total int
	done  int
}

// stopped reports whether the run should go no further. It is checked between batches rather than
// inside one: a server call already issued is seen through, since abandoning it would leave the caller
// unable to say whether those messages moved.
func (r *backfillRun) stopped() bool { return r.ctx.Err() != nil }

// advance records that n more messages have been attempted and reports the new position. Attempted,
// not succeeded: a refused batch has still been waited for, so a bar that ignored it would stop while
// the run carried on.
func (r *backfillRun) advance(n int) {
	r.done += n
	report(r.to, RuleBackfillApplying, r.done, r.total)
}

// applyFlags sets one flag across a set of messages and returns how many took it. Each message is its
// own call because that is the shape of the underlying action: the flag write lands in the cache with
// its pending intent before the server is asked, so a message whose server push fails still shows the
// change locally and is replayed by the next sync.
func (s *RuleBackfillService) applyFlags(ctx context.Context, messageIDs []string,
	set func(context.Context, string, bool) error, what string, run *backfillRun, errs *[]error) int {
	changed := 0
	for _, id := range messageIDs {
		if run.stopped() {
			return changed
		}
		if err := set(ctx, id, true); err != nil {
			*errs = append(*errs, fmt.Errorf("rules: backfill: %s %q: %w", what, id, err))
		} else {
			changed++
		}
		run.advance(1)
	}
	return changed
}

// applyMoves relocates each destination's messages and returns how many left their folder. The count
// comes from the ids each move reports as moved, so a batch the server refuses contributes nothing to
// it while the rest of the destination still goes.
func (s *RuleBackfillService) applyMoves(ctx context.Context, plan *backfillPlan, run *backfillRun, errs *[]error) int {
	moved := 0
	for _, destFolderID := range plan.moveOrder {
		for _, batch := range batched(plan.moves[destFolderID]) {
			if run.stopped() {
				return moved
			}
			done, _, err := s.actions.MoveMany(ctx, batch, destFolderID)
			moved += len(done)
			if err != nil {
				*errs = append(*errs, fmt.Errorf("rules: backfill: move %d message(s) to %q: %w",
					len(batch), destFolderID, err))
			}
			run.advance(len(batch))
		}
	}
	return moved
}

// applyDestroys deletes the rule's destroy set outright and returns how many were removed. The deletion
// is permanent, with no Trash hop, because that is what the destroy action means everywhere else: a
// rule that destroys mail is documented as irreversible and a backfill must not quietly soften it into
// something recoverable.
func (s *RuleBackfillService) applyDestroys(ctx context.Context, plan *backfillPlan, run *backfillRun, errs *[]error) int {
	deleted := 0
	for _, batch := range batched(plan.destroy) {
		if run.stopped() {
			return deleted
		}
		gone, _, err := s.actions.DeleteMany(ctx, batch, true)
		deleted += len(gone)
		if err != nil {
			*errs = append(*errs, fmt.Errorf("rules: backfill: destroy %d message(s): %w", len(batch), err))
		}
		run.advance(len(batch))
	}
	return deleted
}
