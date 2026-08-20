package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// RuleExecutor carries out the side-effecting outcomes of rule evaluation: moving a message into
// another folder and destroying one outright. The domain decides what should happen; this executes it
// against the server, because the domain does no I/O.
//
// It runs inside the sync, on messages just fetched and not yet cached, so a destroyed message never
// enters the local store at all: there is no Trash copy, no local row and nothing to tidy up.
type RuleExecutor struct {
	store  MailStore
	remote MailActions
}

// NewRuleExecutor constructs the executor with its injected store (to resolve move destinations) and
// remote mailbox.
func NewRuleExecutor(store MailStore, remote MailActions) *RuleExecutor {
	return &RuleExecutor{store: store, remote: remote}
}

// Apply runs the rules over the newly arrived messages in one folder, executes what they decided and
// returns the messages that should be saved locally: previously known messages untouched, arrivals with
// their flag actions applied, minus both the destroyed and the successfully moved.
//
// Only the rules covering this account are considered: a rule scoped to one address never sees mail
// arriving on another. Rules act only on arrivals (a message id the local store does not already
// hold), so adding a rule never reaches back over mail already in the mailbox. Destructive actions
// (move, destroy) are further held back until the folder has been baselined: on a folder never synced
// before, nothing is known locally and its whole backlog would look like arrivals, so that first pass
// records what is there and only sets flags. A batch the server refuses leaves its messages in place
// and contributes an error, so a partial failure is never silent.
//
// baselined is read from the store rather than inferred from len(known). An emptied inbox holds no
// cached messages either, so inferring it exempted every message arriving into a filed-clean mailbox
// from the destructive actions, permanently and silently, for anyone who keeps their inbox at zero.
func (e *RuleExecutor) Apply(ctx context.Context, account domain.Account, folder domain.Folder,
	fetched []domain.MessageSummary, known map[string]struct{}, baselined bool,
	rules []domain.Rule) ([]domain.MessageSummary, error) {
	if len(rules) == 0 || len(fetched) == 0 {
		return fetched, nil
	}
	// Narrow to the rules that cover this account before anything else, so an account-scoped rule
	// cannot reach mail arriving on another address.
	rules = domain.RulesForAccount(rules, account.ID())
	if len(rules) == 0 {
		return fetched, nil
	}
	arrivals, arrivalAt := splitArrivals(fetched, known)
	if len(arrivals) == 0 {
		return fetched, nil
	}
	outcomes := domain.EvaluateRules(arrivals, rules)
	// A folder not yet baselined is being seen for the first time: what it holds is a starting point,
	// not an arrival of everything in it.
	if !baselined {
		return applyOutcomeFlags(fetched, arrivalAt, outcomes, nil), nil
	}
	removed, err := e.execute(ctx, account, folder, arrivals, outcomes)
	return applyOutcomeFlags(fetched, arrivalAt, outcomes, removed), err
}

// splitArrivals returns the messages not already known locally, together with each fetched message's
// index into that arrivals slice (-1 when it was already known).
func splitArrivals(fetched []domain.MessageSummary, known map[string]struct{}) ([]domain.MessageSummary, []int) {
	arrivals := make([]domain.MessageSummary, 0, len(fetched))
	at := make([]int, len(fetched))
	for i, m := range fetched {
		if _, seen := known[m.ID()]; seen {
			at[i] = -1
			continue
		}
		at[i] = len(arrivals)
		arrivals = append(arrivals, m)
	}
	return arrivals, at
}

// applyOutcomeFlags rebuilds the set to save: an arrival takes its outcome's message (flag actions
// applied) unless its id is in removed; a known message passes through untouched.
func applyOutcomeFlags(fetched []domain.MessageSummary, arrivalAt []int,
	outcomes []domain.RuleOutcome, removed map[string]struct{}) []domain.MessageSummary {
	out := make([]domain.MessageSummary, 0, len(fetched))
	for i, m := range fetched {
		if arrivalAt[i] < 0 {
			out = append(out, m)
			continue
		}
		if _, gone := removed[m.ID()]; gone {
			continue
		}
		out = append(out, outcomes[arrivalAt[i]].Message)
	}
	return out
}

