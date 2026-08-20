package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// storeCondition builds a condition for the rule-store tests.
func storeCondition(t *testing.T, field domain.RuleField, op domain.RuleOperator, text string) domain.RuleCondition {
	t.Helper()
	c, err := domain.NewRuleCondition(field, op, text)
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	return c
}

// storeCasedCondition builds a case-sensitive condition for the rule-store tests.
func storeCasedCondition(t *testing.T, field domain.RuleField, op domain.RuleOperator, text string) domain.RuleCondition {
	t.Helper()
	c, err := domain.NewRuleConditionCased(field, op, text, true)
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	return c
}

// storeAction builds an action for the rule-store tests.
func storeAction(t *testing.T, kind domain.RuleActionKind, folderID string) domain.RuleAction {
	t.Helper()
	a, err := domain.NewRuleAction(kind, folderID)
	if err != nil {
		t.Fatalf("action: %v", err)
	}
	return a
}

// storeRule builds a rule for the rule-store tests.
func storeRule(t *testing.T, spec domain.RuleSpec) domain.Rule {
	t.Helper()
	r, err := domain.NewRule(spec)
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	return r
}

// TestRuleStoreRoundTrip saves a rule carrying several conditions and several actions and reads it
// back whole, which is the property the child tables exist to provide.
func TestRuleStoreRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	want := storeRule(t, domain.RuleSpec{
		ID: "r1", Name: "Receipts", Enabled: true, Position: 2,
		MatchMode: domain.RuleMatchAny, StopProcessing: true,
		Conditions: []domain.RuleCondition{
			storeCondition(t, domain.RuleFieldSenderDomain, domain.RuleOpEquals, "shop.com"),
			storeCasedCondition(t, domain.RuleFieldSubject, domain.RuleOpContains, "invoice"),
		},
		Actions: []domain.RuleAction{
			storeAction(t, domain.RuleMarkRead, ""),
			storeAction(t, domain.RuleMoveTo, "f2"),
		},
	})
	if err := store.SaveRule(ctx, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := store.ListRules(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rules, want 1", len(got))
	}
	r := got[0]
	if r.ID() != "r1" || r.Name() != "Receipts" || !r.Enabled() || r.Position() != 2 ||
		r.MatchMode() != domain.RuleMatchAny || !r.StopProcessing() {
		t.Errorf("rule row not round-tripped: %+v", r)
	}
	conditions := r.Conditions()
	if len(conditions) != 2 || conditions[0].Field() != domain.RuleFieldSenderDomain ||
		conditions[0].Text() != "shop.com" || conditions[1].Operator() != domain.RuleOpContains {
		t.Errorf("conditions not round-tripped in order: %+v", conditions)
	}
	if conditions[0].CaseSensitive() || !conditions[1].CaseSensitive() {
		t.Errorf("case-sensitivity not round-tripped: %v / %v",
			conditions[0].CaseSensitive(), conditions[1].CaseSensitive())
	}
	actions := r.Actions()
	if len(actions) != 2 || actions[0].Kind() != domain.RuleMarkRead ||
		actions[1].Kind() != domain.RuleMoveTo || actions[1].FolderID() != "f2" {
		t.Errorf("actions not round-tripped in order: %+v", actions)
	}
}

// TestRuleStoreUpdateReplacesChildren pins that saving a rule again replaces its conditions and
// actions outright rather than appending to them.
func TestRuleStoreUpdateReplacesChildren(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	base := domain.RuleSpec{
		ID: "r1", Name: "Rule", Enabled: true,
		Conditions: []domain.RuleCondition{
			storeCondition(t, domain.RuleFieldFrom, domain.RuleOpContains, "a"),
			storeCondition(t, domain.RuleFieldFrom, domain.RuleOpContains, "b"),
		},
		Actions: []domain.RuleAction{storeAction(t, domain.RuleFlag, "")},
	}
	if err := store.SaveRule(ctx, storeRule(t, base)); err != nil {
		t.Fatalf("first save: %v", err)
	}
	base.Name = "Renamed"
	base.Enabled = false
	base.Conditions = []domain.RuleCondition{storeCondition(t, domain.RuleFieldSubject, domain.RuleOpEquals, "c")}
	base.Actions = []domain.RuleAction{storeAction(t, domain.RuleDestroy, "")}
	if err := store.SaveRule(ctx, storeRule(t, base)); err != nil {
		t.Fatalf("second save: %v", err)
	}
	got, err := store.ListRules(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rules, want 1", len(got))
	}
	if got[0].Name() != "Renamed" || got[0].Enabled() {
		t.Errorf("rule row not updated: %+v", got[0])
	}
	if len(got[0].Conditions()) != 1 || len(got[0].Actions()) != 1 {
		t.Errorf("children appended rather than replaced: %d conditions, %d actions",
			len(got[0].Conditions()), len(got[0].Actions()))
	}
	if got[0].Actions()[0].Kind() != domain.RuleDestroy {
		t.Errorf("action not replaced: %v", got[0].Actions()[0].Kind())
	}
}

