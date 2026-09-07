package application

import (
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// localNames is what an incoming rule is measured against: the account ids and folder ids this
// installation actually holds. A rules file is written on one machine and read on another, so every
// name in it is a claim about the reader's world rather than a fact.
type localNames struct {
	accounts map[string]struct{}
	folders  map[string]struct{}
}

// build turns one file record into a domain rule under the given id, switching it OFF where it cannot
// act as written here.
//
// Two things a file can name that this installation may not have: a destination folder and an account
// scope. Neither is an error, since both may exist later (a folder appears on the next sync, an account
// is added tomorrow), so the rule is imported as written and disabled. Disabling is the honest answer
// because the alternatives are worse: left enabled, a rule whose destination is unknown silently does
// nothing on every sync while looking active, which is the failure the editor already refuses to save;
// dropping the unreachable part instead would quietly change what the user wrote. For a scope that
// means widening a rule to every account, which is the last thing a filter should do on its own.
func (l localNames) build(in RuleTransfer, id string) (domain.Rule, error) {
	mode, err := parseTransferMatchMode(in.MatchMode)
	if err != nil {
		return domain.Rule{}, err
	}
	conditions, err := buildTransferConditions(in.Conditions)
	if err != nil {
		return domain.Rule{}, err
	}
	actions, reachable, err := l.buildActions(in.Actions)
	if err != nil {
		return domain.Rule{}, err
	}
	return domain.NewRule(domain.RuleSpec{
		ID:             id,
		Name:           in.Name,
		Enabled:        in.Enabled && reachable && l.scopeIsKnown(in.Accounts),
		MatchMode:      mode,
		StopProcessing: in.StopProcessing,
		AccountIDs:     in.Accounts,
		Conditions:     conditions,
		Actions:        actions,
	})
}

// scopeIsKnown reports whether a rule's account scope means anything here. An unscoped rule covers
// every account, so it is always known; a scoped one needs at least one of its accounts to exist,
// otherwise it can never match a message.
func (l localNames) scopeIsKnown(accounts []string) bool {
	if len(accounts) == 0 {
		return true
	}
	for _, id := range accounts {
		if _, ok := l.accounts[id]; ok {
			return true
		}
	}
	return false
}

// buildActions rebuilds each action, reporting whether every destination it names exists here.
func (l localNames) buildActions(in []RuleTransferAction) ([]domain.RuleAction, bool, error) {
	actions := make([]domain.RuleAction, 0, len(in))
	reachable := true
	for _, a := range in {
		kind, err := parseTransferActionKind(a.Kind)
		if err != nil {
			return nil, false, err
		}
		folderID := ""
		// A move names both halves of its destination or it names none: composing an id from a missing
		// half would produce a folder id that is not empty and not a folder either, which the action
		// constructor would accept and no sync could ever act on. Left empty, it is refused by name.
		if kind == domain.RuleMoveTo && a.Account != "" && a.Folder != "" {
			folderID = domain.FolderIDFor(a.Account, a.Folder)
			if _, ok := l.folders[folderID]; !ok {
				reachable = false
			}
		}
		action, err := domain.NewRuleAction(kind, folderID)
		if err != nil {
			return nil, false, err
		}
		actions = append(actions, action)
	}
	return actions, reachable, nil
}

// buildTransferConditions rebuilds each condition from its file spellings.
func buildTransferConditions(in []RuleTransferCondition) ([]domain.RuleCondition, error) {
	conditions := make([]domain.RuleCondition, 0, len(in))
	for _, c := range in {
		field, err := parseTransferField(c.Field)
		if err != nil {
			return nil, err
		}
		operator, err := parseTransferOperator(c.Operator)
		if err != nil {
			return nil, err
		}
		condition, err := domain.NewRuleConditionFull(field, operator, c.Text, c.MatchCase, c.Not)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, condition)
	}
	return conditions, nil
}

// toRuleTransfer is the outward half: a stored rule as the file records it, with its folder ids split
// back into the account and mailbox path they are made of.
func toRuleTransfer(r domain.Rule) RuleTransfer {
	conditions := make([]RuleTransferCondition, 0, len(r.Conditions()))
	for _, c := range r.Conditions() {
		conditions = append(conditions, RuleTransferCondition{
			Field: c.Field().String(), Operator: c.Operator().String(), Text: c.Text(),
			MatchCase: c.CaseSensitive(), Not: c.Negated(),
		})
	}
	actions := make([]RuleTransferAction, 0, len(r.Actions()))
	for _, a := range r.Actions() {
		account, folder := domain.SplitFolderID(a.FolderID())
		actions = append(actions, RuleTransferAction{Kind: a.Kind().String(), Account: account, Folder: folder})
	}
	accounts := r.AccountIDs()
	if accounts == nil {
		accounts = []string{}
	}
	return RuleTransfer{
		Name: r.Name(), Enabled: r.Enabled(), MatchMode: r.MatchMode().String(),
		StopProcessing: r.StopProcessing(), Accounts: accounts,
		Conditions: conditions, Actions: actions,
	}
}

// The four parsers below turn a file's stable string spellings back into domain values. They are the
// import's own, not the facade's: a file is read from disk and may say anything, so an unknown token
// is named in an error rather than defaulted into something that silently does the wrong thing.

func parseTransferField(s string) (domain.RuleField, error) {
	for _, f := range []domain.RuleField{
		domain.RuleFieldFrom, domain.RuleFieldSubject, domain.RuleFieldTo, domain.RuleFieldCc,
		domain.RuleFieldAnyRecipient, domain.RuleFieldSenderDomain, domain.RuleFieldAll,
	} {
		if f.String() == s {
			return f, nil
		}
	}
	return 0, fmt.Errorf("unknown rule field %q", s)
}

func parseTransferOperator(s string) (domain.RuleOperator, error) {
	for _, o := range []domain.RuleOperator{
		domain.RuleOpContains, domain.RuleOpNotContains, domain.RuleOpEquals,
		domain.RuleOpStartsWith, domain.RuleOpEndsWith,
	} {
		if o.String() == s {
			return o, nil
		}
	}
	return 0, fmt.Errorf("unknown rule operator %q", s)
}

func parseTransferActionKind(s string) (domain.RuleActionKind, error) {
	for _, k := range []domain.RuleActionKind{
		domain.RuleMarkRead, domain.RuleFlag, domain.RuleMoveTo, domain.RuleDestroy,
	} {
		if k.String() == s {
			return k, nil
		}
	}
	return 0, fmt.Errorf("unknown rule action %q", s)
}

func parseTransferMatchMode(s string) (domain.RuleMatchMode, error) {
	for _, m := range []domain.RuleMatchMode{domain.RuleMatchAll, domain.RuleMatchAny} {
		if m.String() == s {
			return m, nil
		}
	}
	return 0, fmt.Errorf("unknown rule match mode %q", s)
}