// execute performs the destroys and moves the outcomes call for and returns the ids that left the
// folder, so the caller drops exactly those and keeps everything a failed batch left behind.
func (e *RuleExecutor) execute(ctx context.Context, account domain.Account, folder domain.Folder,
	arrivals []domain.MessageSummary, outcomes []domain.RuleOutcome) (map[string]struct{}, error) {
	removed := make(map[string]struct{})
	var errs []error
	destroy, moves := e.groupOutcomes(ctx, account, folder, arrivals, outcomes, &errs)
	if len(destroy.uids) > 0 {
		if err := e.destroy(ctx, account, folder, destroy, removed); err != nil {
			errs = append(errs, err)
		}
	}
	for destPath, batch := range moves {
		if err := e.move(ctx, account, folder, destPath, batch, removed); err != nil {
			errs = append(errs, err)
		}
	}
	return removed, errors.Join(errs...)
}

// ruleBatch is a set of messages bound for one destination, holding both the server handles to act on
// and the local ids to drop once the server agrees.
type ruleBatch struct {
	uids []string
	ids  []string
}

// add records one message in the batch.
func (b *ruleBatch) add(m domain.MessageSummary) {
	b.uids = append(b.uids, m.UID())
	b.ids = append(b.ids, m.ID())
}

// groupOutcomes sorts the outcomes into one destroy batch and one move batch per destination mailbox.
// A destination in another account (or the folder the message is already in) is skipped: the first
// cannot be moved to across an account boundary, the second is already where the rule wants it.
func (e *RuleExecutor) groupOutcomes(ctx context.Context, account domain.Account, folder domain.Folder,
	arrivals []domain.MessageSummary, outcomes []domain.RuleOutcome, errs *[]error) (ruleBatch, map[string]*ruleBatch) {
	var destroy ruleBatch
	moves := make(map[string]*ruleBatch)
	for i, o := range outcomes {
		switch {
		case o.Destroy:
			destroy.add(arrivals[i])
		case o.MoveToFolderID == "" || o.MoveToFolderID == folder.ID():
			continue
		default:
			dest, err := e.store.GetFolder(ctx, o.MoveToFolderID)
			if err != nil {
				*errs = append(*errs, fmt.Errorf("rules: resolve destination %q: %w", o.MoveToFolderID, err))
				continue
			}
			if dest.AccountID() != account.ID() {
				continue
			}
			batch, ok := moves[dest.Path()]
			if !ok {
				batch = &ruleBatch{}
				moves[dest.Path()] = batch
			}
			batch.add(arrivals[i])
		}
	}
	return destroy, moves
}

// destroy expunges a batch from the server outright. The empty trash path is what makes it permanent:
// the messages are marked deleted and expunged where they stand, never copied to Trash first.
func (e *RuleExecutor) destroy(ctx context.Context, account domain.Account, folder domain.Folder,
	batch ruleBatch, removed map[string]struct{}) error {
	if _, err := e.remote.DeleteMany(ctx, account, folder, batch.uids, ""); err != nil {
		return fmt.Errorf("rules: destroy %d message(s) in %q: %w", len(batch.uids), folder.Path(), err)
	}
	markRemoved(batch.ids, removed)
	return nil
}

// move relocates a batch into another mailbox of the same account.
func (e *RuleExecutor) move(ctx context.Context, account domain.Account, folder domain.Folder,
	destPath string, batch *ruleBatch, removed map[string]struct{}) error {
	if _, err := e.remote.MoveMany(ctx, account, folder, batch.uids, destPath); err != nil {
		return fmt.Errorf("rules: move %d message(s) to %q: %w", len(batch.uids), destPath, err)
	}
	markRemoved(batch.ids, removed)
	return nil
}

// markRemoved records the ids that have left the folder on the server.
func markRemoved(ids []string, removed map[string]struct{}) {
	for _, id := range ids {
		removed[id] = struct{}{}
	}
}
