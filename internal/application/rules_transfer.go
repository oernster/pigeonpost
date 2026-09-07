package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// RuleTransferCondition is one condition as it travels to and from a rules file: the same fields the
// editor shows, in the stable string spellings the wire already uses, with no local identity in it.
type RuleTransferCondition struct {
	Field     string
	Operator  string
	Text      string
	MatchCase bool
	Not       bool
}

// RuleTransferAction is one action as it travels to and from a rules file. A move names its
// destination as the account plus the mailbox path rather than as a folder id, because a folder id is
// exactly those two joined by a control character: splitting it keeps the file readable and lets an
// import say which folder a rule wanted when the local cache does not hold it.
type RuleTransferAction struct {
	Kind    string
	Account string
	Folder  string
}

// RuleTransfer is one rule as it travels to and from a rules file. It deliberately carries no id and
// no position: an id is local to the database that minted it and would be meaningless on another
// machine, while the order is the order of the file.
type RuleTransfer struct {
	Name           string
	Enabled        bool
	MatchMode      string
	StopProcessing bool
	Accounts       []string
	Conditions     []RuleTransferCondition
	Actions        []RuleTransferAction
}

// RuleCodec serialises rules to and from a file's bytes. The format lives in infrastructure; the use
// case below knows only that it round-trips.
type RuleCodec interface {
	Encode(rules []RuleTransfer) ([]byte, error)
	Decode(data []byte) ([]RuleTransfer, error)
}

// RuleImportPlan is what an import WOULD do, reported before anything is written. Rules move and
// destroy mail unattended, so a file is described before it is applied rather than after.
type RuleImportPlan struct {
	// Add and Replace name the rules that would be added and the ones that would replace a stored rule
	// of the same name. The name is the identity across machines; an id is not.
	Add     []string
	Replace []string
	// Disable names the rules that would arrive switched off because they cannot act as written here:
	// a move to a folder this installation does not hold; a scope naming only unknown accounts.
	Disable []string
	// Destructive names the rules that move or destroy mail, so the reader sees what they are agreeing
	// to run.
	Destructive []string
}

// Total reports how many rules the plan covers.
func (p RuleImportPlan) Total() int { return len(p.Add) + len(p.Replace) }

// RuleImportResult reports what an import actually did.
type RuleImportResult struct {
	Added    int
	Replaced int
	Disabled []string
}

// RuleTransferService is the use-case boundary for reading and writing a rules file. It is separate
// from RuleService because the two answer different questions: one edits a rule, this one moves a set
// of them between installations, where nothing local (ids, positions, folder ids) may be assumed.
type RuleTransferService struct {
	rules    RuleStore
	accounts AccountStore
	folders  folderLister
	newID    IDGenerator
}

// NewRuleTransferService constructs the service with its injected stores and id generator.
func NewRuleTransferService(
	rules RuleStore, accounts AccountStore, folders folderLister, newID IDGenerator,
) *RuleTransferService {
	return &RuleTransferService{rules: rules, accounts: accounts, folders: folders, newID: newID}
}

// Export encodes every stored rule, in evaluation order, into the codec's file bytes.
func (s *RuleTransferService) Export(ctx context.Context, codec RuleCodec) ([]byte, error) {
	rules, err := s.rules.ListRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("rules: list for export: %w", err)
	}
	out := make([]RuleTransfer, 0, len(rules))
	for _, r := range rules {
		out = append(out, toRuleTransfer(r))
	}
	data, err := codec.Encode(out)
	if err != nil {
		return nil, fmt.Errorf("rules: encode export: %w", err)
	}
	return data, nil
}

// Plan decodes a rules file and reports what importing it would do, writing nothing.
func (s *RuleTransferService) Plan(ctx context.Context, codec RuleCodec, data []byte) (RuleImportPlan, error) {
	incoming, existing, local, err := s.read(ctx, codec, data)
	if err != nil {
		return RuleImportPlan{}, err
	}
	var plan RuleImportPlan
	for _, in := range incoming {
		rule, err := local.build(in, "plan")
		if err != nil {
			return RuleImportPlan{}, fmt.Errorf("rules: import %q: %w", in.Name, err)
		}
		if _, found := existing[in.Name]; found {
			plan.Replace = append(plan.Replace, in.Name)
		} else {
			plan.Add = append(plan.Add, in.Name)
		}
		if in.Enabled && !rule.Enabled() {
			plan.Disable = append(plan.Disable, in.Name)
		}
		if rule.Destructive() {
			plan.Destructive = append(plan.Destructive, in.Name)
		}
	}
	return plan, nil
}

// Import writes the file's rules. A rule whose name matches a stored one replaces it in place, keeping
// its position, so re-importing a file updates rather than duplicating; anything else is appended after
// the existing rules in the order the file gives.
func (s *RuleTransferService) Import(ctx context.Context, codec RuleCodec, data []byte) (RuleImportResult, error) {
	incoming, existing, local, err := s.read(ctx, codec, data)
	if err != nil {
		return RuleImportResult{}, err
	}
	next := len(existing)
	var result RuleImportResult
	for _, in := range incoming {
		id, position := s.newID(), next
		match, found := existing[in.Name]
		if found {
			id, position = match.ID(), match.Position()
		} else {
			next++
		}
		rule, err := local.build(in, id)
		if err != nil {
			return result, fmt.Errorf("rules: import %q: %w", in.Name, err)
		}
		if err := s.rules.SaveRule(ctx, rule.WithPosition(position)); err != nil {
			return result, fmt.Errorf("rules: save imported %q: %w", in.Name, err)
		}
		if found {
			result.Replaced++
		} else {
			result.Added++
		}
		if in.Enabled && !rule.Enabled() {
			result.Disabled = append(result.Disabled, in.Name)
		}
	}
	return result, nil
}

// read decodes the file and gathers what the incoming rules are resolved against: the stored rules by
// name, plus this installation's accounts and folders.
func (s *RuleTransferService) read(
	ctx context.Context, codec RuleCodec, data []byte,
) ([]RuleTransfer, map[string]domain.Rule, localNames, error) {
	incoming, err := codec.Decode(data)
	if err != nil {
		return nil, nil, localNames{}, fmt.Errorf("rules: decode import: %w", err)
	}
	stored, err := s.rules.ListRules(ctx)
	if err != nil {
		return nil, nil, localNames{}, fmt.Errorf("rules: list for import: %w", err)
	}
	existing := make(map[string]domain.Rule, len(stored))
	for _, r := range stored {
		existing[r.Name()] = r
	}
	local, err := s.localNames(ctx)
	if err != nil {
		return nil, nil, localNames{}, err
	}
	return incoming, existing, local, nil
}

// localNames gathers the account ids and folder ids this installation actually holds, which is what
// decides whether an incoming rule can act as written.
func (s *RuleTransferService) localNames(ctx context.Context) (localNames, error) {
	accounts, err := s.accounts.ListAccounts(ctx)
	if err != nil {
		return localNames{}, fmt.Errorf("rules: list accounts for import: %w", err)
	}
	known := localNames{accounts: make(map[string]struct{}, len(accounts)), folders: map[string]struct{}{}}
	for _, a := range accounts {
		known.accounts[a.ID()] = struct{}{}
		folders, err := s.folders.ListFolders(ctx, a.ID())
		if err != nil {
			return localNames{}, fmt.Errorf("rules: list folders for import: %w", err)
		}
		for _, f := range folders {
			known.folders[f.ID()] = struct{}{}
		}
	}
	return known, nil
}