// TestRuleStoreListOrder pins the evaluation order the list must hand back: position first, then name.
func TestRuleStoreListOrder(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	spec := func(id, name string, position int) domain.RuleSpec {
		return domain.RuleSpec{
			ID: id, Name: name, Enabled: true, Position: position,
			Conditions: []domain.RuleCondition{storeCondition(t, domain.RuleFieldFrom, domain.RuleOpContains, "x")},
			Actions:    []domain.RuleAction{storeAction(t, domain.RuleFlag, "")},
		}
	}
	for _, s := range []domain.RuleSpec{spec("c", "Zed", 1), spec("a", "Bee", 0), spec("b", "Ant", 0)} {
		if err := store.SaveRule(ctx, storeRule(t, s)); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	got, err := store.ListRules(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"b", "a", "c"}
	for i, id := range want {
		if got[i].ID() != id {
			t.Fatalf("order = %v, want %v", ruleIDs(got), want)
		}
	}
}

// TestRuleStoreDeleteRemovesChildren pins that deleting a rule takes its conditions and actions with
// it, so a later rule reusing the id cannot inherit them.
func TestRuleStoreDeleteRemovesChildren(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	rule := storeRule(t, domain.RuleSpec{
		ID: "r1", Name: "Rule", Enabled: true,
		Conditions: []domain.RuleCondition{storeCondition(t, domain.RuleFieldFrom, domain.RuleOpContains, "a")},
		Actions:    []domain.RuleAction{storeAction(t, domain.RuleDestroy, "")},
	})
	if err := store.SaveRule(ctx, rule); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.DeleteRule(ctx, "r1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, err := store.ListRules(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d rules, want none", len(got))
	}
	for _, table := range []string{"rule_condition", "rule_action"} {
		var count int
		if err := store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+";").Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s left %d orphaned row(s)", table, count)
		}
	}
}

// ruleIDs lists the ids of the given rules, for a readable failure message.
func ruleIDs(rules []domain.Rule) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		out = append(out, r.ID())
	}
	return out
}

// TestRuleMigrationCarriesLegacyRules builds a database at the schema version just before the rule set
// was rebuilt, writes a rule in the old flat shape, then opens it normally so the migration runs. The
// rule must come back as an equivalent one-condition, one-action rule, because a filter silently
// changing behaviour across an update is worse than one that fails loudly.
func TestRuleMigrationCarriesLegacyRules(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open(driverName, path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	// The handle is closed explicitly below before the store reopens the file; this guards the failure
	// paths, which would otherwise leave it open and block the temp-directory cleanup on Windows.
	t.Cleanup(func() { _ = db.Close() })
	// legacyRuleVersion is the version schemaV50 upgrades FROM, the last one where rules still lived in
	// the flat rule table. It is a fixed number rather than an offset from schemaVersion, so later
	// migrations do not quietly move this test off the shape it exists to cover.
	const legacyRuleVersion = 49
	for _, step := range migrations[:legacyRuleVersion] {
		if _, err := db.ExecContext(ctx, step); err != nil {
			t.Fatalf("apply legacy migrations: %v", err)
		}
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d;", legacyRuleVersion)); err != nil {
		t.Fatalf("set legacy version: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO rule (id, name, field, operator, contains, action) VALUES (?, ?, ?, ?, ?, ?);",
		"old-1", "Newsletters", int(domain.RuleFieldSubject), int(domain.RuleOpEndsWith),
		"digest", int(domain.RuleMarkRead)); err != nil {
		t.Fatalf("insert legacy rule: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open store (migration failed): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	got, err := store.ListRules(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rules, want the carried-over one", len(got))
	}
	r := got[0]
	if r.ID() != "old-1" || r.Name() != "Newsletters" {
		t.Errorf("identity lost: %q / %q", r.ID(), r.Name())
	}
	if !r.Enabled() {
		t.Errorf("a carried-over rule must stay switched on")
	}
	if r.MatchMode() != domain.RuleMatchAll || r.StopProcessing() {
		t.Errorf("carried-over defaults wrong: %v / %v", r.MatchMode(), r.StopProcessing())
	}
	conditions := r.Conditions()
	if len(conditions) != 1 || conditions[0].Field() != domain.RuleFieldSubject ||
		conditions[0].Operator() != domain.RuleOpEndsWith || conditions[0].Text() != "digest" {
		t.Errorf("condition not carried over verbatim: %+v", conditions)
	}
	// Rules written before the flag existed compared case-insensitively, so a carried-over one must
	// keep doing exactly that rather than silently tightening.
	if conditions[0].CaseSensitive() {
		t.Errorf("a carried-over condition became case-sensitive")
	}
	actions := r.Actions()
	if len(actions) != 1 || actions[0].Kind() != domain.RuleMarkRead {
		t.Errorf("action not carried over verbatim: %+v", actions)
	}
}
