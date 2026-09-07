package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ErrRuleNotFound is returned when a backfill names a rule that no longer exists.
var ErrRuleNotFound = errors.New("rule not found")

// ErrRuleDisabled is returned when a backfill is asked for a disabled rule. A disabled rule is skipped
// by the evaluator, so running one would report that it changed nothing and leave the user guessing
// whether the rule matched nothing or was simply switched off.
var ErrRuleDisabled = errors.New("rule is disabled")

// RuleBackfillCounts reports the work one rule implies over mail already in the mailboxes: what a
// preview found or what a run actually carried out. The two are read the same way, so the dialog that
// asks for confirmation and the message that reports the result share one shape.
//
// A message is counted once per action it receives, so a rule that both flags and moves a message
// contributes to Flag and to Move. The counts are therefore a description of the work, never a count of
// distinct messages.
type RuleBackfillCounts struct {
	// Scanned is how many cached messages the rule was evaluated against.
	Scanned int `json:"scanned"`
	// MarkRead is how many unread messages the rule marks as read.
	MarkRead int `json:"markRead"`
	// Flag is how many unflagged messages the rule flags.
	Flag int `json:"flag"`
	// Move is how many messages the rule moves into another folder.
	Move int `json:"move"`
	// Destroy is how many messages the rule deletes outright, with no Trash hop.
	Destroy int `json:"destroy"`
}

// Acts reports whether the rule changes anything at all, so a caller can say "nothing to do" rather
// than raise a confirmation for an empty plan.
func (c RuleBackfillCounts) Acts() bool {
	return c.MarkRead > 0 || c.Flag > 0 || c.Move > 0 || c.Destroy > 0
}

// RuleBackfillActions is the subset of message actions a backfill carries out. It is the existing
// MessageActionService seen through a narrower door: those methods already drive the server and the
// local cache together; reproducing that here would be a second implementation of moving and
// deleting mail, free to drift from the first.
type RuleBackfillActions interface {
	MarkRead(ctx context.Context, messageID string, read bool) error
	MarkFlagged(ctx context.Context, messageID string, flagged bool) error
	MoveMany(ctx context.Context, messageIDs []string, destFolderID string) ([]string, map[string]string, error)
	DeleteMany(ctx context.Context, messageIDs []string, permanent bool) ([]string, map[string]string, error)
}

// RuleBackfillService applies one existing rule to mail already stored, which the sync deliberately
// never does: RuleExecutor acts on arrivals only, so writing a rule has no effect on the backlog. This
// is the user asking for that backlog to be filtered, once, on demand.
//
// Two differences from the sync path are deliberate. It runs over EVERY folder of every account the
// rule covers, not the Inbox alone, because a backfill is an explicit instruction about mail already
// filed rather than an unattended reaction to mail arriving. It evaluates the ONE named rule rather
// than the whole ordered set, because that is what the button on that rule's row says it does; a rule's
// stop-processing flag consequently has nothing to stop.
type RuleBackfillService struct {
	rules    RuleStore
	accounts AccountStore
	store    MailStore
	actions  RuleBackfillActions
}

// NewRuleBackfillService constructs the service with its injected rule store, account store, mail store
// and message actions.
func NewRuleBackfillService(rules RuleStore, accounts AccountStore, store MailStore,
	actions RuleBackfillActions) *RuleBackfillService {
	return &RuleBackfillService{rules: rules, accounts: accounts, store: store, actions: actions}
}

// Preview evaluates the rule over the stored mail and reports what running it would do, without
// touching a message. It exists so a destructive rule can be confirmed against real counts: a backfill
// runs unattended over a whole backlog, so it cannot ask about each message the way a manual delete can.
//
// A folder that cannot be read contributes an error and is left out of the counts, so a preview is
// never quietly narrower than it looks.
func (s *RuleBackfillService) Preview(ctx context.Context, ruleID string) (RuleBackfillCounts, error) {
	plan, err := s.plan(ctx, ruleID)
	if plan == nil {
		return RuleBackfillCounts{}, err
	}
	return plan.counts, err
}

// backfillPlan is the work one rule implies, gathered before any of it is carried out. Moves are
// grouped by destination so each destination costs one batched server round trip; moveOrder keeps
// those destinations in the order they were first seen, so a run is reproducible.
type backfillPlan struct {
	counts    RuleBackfillCounts
	markRead  []string
	flag      []string
	moves     map[string][]string
	moveOrder []string
	destroy   []string
}

// newBackfillPlan returns an empty plan ready to collect work.
func newBackfillPlan() *backfillPlan {
	return &backfillPlan{moves: make(map[string][]string)}
}

