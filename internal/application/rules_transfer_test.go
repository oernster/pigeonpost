package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// fakeRuleCodec is the file format seen from the use case: whatever the test puts in comes back out.
type fakeRuleCodec struct {
	decoded   []RuleTransfer
	encoded   []RuleTransfer
	encodeErr error
	decodeErr error
}

func (c *fakeRuleCodec) Encode(rules []RuleTransfer) ([]byte, error) {
	if c.encodeErr != nil {
		return nil, c.encodeErr
	}
	c.encoded = rules
	return []byte("encoded"), nil
}

func (c *fakeRuleCodec) Decode([]byte) ([]RuleTransfer, error) {
	if c.decodeErr != nil {
		return nil, c.decodeErr
	}
	return c.decoded, nil
}

// transferHarness wires the service over the in-memory fakes, with one account holding one folder.
func transferHarness(t *testing.T) (*RuleTransferService, *fakeRuleStore, *fakeAccountStore, *fakeMailStore) {
	t.Helper()
	rules := &fakeRuleStore{}
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{testFolder(t, domain.FolderIDFor("a1", "Filed"), "a1", "Filed")}
	minted := 0
	newID := func() string {
		minted++
		return "new-id"
	}
	return NewRuleTransferService(rules, accounts, mail, newID), rules, accounts, mail
}

// transferRule is one file record, complete and importable, which each test then varies.
func transferRule() RuleTransfer {
	return RuleTransfer{
		Name: "News", Enabled: true, MatchMode: "all",
		Conditions: []RuleTransferCondition{{Field: "from", Operator: "contains", Text: "news@"}},
		Actions:    []RuleTransferAction{{Kind: "markRead"}},
	}
}

func TestExportRulesRecordsEachRuleWithoutItsLocalIdentity(t *testing.T) {
	service, rules, _, _ := transferHarness(t)
	cond, err := domain.NewRuleConditionFull(domain.RuleFieldSubject, domain.RuleOpEndsWith, "digest", true, true)
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	move, err := domain.NewRuleAction(domain.RuleMoveTo, domain.FolderIDFor("a1", "Filed"))
	if err != nil {
		t.Fatalf("action: %v", err)
	}
	stored, err := domain.NewRule(domain.RuleSpec{
		ID: "r1", Name: "Digests", Enabled: true, Position: 3, MatchMode: domain.RuleMatchAny,
		StopProcessing: true, AccountIDs: []string{"a1"},
		Conditions: []domain.RuleCondition{cond}, Actions: []domain.RuleAction{move},
	})
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	rules.rules = []domain.Rule{stored}
	codec := &fakeRuleCodec{}

	data, err := service.Export(context.Background(), codec)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	if string(data) != "encoded" {
		t.Fatalf("export returned %q", data)
	}
	if len(codec.encoded) != 1 {
		t.Fatalf("expected one exported rule, got %d", len(codec.encoded))
	}
	got := codec.encoded[0]
	if got.Name != "Digests" || got.MatchMode != "any" || !got.StopProcessing {
		t.Errorf("rule fields did not survive the export: %+v", got)
	}
	if len(got.Conditions) != 1 || got.Conditions[0].Operator != "endsWith" ||
		!got.Conditions[0].MatchCase || !got.Conditions[0].Not {
		t.Errorf("condition did not survive the export: %+v", got.Conditions)
	}
	// The destination is written as the account and the mailbox path, never as the joined folder id.
	if len(got.Actions) != 1 || got.Actions[0].Account != "a1" || got.Actions[0].Folder != "Filed" {
		t.Errorf("action destination did not split: %+v", got.Actions)
	}
}

// An unscoped rule exports its scope as an empty list rather than as nothing: a nil slice encodes as
// JSON null while a reader expects an array.
func TestExportRulesGivesAnUnscopedRuleAnEmptyAccountList(t *testing.T) {
	service, rules, _, _ := transferHarness(t)
	rules.rules = []domain.Rule{testRule(t, "r1", 0)}
	codec := &fakeRuleCodec{}

	if _, err := service.Export(context.Background(), codec); err != nil {
		t.Fatalf("export: %v", err)
	}

	if codec.encoded[0].Accounts == nil {
		t.Fatal("an unscoped rule exported a nil account list")
	}
	if len(codec.encoded[0].Accounts) != 0 {
		t.Errorf("accounts are %v, want none", codec.encoded[0].Accounts)
	}
}

func TestExportRulesReportsAStoreFailure(t *testing.T) {
	service, rules, _, _ := transferHarness(t)
	rules.listErr = errors.New("boom")
	if _, err := service.Export(context.Background(), &fakeRuleCodec{}); err == nil {
		t.Fatal("expected the store failure to surface")
	}
}

func TestExportRulesReportsAnEncodeFailure(t *testing.T) {
	service, _, _, _ := transferHarness(t)
	if _, err := service.Export(context.Background(), &fakeRuleCodec{encodeErr: errors.New("boom")}); err == nil {
		t.Fatal("expected the encode failure to surface")
	}
}

