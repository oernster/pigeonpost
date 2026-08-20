package domain

import (
	"errors"
	"testing"
)

func ruleAddress(t *testing.T, name, addr string) EmailAddress {
	t.Helper()
	a, err := NewEmailAddress(name, addr)
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	return a
}

func ruleMessage(t *testing.T, fromName, fromAddr, subject string) MessageSummary {
	t.Helper()
	m, err := NewMessageSummary(MessageSummaryInput{
		ID: "m1", FolderID: "f1", UID: "1", From: ruleAddress(t, fromName, fromAddr),
		Subject: subject, Size: 1, Flags: NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return m
}

func ruleMessageWithRecipients(t *testing.T, to, cc []EmailAddress) MessageSummary {
	t.Helper()
	m, err := NewMessageSummary(MessageSummaryInput{
		ID: "m1", FolderID: "f1", UID: "1", From: ruleAddress(t, "Sender", "sender@x.com"),
		To: to, Cc: cc, Subject: "hi", Size: 1, Flags: NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return m
}

// mustCondition builds a condition the test expects to be valid.
func mustCondition(t *testing.T, field RuleField, op RuleOperator, text string) RuleCondition {
	t.Helper()
	c, err := NewRuleCondition(field, op, text)
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	return c
}

// mustAction builds an action the test expects to be valid.
func mustAction(t *testing.T, kind RuleActionKind, folderID string) RuleAction {
	t.Helper()
	a, err := NewRuleAction(kind, folderID)
	if err != nil {
		t.Fatalf("action: %v", err)
	}
	return a
}

// mustRule builds a rule the test expects to be valid.
func mustRule(t *testing.T, spec RuleSpec) Rule {
	t.Helper()
	if spec.ID == "" {
		spec.ID = "r1"
	}
	if spec.Name == "" {
		spec.Name = "Rule"
	}
	r, err := NewRule(spec)
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	return r
}

func TestNewRuleCondition(t *testing.T) {
	c := mustCondition(t, RuleFieldFrom, RuleOpContains, "  news@  ")
	if c.Text() != "news@" {
		t.Errorf("text not trimmed: %q", c.Text())
	}
	if c.Field() != RuleFieldFrom || c.Operator() != RuleOpContains {
		t.Errorf("field or operator wrong: %v / %v", c.Field(), c.Operator())
	}
}

func TestNewRuleConditionInvalid(t *testing.T) {
	cases := map[string]struct {
		field    RuleField
		operator RuleOperator
		text     string
		want     error
	}{
		"empty text":       {RuleFieldFrom, RuleOpContains, "   ", ErrEmptyRuleMatch},
		"bad field":        {RuleField(99), RuleOpContains, "x", ErrInvalidRuleField},
		"negative field":   {RuleField(-1), RuleOpContains, "x", ErrInvalidRuleField},
		"bad operator":     {RuleFieldFrom, RuleOperator(99), "x", ErrInvalidRuleOperator},
		"negative operatr": {RuleFieldFrom, RuleOperator(-1), "x", ErrInvalidRuleOperator},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewRuleCondition(c.field, c.operator, c.text); !errors.Is(err, c.want) {
				t.Errorf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestNewRuleAction(t *testing.T) {
	a := mustAction(t, RuleMoveTo, "  folder-1  ")
	if a.Kind() != RuleMoveTo || a.FolderID() != "folder-1" {
		t.Errorf("move action wrong: %v / %q", a.Kind(), a.FolderID())
	}
	// A non-move kind carries no destination, even when one is supplied.
	if got := mustAction(t, RuleDestroy, "folder-1"); got.FolderID() != "" {
		t.Errorf("destroy kept a folder: %q", got.FolderID())
	}
}

func TestNewRuleActionInvalid(t *testing.T) {
	if _, err := NewRuleAction(RuleActionKind(99), ""); !errors.Is(err, ErrInvalidRuleAction) {
		t.Errorf("bad kind accepted")
	}
	if _, err := NewRuleAction(RuleActionKind(-1), ""); !errors.Is(err, ErrInvalidRuleAction) {
		t.Errorf("negative kind accepted")
	}
	if _, err := NewRuleAction(RuleMoveTo, "  "); !errors.Is(err, ErrMissingRuleFolder) {
		t.Errorf("move without a destination accepted")
	}
}

func TestNewRule(t *testing.T) {
	r := mustRule(t, RuleSpec{
		ID: "  r1  ", Name: "  Newsletters  ", Enabled: true, Position: 3,
		MatchMode: RuleMatchAny, StopProcessing: true,
		Conditions: []RuleCondition{mustCondition(t, RuleFieldFrom, RuleOpContains, "news@")},
		Actions:    []RuleAction{mustAction(t, RuleMarkRead, "")},
	})
	if r.ID() != "r1" || r.Name() != "Newsletters" {
		t.Errorf("id or name not trimmed: %q / %q", r.ID(), r.Name())
	}
	if !r.Enabled() || r.Position() != 3 || r.MatchMode() != RuleMatchAny || !r.StopProcessing() {
		t.Errorf("rule settings wrong: %+v", r)
	}
	if len(r.Conditions()) != 1 || len(r.Actions()) != 1 {
		t.Errorf("children wrong: %d conditions, %d actions", len(r.Conditions()), len(r.Actions()))
	}
}

func TestNewRuleCopiesChildren(t *testing.T) {
	conditions := []RuleCondition{mustCondition(t, RuleFieldFrom, RuleOpContains, "a")}
	actions := []RuleAction{mustAction(t, RuleFlag, "")}
	r := mustRule(t, RuleSpec{Conditions: conditions, Actions: actions})
	conditions[0] = mustCondition(t, RuleFieldSubject, RuleOpEquals, "b")
	actions[0] = mustAction(t, RuleDestroy, "")
	if r.Conditions()[0].Field() != RuleFieldFrom || r.Actions()[0].Kind() != RuleFlag {
		t.Errorf("rule shares its caller's slices")
	}
	// The accessors hand out copies too.
	r.Conditions()[0] = mustCondition(t, RuleFieldSubject, RuleOpEquals, "b")
	if r.Conditions()[0].Field() != RuleFieldFrom {
		t.Errorf("Conditions() exposed the rule's own slice")
	}
	r.Actions()[0] = mustAction(t, RuleDestroy, "")
	if r.Actions()[0].Kind() != RuleFlag {
		t.Errorf("Actions() exposed the rule's own slice")
	}
}

func TestNewRuleInvalid(t *testing.T) {
	good := []RuleCondition{mustCondition(t, RuleFieldFrom, RuleOpContains, "a")}
	goodAct := []RuleAction{mustAction(t, RuleFlag, "")}
	cases := map[string]struct {
		spec RuleSpec
		want error
	}{
		"empty id":     {RuleSpec{ID: "  ", Name: "n", Conditions: good, Actions: goodAct}, ErrEmptyRuleID},
		"empty name":   {RuleSpec{ID: "r", Name: "  ", Conditions: good, Actions: goodAct}, ErrEmptyRuleName},
		"bad mode":     {RuleSpec{ID: "r", Name: "n", MatchMode: RuleMatchMode(9), Conditions: good, Actions: goodAct}, ErrInvalidRuleMatchMode},
		"no condition": {RuleSpec{ID: "r", Name: "n", Actions: goodAct}, ErrNoRuleConditions},
		"no action":    {RuleSpec{ID: "r", Name: "n", Conditions: good}, ErrNoRuleActions},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewRule(c.spec); !errors.Is(err, c.want) {
				t.Errorf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestRuleTokens(t *testing.T) {
	fields := map[RuleField]string{
		RuleFieldFrom: "from", RuleFieldSubject: "subject", RuleFieldTo: "to", RuleFieldCc: "cc",
		RuleFieldAnyRecipient: "anyRecipient", RuleFieldSenderDomain: "senderDomain", RuleFieldAll: "all",
		RuleField(99): "unknown",
	}
	for field, want := range fields {
		if got := field.String(); got != want {
			t.Errorf("field %d: got %q, want %q", field, got, want)
		}
	}
	operators := map[RuleOperator]string{
		RuleOpContains: "contains", RuleOpNotContains: "notContains", RuleOpEquals: "equals",
		RuleOpStartsWith: "startsWith", RuleOpEndsWith: "endsWith", RuleOperator(99): "unknown",
	}
	for op, want := range operators {
		if got := op.String(); got != want {
			t.Errorf("operator %d: got %q, want %q", op, got, want)
		}
	}
	kinds := map[RuleActionKind]string{
		RuleMarkRead: "markRead", RuleFlag: "flag", RuleMoveTo: "moveTo", RuleDestroy: "destroy",
		RuleActionKind(99): "unknown",
	}
	for kind, want := range kinds {
		if got := kind.String(); got != want {
			t.Errorf("kind %d: got %q, want %q", kind, got, want)
		}
	}
	modes := map[RuleMatchMode]string{RuleMatchAll: "all", RuleMatchAny: "any", RuleMatchMode(9): "unknown"}
	for mode, want := range modes {
		if got := mode.String(); got != want {
			t.Errorf("mode %d: got %q, want %q", mode, got, want)
		}
	}
}

func TestRuleActionKindDestructive(t *testing.T) {
	cases := map[RuleActionKind]bool{
		RuleMarkRead: false, RuleFlag: false, RuleMoveTo: true, RuleDestroy: true,
	}
	for kind, want := range cases {
		if got := kind.Destructive(); got != want {
			t.Errorf("kind %v destructive = %v, want %v", kind, got, want)
		}
	}
}

func TestRuleDestructive(t *testing.T) {
	flagOnly := mustRule(t, RuleSpec{
		Conditions: []RuleCondition{mustCondition(t, RuleFieldFrom, RuleOpContains, "a")},
		Actions:    []RuleAction{mustAction(t, RuleMarkRead, ""), mustAction(t, RuleFlag, "")},
	})
	if flagOnly.Destructive() {
		t.Errorf("flag-only rule reported destructive")
	}
	withMove := mustRule(t, RuleSpec{
		Conditions: []RuleCondition{mustCondition(t, RuleFieldFrom, RuleOpContains, "a")},
		Actions:    []RuleAction{mustAction(t, RuleMarkRead, ""), mustAction(t, RuleMoveTo, "f2")},
	})
	if !withMove.Destructive() {
		t.Errorf("rule with a move not reported destructive")
	}
}
