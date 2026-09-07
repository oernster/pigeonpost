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
		MatchMode: domain.RuleMatchAny, StopProcessing: true, AccountIDs: []string{"a1", "a2"},
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
	if got := r.AccountIDs(); len(got) != 2 || got[0] != "a1" || got[1] != "a2" {
		t.Errorf("account scope not round-tripped: %v", got)
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
		ID: "r1", Name: "Rule", Enabled: true, AccountIDs: []string{"a1"},
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
	base.AccountIDs = nil
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
	// Widening a rule back to every account must clear its scope rows, not leave the old one behind.
	if scope := got[0].AccountIDs(); len(scope) != 0 {
		t.Errorf("account scope not cleared on update: %v", scope)
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
		ID: "r1", Name: "Rule", Enabled: true, AccountIDs: []string{"a1"},
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
	for _, table := range []string{"rule_condition", "rule_action", "rule_account"} {
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
	// A rule written before the scope existed named no account, which must keep meaning every account.
	if scope := r.AccountIDs(); len(scope) != 0 {
		t.Errorf("a carried-over rule gained an account scope: %v", scope)
	}
	actions := r.Actions()
	if len(actions) != 1 || actions[0].Kind() != domain.RuleMarkRead {
		t.Errorf("action not carried over verbatim: %+v", actions)
	}
}

// TestRuleMatchModeMigrationDisarmsOneConditionRules builds a database at the version just before the
// match-mode step, writes a one-condition rule on "any" plus a two-condition rule on "any", then opens
// it normally so the migration runs. The one-condition rule must come back on "all" (identical
// behaviour today; a second condition will now narrow it rather than widen it); the two-condition
// rule must be left exactly as it was, because its mode is a choice someone made.
func TestRuleMatchModeMigrationDisarmsOneConditionRules(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "modes.db")
	db, err := sql.Open(driverName, path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// preMatchModeVersion is the version schemaV55 upgrades FROM. It is a fixed number rather than an
	// offset from schemaVersion, so later migrations cannot move this test off its step.
	const preMatchModeVersion = 54
	for _, step := range migrations[:preMatchModeVersion] {
		if _, err := db.ExecContext(ctx, step); err != nil {
			t.Fatalf("apply migrations up to %d: %v", preMatchModeVersion, err)
		}
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d;", preMatchModeVersion)); err != nil {
		t.Fatalf("set version: %v", err)
	}
	insertRule := func(id string, position int, conditions ...string) {
		t.Helper()
		if _, err := db.ExecContext(ctx,
			"INSERT INTO rule (id, name, enabled, position, match_mode, stop_processing) VALUES (?, ?, 1, ?, ?, 0);",
			id, id, position, int(domain.RuleMatchAny)); err != nil {
			t.Fatalf("insert rule %q: %v", id, err)
		}
		for i, text := range conditions {
			if _, err := db.ExecContext(ctx,
				`INSERT INTO rule_condition (rule_id, position, field, operator, match_text, case_sensitive)
				 VALUES (?, ?, ?, ?, ?, 0);`,
				id, i, int(domain.RuleFieldAll), int(domain.RuleOpContains), text); err != nil {
				t.Fatalf("insert condition for %q: %v", id, err)
			}
		}
		if _, err := db.ExecContext(ctx,
			"INSERT INTO rule_action (rule_id, position, kind, folder_id) VALUES (?, 0, ?, '');",
			id, int(domain.RuleFlag)); err != nil {
			t.Fatalf("insert action for %q: %v", id, err)
		}
	}
	insertRule("single", 0, "shop")
	insertRule("pair", 1, "shop", "store")
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open store (migration failed): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	rules, err := store.ListRules(ctx)
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	modes := map[string]domain.RuleMatchMode{}
	for _, r := range rules {
		modes[r.ID()] = r.MatchMode()
	}
	if modes["single"] != domain.RuleMatchAll {
		t.Errorf("a one-condition rule was left on any, so its next condition widens it")
	}
	if modes["pair"] != domain.RuleMatchAny {
		t.Errorf("a two-condition rule had its mode rewritten, changing what it matches")
	}
}

// TestNegateMigrationRewritesTheRetiredOperator builds a database at the version just before negation
// became its own flag, writes a not-contains condition in the old spelling, then opens it normally so
// the migration runs. The condition must come back as contains-and-negated: the same test, under the
// spelling every operator now shares.
func TestNegateMigrationRewritesTheRetiredOperator(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "negate.db")
	db, err := sql.Open(driverName, path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// preNegateVersion is the version schemaV56 upgrades FROM. It is a fixed number rather than an
	// offset from schemaVersion, so later migrations cannot move this test off its step.
	const preNegateVersion = 55
	for _, step := range migrations[:preNegateVersion] {
		if _, err := db.ExecContext(ctx, step); err != nil {
			t.Fatalf("apply migrations up to %d: %v", preNegateVersion, err)
		}
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d;", preNegateVersion)); err != nil {
		t.Fatalf("set version: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO rule (id, name, enabled, position, match_mode, stop_processing) VALUES ('r1', 'Music', 1, 0, ?, 0);",
		int(domain.RuleMatchAny)); err != nil {
		t.Fatalf("insert rule: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO rule_condition (rule_id, position, field, operator, match_text, case_sensitive)
		 VALUES ('r1', 0, ?, ?, 'mediamonkey', 0);`,
		int(domain.RuleFieldAll), int(domain.RuleOpNotContains)); err != nil {
		t.Fatalf("insert condition: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO rule_action (rule_id, position, kind, folder_id) VALUES ('r1', 0, ?, '');",
		int(domain.RuleFlag)); err != nil {
		t.Fatalf("insert action: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open store (migration failed): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	rules, err := store.ListRules(ctx)
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 1 || len(rules[0].Conditions()) != 1 {
		t.Fatalf("expected one rule with one condition, got %d", len(rules))
	}
	got := rules[0].Conditions()[0]
	if !got.Negated() {
		t.Error("the migrated condition is not negated, so the rule stopped excluding what it excluded")
	}
	if got.Operator() != domain.RuleOpContains {
		t.Errorf("operator is %v, want contains", got.Operator())
	}
}

// A negated condition survives a save and a read: the flag is stored beside the comparison rather
// than folded back into an operator that only one comparison has.
func TestRuleStoreRoundTripsANegatedCondition(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	cond, err := domain.NewRuleConditionFull(domain.RuleFieldSubject, domain.RuleOpEquals, "digest", false, true)
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	action, err := domain.NewRuleAction(domain.RuleFlag, "")
	if err != nil {
		t.Fatalf("action: %v", err)
	}
	rule, err := domain.NewRule(domain.RuleSpec{
		ID: "r1", Name: "Not the digest", Enabled: true,
		MatchMode: domain.RuleMatchAll, Conditions: []domain.RuleCondition{cond},
		Actions: []domain.RuleAction{action},
	})
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	if err := store.SaveRule(ctx, rule); err != nil {
		t.Fatalf("save rule: %v", err)
	}

	rules, err := store.ListRules(ctx)
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 1 || len(rules[0].Conditions()) != 1 {
		t.Fatalf("expected one rule with one condition, got %d", len(rules))
	}
	got := rules[0].Conditions()[0]
	if !got.Negated() || got.Operator() != domain.RuleOpEquals {
		t.Errorf("read back operator %v negated %v, want equals negated", got.Operator(), got.Negated())
	}
}