func TestImportAddsANewRuleAndReplacesOneOfTheSameName(t *testing.T) {
	service, rules, _, _ := transferHarness(t)
	rules.rules = []domain.Rule{testRule(t, "existing-id", 0)} // named "News"
	fresh := transferRule()
	fresh.Name = "Receipts"
	codec := &fakeRuleCodec{decoded: []RuleTransfer{transferRule(), fresh}}

	result, err := service.Import(context.Background(), codec, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if result.Added != 1 || result.Replaced != 1 {
		t.Fatalf("got added %d replaced %d, want 1 and 1", result.Added, result.Replaced)
	}
	if len(rules.saved) != 2 {
		t.Fatalf("expected two saves, got %d", len(rules.saved))
	}
	// The replacement keeps the stored rule's id and position, so re-importing a file updates the rule
	// in place rather than leaving a second copy of it further down the order.
	if rules.saved[0].ID() != "existing-id" || rules.saved[0].Position() != 0 {
		t.Errorf("replacement did not keep its identity: %q at %d", rules.saved[0].ID(), rules.saved[0].Position())
	}
	// The addition lands after the rules already stored.
	if rules.saved[1].ID() != "new-id" || rules.saved[1].Position() != 1 {
		t.Errorf("addition did not land after the stored rules: %q at %d", rules.saved[1].ID(), rules.saved[1].Position())
	}
}

func TestImportDisablesARuleWhoseDestinationIsNotHere(t *testing.T) {
	service, rules, _, _ := transferHarness(t)
	in := transferRule()
	in.Actions = []RuleTransferAction{{Kind: "moveTo", Account: "a1", Folder: "Nowhere"}}
	codec := &fakeRuleCodec{decoded: []RuleTransfer{in}}

	result, err := service.Import(context.Background(), codec, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if len(rules.saved) != 1 || rules.saved[0].Enabled() {
		t.Fatal("a rule whose destination is unknown here was imported switched on")
	}
	if len(result.Disabled) != 1 || result.Disabled[0] != "News" {
		t.Errorf("the disabled rule was not reported: %v", result.Disabled)
	}
	// The action is kept as written, so the rule works once that folder exists.
	if got := rules.saved[0].Actions()[0].FolderID(); got != domain.FolderIDFor("a1", "Nowhere") {
		t.Errorf("the unreachable destination was rewritten to %q", got)
	}
}

func TestImportKeepsARuleEnabledWhenItsDestinationIsHere(t *testing.T) {
	service, rules, _, _ := transferHarness(t)
	in := transferRule()
	in.Actions = []RuleTransferAction{{Kind: "moveTo", Account: "a1", Folder: "Filed"}}
	codec := &fakeRuleCodec{decoded: []RuleTransfer{in}}

	if _, err := service.Import(context.Background(), codec, nil); err != nil {
		t.Fatalf("import: %v", err)
	}

	if !rules.saved[0].Enabled() {
		t.Fatal("a rule whose destination exists here was imported switched off")
	}
}

func TestImportDisablesARuleScopedOnlyToUnknownAccounts(t *testing.T) {
	service, rules, _, _ := transferHarness(t)
	unknown := transferRule()
	unknown.Accounts = []string{"gone@example.com"}
	partly := transferRule()
	partly.Name = "Partly"
	partly.Accounts = []string{"gone@example.com", "a1"}
	codec := &fakeRuleCodec{decoded: []RuleTransfer{unknown, partly}}

	result, err := service.Import(context.Background(), codec, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if rules.saved[0].Enabled() {
		t.Error("a rule scoped only to accounts that are not here was imported switched on")
	}
	// One known account is enough: the rule can act on that one.
	if !rules.saved[1].Enabled() {
		t.Error("a rule naming one account that is here was imported switched off")
	}
	if len(result.Disabled) != 1 || result.Disabled[0] != "News" {
		t.Errorf("the disabled rule was not reported: %v", result.Disabled)
	}
	// The scope is kept verbatim, so the rule starts working when that account is added.
	if got := rules.saved[0].AccountIDs(); len(got) != 1 || got[0] != "gone@example.com" {
		t.Errorf("the unknown scope was rewritten to %v", got)
	}
}

func TestImportKeepsARuleDisabledThatWasExportedDisabled(t *testing.T) {
	service, rules, _, _ := transferHarness(t)
	in := transferRule()
	in.Enabled = false
	codec := &fakeRuleCodec{decoded: []RuleTransfer{in}}

	result, err := service.Import(context.Background(), codec, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if rules.saved[0].Enabled() {
		t.Fatal("a rule exported switched off came back on")
	}
	// It is not REPORTED as disabled: nothing was done to it, so saying so would be noise.
	if len(result.Disabled) != 0 {
		t.Errorf("a rule that was already off was reported as disabled by the import: %v", result.Disabled)
	}
}

func TestPlanDescribesTheImportWithoutWritingAnything(t *testing.T) {
	service, rules, _, _ := transferHarness(t)
	rules.rules = []domain.Rule{testRule(t, "existing-id", 0)} // named "News"
	destroying := transferRule()
	destroying.Name = "Kill it"
	destroying.Actions = []RuleTransferAction{{Kind: "destroy"}}
	unreachable := transferRule()
	unreachable.Name = "Filed elsewhere"
	unreachable.Actions = []RuleTransferAction{{Kind: "moveTo", Account: "a1", Folder: "Nowhere"}}
	codec := &fakeRuleCodec{decoded: []RuleTransfer{transferRule(), destroying, unreachable}}

	plan, err := service.Plan(context.Background(), codec, nil)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if len(rules.saved) != 0 {
		t.Fatal("the plan wrote rules")
	}
	if plan.Total() != 3 {
		t.Errorf("total is %d, want 3", plan.Total())
	}
	if len(plan.Replace) != 1 || plan.Replace[0] != "News" {
		t.Errorf("replacements are %v", plan.Replace)
	}
	if len(plan.Add) != 2 {
		t.Errorf("additions are %v", plan.Add)
	}
	if len(plan.Destructive) != 2 {
		t.Errorf("destructive rules are %v, want the destroy and the move", plan.Destructive)
	}
	if len(plan.Disable) != 1 || plan.Disable[0] != "Filed elsewhere" {
		t.Errorf("disabled rules are %v", plan.Disable)
	}
}

func TestImportRejectsAFileNamingSomethingUnknown(t *testing.T) {
	cases := []struct {
		name string
		vary func(*RuleTransfer)
	}{
		{"field", func(r *RuleTransfer) { r.Conditions[0].Field = "sender" }},
		{"operator", func(r *RuleTransfer) { r.Conditions[0].Operator = "matches" }},
		{"action", func(r *RuleTransfer) { r.Actions[0].Kind = "shred" }},
		{"match mode", func(r *RuleTransfer) { r.MatchMode = "some" }},
		{"empty match text", func(r *RuleTransfer) { r.Conditions[0].Text = "" }},
		{"no name", func(r *RuleTransfer) { r.Name = "" }},
		// A move with half a destination is refused rather than composed into a folder id that names
		// nothing: the file is wrong and saying so is more use than storing a rule that cannot act.
		{"a move naming no folder", func(r *RuleTransfer) { r.Actions[0] = RuleTransferAction{Kind: "moveTo"} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			service, rules, _, _ := transferHarness(t)
			in := transferRule()
			c.vary(&in)
			codec := &fakeRuleCodec{decoded: []RuleTransfer{in}}

			if _, err := service.Import(context.Background(), codec, nil); err == nil {
				t.Fatal("expected the unknown value to be refused")
			}
			if len(rules.saved) != 0 {
				t.Error("a rule was written despite the refusal")
			}
			if _, err := service.Plan(context.Background(), codec, nil); err == nil {
				t.Fatal("expected the plan to refuse it too")
			}
		})
	}
}

func TestImportReportsAFailureFromEachDependency(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(*fakeRuleCodec, *fakeRuleStore, *fakeAccountStore, *fakeMailStore)
	}{
		{"decode", func(c *fakeRuleCodec, _ *fakeRuleStore, _ *fakeAccountStore, _ *fakeMailStore) {
			c.decodeErr = errors.New("boom")
		}},
		{"list rules", func(_ *fakeRuleCodec, r *fakeRuleStore, _ *fakeAccountStore, _ *fakeMailStore) {
			r.listErr = errors.New("boom")
		}},
		{"list accounts", func(_ *fakeRuleCodec, _ *fakeRuleStore, a *fakeAccountStore, _ *fakeMailStore) {
			a.listErr = errors.New("boom")
		}},
		{"list folders", func(_ *fakeRuleCodec, _ *fakeRuleStore, _ *fakeAccountStore, m *fakeMailStore) {
			m.listFoldersErr = errors.New("boom")
		}},
		{"save", func(_ *fakeRuleCodec, r *fakeRuleStore, _ *fakeAccountStore, _ *fakeMailStore) {
			r.saveErr = errors.New("boom")
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			service, rules, accounts, mail := transferHarness(t)
			codec := &fakeRuleCodec{decoded: []RuleTransfer{transferRule()}}
			c.break_(codec, rules, accounts, mail)

			if _, err := service.Import(context.Background(), codec, nil); err == nil {
				t.Fatal("expected the failure to surface from the import")
			}
			if c.name == "save" {
				return // the plan writes nothing, so it cannot meet a save failure
			}
			if _, err := service.Plan(context.Background(), codec, nil); err == nil {
				t.Fatal("expected the failure to surface from the plan")
			}
		})
	}
}
