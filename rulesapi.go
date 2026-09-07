package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// RuleConditionDTO is the JSON-serialisable view of one rule condition. Field and operator are stable
// string tokens ("from", "anyRecipient", "contains", "endsWith" and so on) so the front end does not
// depend on the domain enum values.
type RuleConditionDTO struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Text     string `json:"text"`
	// CaseSensitive makes the comparison exact; the default is case-insensitive matching.
	CaseSensitive bool `json:"caseSensitive"`
}

// RuleActionDTO is the JSON-serialisable view of one rule action. Kind is a stable string token
// ("markRead", "flag", "moveTo", "destroy"); FolderID is the destination of a move and empty otherwise.
type RuleActionDTO struct {
	Kind     string `json:"kind"`
	FolderID string `json:"folderId"`
}

// RuleDTO is the JSON-serialisable view of a filter rule, carried in both directions: the front end
// lists rules as these and sends one back to save. An empty ID on a save means a new rule.
type RuleDTO struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Enabled        bool   `json:"enabled"`
	Position       int    `json:"position"`
	MatchMode      string `json:"matchMode"`
	StopProcessing bool   `json:"stopProcessing"`
	// AccountIDs limits the rule to the named accounts; empty means every account.
	AccountIDs []string           `json:"accountIds"`
	Conditions []RuleConditionDTO `json:"conditions"`
	Actions    []RuleActionDTO    `json:"actions"`
}

// ListRules returns all filter rules in evaluation order.
func (a *App) ListRules() ([]RuleDTO, error) {
	rules, err := a.rules.List(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RuleDTO, 0, len(rules))
	for _, r := range rules {
		out = append(out, ruleToDTO(r))
	}
	return out, nil
}

// SaveRule creates or updates a filter rule.
func (a *App) SaveRule(req RuleDTO) error {
	matchMode, err := parseRuleMatchMode(req.MatchMode)
	if err != nil {
		return err
	}
	conditions, err := parseRuleConditions(req.Conditions)
	if err != nil {
		return err
	}
	actions, err := parseRuleActions(req.Actions)
	if err != nil {
		return err
	}
	return a.rules.Save(a.ctx, application.RuleInput{
		ID:             req.ID,
		Name:           req.Name,
		Enabled:        req.Enabled,
		Position:       req.Position,
		MatchMode:      matchMode,
		StopProcessing: req.StopProcessing,
		AccountIDs:     req.AccountIDs,
		Conditions:     conditions,
		Actions:        actions,
	})
}

// DeleteRule removes a filter rule by id.
func (a *App) DeleteRule(ruleID string) error {
	return a.rules.Delete(a.ctx, ruleID)
}

// ReorderRules writes the evaluation order, the rule at index i taking position i.
func (a *App) ReorderRules(orderedIDs []string) error {
	return a.rules.Reorder(a.ctx, orderedIDs)
}

// RuleBackfillDTO is the JSON-serialisable view of what applying one rule to the mail already stored
// would do or did do. The same shape carries both: the counts fill the confirmation before a run and
// report the outcome after it, so the front end formats one thing rather than two.
type RuleBackfillDTO struct {
	Scanned  int `json:"scanned"`
	MarkRead int `json:"markRead"`
	Flag     int `json:"flag"`
	Move     int `json:"move"`
	Destroy  int `json:"destroy"`
	// Cancelled reports that the run stopped early, so the counts above are of the work that had
	// already landed. That work is irreversible, so it is reported rather than hidden.
	Cancelled bool `json:"cancelled"`
}

