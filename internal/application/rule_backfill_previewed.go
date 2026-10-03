package application

import (
	"errors"
	"sync"
)

// ErrRuleBackfillNotPreviewed is returned when a run is asked for a rule with no preview waiting. A run
// carries out exactly the work a preview showed and the user agreed to, so with no preview there is
// nothing it has been permitted to do. Each preview permits one run.
var ErrRuleBackfillNotPreviewed = errors.New("rule backfill has not been previewed")

// previewedPlans holds the last previewed plan of each rule until a run takes it. It is keyed by rule
// because that is what the confirmation is about: the dialog shows the counts of the preview it just
// made, for the rule whose Now button was pressed, so the latest preview of that rule is the one agreed
// to. Bound calls arrive on their own goroutines, hence the lock.
type previewedPlans struct {
	mu     sync.Mutex
	byRule map[string]*backfillPlan
}

// keep records a rule's previewed plan, replacing any earlier one. A nil plan (a preview that failed
// outright) drops the earlier one instead, so a run can never act on a plan older than the counts the
// user last saw.
func (p *previewedPlans) keep(ruleID string, plan *backfillPlan) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if plan == nil {
		delete(p.byRule, ruleID)
		return
	}
	if p.byRule == nil {
		p.byRule = make(map[string]*backfillPlan)
	}
	p.byRule[ruleID] = plan
}

// take removes and returns a rule's previewed plan; it returns nil when none is waiting. Removing it is what
// makes one preview good for one run: a second run must be previewed again.
func (p *previewedPlans) take(ruleID string) *backfillPlan {
	p.mu.Lock()
	defer p.mu.Unlock()
	plan := p.byRule[ruleID]
	delete(p.byRule, ruleID)
	return plan
}

// confirmedBy is the part of a previewed plan that a fresh scan still wants, action by action: a message
// is kept for an action only when both the preview and the fresh scan list it for that action (for a
// move, to the same destination). Mail that arrived after the preview is in the fresh scan alone, so it
// is left untouched; a previewed message that has since gone, been changed by hand or stopped matching
// is in the preview alone, so it is skipped. Run reports what it then did, so the skipped messages
// simply do not appear in its counts.
func (p *backfillPlan) confirmedBy(fresh *backfillPlan) *backfillPlan {
	kept := newBackfillPlan()
	kept.cancelled = fresh.cancelled
	kept.counts.Scanned = fresh.counts.Scanned
	kept.markRead = stillListed(p.markRead, fresh.markRead)
	kept.flag = stillListed(p.flag, fresh.flag)
	for _, dest := range p.moveOrder {
		for _, id := range stillListed(p.moves[dest], fresh.moves[dest]) {
			kept.addMove(dest, id)
		}
	}
	kept.destroy = stillListed(p.destroy, fresh.destroy)
	return kept
}

// stillListed returns the previewed ids the fresh list also holds, in their previewed order.
func stillListed(previewed, fresh []string) []string {
	present := make(map[string]bool, len(fresh))
	for _, id := range fresh {
		present[id] = true
	}
	var out []string
	for _, id := range previewed {
		if present[id] {
			out = append(out, id)
		}
	}
	return out
}