// addMove records one message bound for a destination folder, remembering a destination the first time
// it is seen.
func (p *backfillPlan) addMove(destFolderID, messageID string) {
	if _, seen := p.moves[destFolderID]; !seen {
		p.moveOrder = append(p.moveOrder, destFolderID)
	}
	p.moves[destFolderID] = append(p.moves[destFolderID], messageID)
	p.counts.Move++
}

// plan builds the whole plan for a rule: every folder of every account the rule covers, evaluated and
// sorted into the four kinds of work. Errors from individual folders are joined and returned alongside
// the plan rather than instead of it, so an unreadable folder does not cancel the rest of the backfill.
func (s *RuleBackfillService) plan(ctx context.Context, ruleID string) (*backfillPlan, error) {
	rule, err := s.findRule(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	accounts, err := s.accounts.ListAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("rules: backfill: list accounts: %w", err)
	}
	plan := newBackfillPlan()
	var errs []error
	for _, account := range accounts {
		if !rule.AppliesTo(account.ID()) {
			continue
		}
		folders, err := s.store.ListFolders(ctx, account.ID())
		if err != nil {
			errs = append(errs, fmt.Errorf("rules: backfill: list folders for %q: %w", account.ID(), err))
			continue
		}
		for _, folder := range folders {
			messages, err := s.store.ListMessages(ctx, folder.ID())
			if err != nil {
				errs = append(errs, fmt.Errorf("rules: backfill: list messages in %q: %w", folder.ID(), err))
				continue
			}
			s.planFolder(ctx, account, folder, messages, rule, plan, &errs)
		}
	}
	return plan, errors.Join(errs...)
}

// findRule reads the named rule and refuses a missing or disabled one.
func (s *RuleBackfillService) findRule(ctx context.Context, ruleID string) (domain.Rule, error) {
	rules, err := s.rules.ListRules(ctx)
	if err != nil {
		return domain.Rule{}, fmt.Errorf("rules: backfill: list: %w", err)
	}
	for _, r := range rules {
		if r.ID() != ruleID {
			continue
		}
		if !r.Enabled() {
			return domain.Rule{}, ErrRuleDisabled
		}
		return r, nil
	}
	return domain.Rule{}, fmt.Errorf("rules: backfill %q: %w", ruleID, ErrRuleNotFound)
}

// planFolder evaluates one folder's stored messages against the rule and records the work.
//
// A destroy takes the message whole: nothing else is worth doing to a message that will not exist, so
// the flag actions are skipped for it, matching the domain's own rule that a destroy ends evaluation.
// Flag actions are recorded only where they CHANGE the message, so a rule that marks mail read does not
// report the whole folder as work when most of it has been read for months.
func (s *RuleBackfillService) planFolder(ctx context.Context, account domain.Account, folder domain.Folder,
	messages []domain.MessageSummary, rule domain.Rule, plan *backfillPlan, errs *[]error) {
	plan.counts.Scanned += len(messages)
	outcomes := domain.EvaluateRules(messages, []domain.Rule{rule})
	for i, outcome := range outcomes {
		message := messages[i]
		if outcome.Destroy {
			plan.destroy = append(plan.destroy, message.ID())
			plan.counts.Destroy++
			continue
		}
		before, after := message.Flags(), outcome.Message.Flags()
		if !before.Has(domain.FlagSeen) && after.Has(domain.FlagSeen) {
			plan.markRead = append(plan.markRead, message.ID())
			plan.counts.MarkRead++
		}
		if !before.Has(domain.FlagFlagged) && after.Has(domain.FlagFlagged) {
			plan.flag = append(plan.flag, message.ID())
			plan.counts.Flag++
		}
		if dest, ok := s.moveDestination(ctx, account, folder, outcome, errs); ok {
			plan.addMove(dest, message.ID())
		}
	}
}

// moveDestination reports the folder a message should move into and whether the move applies at all. A
// message already in the destination has nowhere to go; a destination in another account cannot be
// reached from here, since a move is a server-side operation within one mailbox tree. Both are skipped
// silently rather than reported, because neither is a failure: they are the rule asking for something
// that is already true or for something outside the account it is running over.
func (s *RuleBackfillService) moveDestination(ctx context.Context, account domain.Account,
	folder domain.Folder, outcome domain.RuleOutcome, errs *[]error) (string, bool) {
	if outcome.MoveToFolderID == "" || outcome.MoveToFolderID == folder.ID() {
		return "", false
	}
	dest, err := s.store.GetFolder(ctx, outcome.MoveToFolderID)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("rules: backfill: resolve destination %q: %w", outcome.MoveToFolderID, err))
		return "", false
	}
	if dest.AccountID() != account.ID() {
		return "", false
	}
	return outcome.MoveToFolderID, true
}