// RuleBackfillProgressDTO is the JSON-serialisable view of how far a backfill has got, carried on the
// ruleBackfillProgressEvent Wails event rather than returned: a bound call answers once, at the end,
// which is exactly when progress has stopped being useful.
type RuleBackfillProgressDTO struct {
	Phase string `json:"phase"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

// ruleBackfillProgressEvent is the Wails event a backfill's progress is emitted on. It is one event for
// both phases, each naming itself, so the front end holds one listener rather than one per phase.
const ruleBackfillProgressEvent = "rules:backfill-progress"

// PreviewRuleBackfill reports what applying the named rule to the mail already in the mailboxes would
// do, changing nothing. The front end shows these counts for confirmation before calling RunRuleBackfill,
// because a backfill acts on a whole backlog unattended and so cannot ask about each message.
func (a *App) PreviewRuleBackfill(ruleID string) (RuleBackfillDTO, error) {
	ctx, done := a.beginBackfill()
	defer done()
	counts, err := a.ruleBackfill.Preview(ctx, ruleID, a.emitBackfillProgress)
	return ruleBackfillToDTO(counts), summariseBackfillError(err)
}

// RunRuleBackfill applies the named rule to the mail already in the mailboxes and reports what it did.
// The counts are of work that succeeded, so a partially refused run reports the part that landed and
// returns the error describing the rest.
func (a *App) RunRuleBackfill(ruleID string) (RuleBackfillDTO, error) {
	ctx, done := a.beginBackfill()
	defer done()
	counts, err := a.ruleBackfill.Run(ctx, ruleID, a.emitBackfillProgress)
	return ruleBackfillToDTO(counts), summariseBackfillError(err)
}

// maxReportedBackfillFailures bounds how many of a backfill's failures reach the interface. A backfill
// acts on thousands of messages, so a bad server or a bad batch can produce thousands of errors; joined,
// they arrive as one unreadable wall in a banner meant for a sentence. Three is enough to show what kind
// of failure it is; the count that follows is what says how widespread it was.
const maxReportedBackfillFailures = 3

// summariseBackfillError turns a joined error into something a person can read. errors.Join separates
// its parts with newlines, which is what makes the count possible; the parts beyond the cap are reduced
// to how many there were rather than dropped silently, since the difference between four failures and
// four hundred is the whole story.
func summariseBackfillError(err error) error {
	if err == nil {
		return nil
	}
	failures := strings.Split(err.Error(), "\n")
	if len(failures) <= maxReportedBackfillFailures {
		return err
	}
	shown := strings.Join(failures[:maxReportedBackfillFailures], "\n")
	return fmt.Errorf("%s\nand %d more like this", shown, len(failures)-maxReportedBackfillFailures)
}

// CancelRuleBackfill stops the backfill in flight, if there is one. It is safe to call when none is
// running, since the dialog's Cancel can race the run's own completion. A cancelled run is not an
// error: it returns the work that had already landed, with Cancelled set on its counts.
func (a *App) CancelRuleBackfill() {
	a.backfillMu.Lock()
	stop := a.backfillStop
	a.backfillMu.Unlock()
	if stop != nil {
		stop()
	}
}

// beginBackfill derives a cancellable context for one backfill and publishes its cancel so
// CancelRuleBackfill can reach it. The returned function releases both and must be deferred: a stale
// cancel left published would stop the NEXT run the moment it started.
//
// The generation is what makes that release safe. Only one backfill should run at a time (the button
// is disabled while one does), yet a run that finished can only clear the published cancel if it is
// still its own: clearing unconditionally would strand a newer run with nothing able to stop it.
func (a *App) beginBackfill() (context.Context, func()) {
	ctx, cancel := context.WithCancel(a.ctx)
	a.backfillMu.Lock()
	a.backfillGen++
	mine := a.backfillGen
	a.backfillStop = cancel
	a.backfillMu.Unlock()
	return ctx, func() {
		a.backfillMu.Lock()
		if a.backfillGen == mine {
			a.backfillStop = nil
		}
		a.backfillMu.Unlock()
		cancel()
	}
}

// emitBackfillProgress puts one progress reading on the wire. The application layer knows nothing of
// Wails, so it reports through a plain function and this is the one place that turns a reading into an
// event.
func (a *App) emitBackfillProgress(p application.RuleBackfillProgress) {
	runtime.EventsEmit(a.ctx, ruleBackfillProgressEvent, RuleBackfillProgressDTO{
		Phase: p.Phase, Done: p.Done, Total: p.Total,
	})
}

// ruleBackfillToDTO converts the application counts to their wire view.
func ruleBackfillToDTO(c application.RuleBackfillCounts) RuleBackfillDTO {
	return RuleBackfillDTO{
		Scanned: c.Scanned, MarkRead: c.MarkRead, Flag: c.Flag, Move: c.Move, Destroy: c.Destroy,
		Cancelled: c.Cancelled,
	}
}

// ruleToDTO converts a domain rule to its wire view.
func ruleToDTO(r domain.Rule) RuleDTO {
	conditions := make([]RuleConditionDTO, 0, len(r.Conditions()))
	for _, c := range r.Conditions() {
		conditions = append(conditions, RuleConditionDTO{
			Field: c.Field().String(), Operator: c.Operator().String(), Text: c.Text(),
			CaseSensitive: c.CaseSensitive(),
		})
	}
	actions := make([]RuleActionDTO, 0, len(r.Actions()))
	for _, a := range r.Actions() {
		actions = append(actions, RuleActionDTO{Kind: a.Kind().String(), FolderID: a.FolderID()})
	}
	// A rule limited to no account has a nil AccountIDs; encoding/json writes a nil slice as null
	// rather than []. The front end's type declares an array, so a null there is not a wrong value but
	// a crash when its length is read, with no error boundary above the app, so that takes the whole
	// window down instead of one dialog. Conditions and actions are already built with make, so they
	// cannot be nil; this is the one list that can. It is fixed here rather than in the front end
	// because the wire shape is this function's promise to keep.
	accountIDs := r.AccountIDs()
	if accountIDs == nil {
		accountIDs = []string{}
	}
	return RuleDTO{
		ID: r.ID(), Name: r.Name(), Enabled: r.Enabled(), Position: r.Position(),
		MatchMode: r.MatchMode().String(), StopProcessing: r.StopProcessing(),
		AccountIDs: accountIDs, Conditions: conditions, Actions: actions,
	}
}

// parseRuleConditions converts the wire conditions to their application inputs.
func parseRuleConditions(in []RuleConditionDTO) ([]application.RuleConditionInput, error) {
	out := make([]application.RuleConditionInput, 0, len(in))
	for _, c := range in {
		field, err := parseRuleField(c.Field)
		if err != nil {
			return nil, err
		}
		operator, err := parseRuleOperator(c.Operator)
		if err != nil {
			return nil, err
		}
		out = append(out, application.RuleConditionInput{
			Field: field, Operator: operator, Text: c.Text, CaseSensitive: c.CaseSensitive,
		})
	}
	return out, nil
}

// parseRuleActions converts the wire actions to their application inputs.
func parseRuleActions(in []RuleActionDTO) ([]application.RuleActionInput, error) {
	out := make([]application.RuleActionInput, 0, len(in))
	for _, a := range in {
		kind, err := parseRuleActionKind(a.Kind)
		if err != nil {
			return nil, err
		}
		out = append(out, application.RuleActionInput{Kind: kind, FolderID: a.FolderID})
	}
	return out, nil
}

func parseRuleField(s string) (domain.RuleField, error) {
	switch s {
	case "from":
		return domain.RuleFieldFrom, nil
	case "subject":
		return domain.RuleFieldSubject, nil
	case "to":
		return domain.RuleFieldTo, nil
	case "cc":
		return domain.RuleFieldCc, nil
	case "anyRecipient":
		return domain.RuleFieldAnyRecipient, nil
	case "senderDomain":
		return domain.RuleFieldSenderDomain, nil
	case "all":
		return domain.RuleFieldAll, nil
	default:
		return 0, fmt.Errorf("unknown rule field %q", s)
	}
}

func parseRuleOperator(s string) (domain.RuleOperator, error) {
	switch s {
	case "contains":
		return domain.RuleOpContains, nil
	case "notContains":
		return domain.RuleOpNotContains, nil
	case "equals":
		return domain.RuleOpEquals, nil
	case "startsWith":
		return domain.RuleOpStartsWith, nil
	case "endsWith":
		return domain.RuleOpEndsWith, nil
	default:
		return 0, fmt.Errorf("unknown rule operator %q", s)
	}
}

func parseRuleMatchMode(s string) (domain.RuleMatchMode, error) {
	switch s {
	case "all":
		return domain.RuleMatchAll, nil
	case "any":
		return domain.RuleMatchAny, nil
	default:
		return 0, fmt.Errorf("unknown rule match mode %q", s)
	}
}

func parseRuleActionKind(s string) (domain.RuleActionKind, error) {
	switch s {
	case "markRead":
		return domain.RuleMarkRead, nil
	case "flag":
		return domain.RuleFlag, nil
	case "moveTo":
		return domain.RuleMoveTo, nil
	case "destroy":
		return domain.RuleDestroy, nil
	default:
		return 0, fmt.Errorf("unknown rule action %q", s)
	}
}
