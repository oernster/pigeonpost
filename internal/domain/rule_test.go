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

func TestNewRule(t *testing.T) {
	r, err := NewRule("  r1  ", "  Newsletters  ", RuleFieldFrom, RuleOpContains, "  news@  ", RuleMarkRead)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID() != "r1" || r.Name() != "Newsletters" || r.Contains() != "news@" {
		t.Errorf("fields not trimmed: %+v", r)
	}
	if r.Field() != RuleFieldFrom || r.Operator() != RuleOpContains || r.Action() != RuleMarkRead {
		t.Errorf("field/operator/action wrong: %v / %v / %v", r.Field(), r.Operator(), r.Action())
	}
}

func TestNewRuleInvalid(t *testing.T) {
	cases := map[string]struct {
		id, name string
		field    RuleField
		operator RuleOperator
		match    string
		action   RuleAction
		want     error
	}{
		"empty id":         {"", "n", RuleFieldFrom, RuleOpContains, "x", RuleMarkRead, ErrEmptyRuleID},
		"empty name":       {"r", "", RuleFieldFrom, RuleOpContains, "x", RuleMarkRead, ErrEmptyRuleName},
		"empty match":      {"r", "n", RuleFieldFrom, RuleOpContains, "  ", RuleMarkRead, ErrEmptyRuleMatch},
		"invalid field":    {"r", "n", RuleField(9), RuleOpContains, "x", RuleMarkRead, ErrInvalidRuleField},
		"invalid operator": {"r", "n", RuleFieldFrom, RuleOperator(9), "x", RuleMarkRead, ErrInvalidRuleOperator},
		"invalid action":   {"r", "n", RuleFieldFrom, RuleOpContains, "x", RuleAction(9), ErrInvalidRuleAction},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewRule(tc.id, tc.name, tc.field, tc.operator, tc.match, tc.action); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestRuleFieldString(t *testing.T) {
	cases := map[RuleField]string{
		RuleFieldFrom: "from", RuleFieldSubject: "subject", RuleFieldTo: "to", RuleFieldCc: "cc",
		RuleField(9): "unknown",
	}
	for f, want := range cases {
		if f.String() != want {
			t.Errorf("RuleField(%d).String() = %q, want %q", f, f.String(), want)
		}
	}
	for _, f := range []RuleField{RuleFieldFrom, RuleFieldSubject, RuleFieldTo, RuleFieldCc} {
		if !f.Valid() {
			t.Errorf("RuleField %v should be valid", f)
		}
	}
	if RuleField(9).Valid() || RuleField(-1).Valid() {
		t.Error("out-of-range fields should be invalid")
	}
}

func TestRuleOperatorString(t *testing.T) {
	cases := map[RuleOperator]string{
		RuleOpContains: "contains", RuleOpNotContains: "notContains", RuleOpEquals: "equals",
		RuleOpStartsWith: "startsWith", RuleOpEndsWith: "endsWith", RuleOperator(9): "unknown",
	}
	for o, want := range cases {
		if o.String() != want {
			t.Errorf("RuleOperator(%d).String() = %q, want %q", o, o.String(), want)
		}
	}
	for _, o := range []RuleOperator{RuleOpContains, RuleOpNotContains, RuleOpEquals, RuleOpStartsWith, RuleOpEndsWith} {
		if !o.Valid() {
			t.Errorf("RuleOperator %v should be valid", o)
		}
	}
	if RuleOperator(9).Valid() || RuleOperator(-1).Valid() {
		t.Error("out-of-range operators should be invalid")
	}
}

func TestRuleActionString(t *testing.T) {
	if RuleMarkRead.String() != "markRead" || RuleFlag.String() != "flag" || RuleAction(9).String() != "unknown" {
		t.Error("RuleAction.String wrong")
	}
	if !RuleMarkRead.Valid() || !RuleFlag.Valid() || RuleAction(9).Valid() {
		t.Error("RuleAction.Valid wrong")
	}
}

func TestRuleMatchesOperators(t *testing.T) {
	msg := ruleMessage(t, "The Boss", "boss@corp.com", "Your Invoice is ready")

	cases := []struct {
		name     string
		field    RuleField
		operator RuleOperator
		match    string
		want     bool
	}{
		{"from contains", RuleFieldFrom, RuleOpContains, "BOSS", true},
		{"from not-contains hit", RuleFieldFrom, RuleOpNotContains, "boss", false},
		{"from not-contains miss", RuleFieldFrom, RuleOpNotContains, "nobody", true},
		{"from equals address", RuleFieldFrom, RuleOpEquals, "boss@corp.com", true},
		{"from equals miss", RuleFieldFrom, RuleOpEquals, "the bos", false},
		{"subject contains", RuleFieldSubject, RuleOpContains, "invoice", true},
		{"subject starts-with", RuleFieldSubject, RuleOpStartsWith, "your", true},
		{"subject starts-with miss", RuleFieldSubject, RuleOpStartsWith, "invoice", false},
		{"subject ends-with", RuleFieldSubject, RuleOpEndsWith, "ready", true},
		{"subject ends-with miss", RuleFieldSubject, RuleOpEndsWith, "invoice", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewRule("r", "n", tc.field, tc.operator, tc.match, RuleFlag)
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}
			if got := r.Matches(msg); got != tc.want {
				t.Errorf("Matches = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRuleMatchesRecipients(t *testing.T) {
	msg := ruleMessageWithRecipients(t,
		[]EmailAddress{ruleAddress(t, "Alice", "alice@team.com"), ruleAddress(t, "Bob", "bob@team.com")},
		[]EmailAddress{ruleAddress(t, "Carol", "carol@other.com")})

	toRule, _ := NewRule("r", "n", RuleFieldTo, RuleOpContains, "bob@team.com", RuleFlag)
	if !toRule.Matches(msg) {
		t.Error("to rule should match a recipient address")
	}
	toName, _ := NewRule("r", "n", RuleFieldTo, RuleOpEquals, "Alice", RuleFlag)
	if !toName.Matches(msg) {
		t.Error("to rule should match a recipient display name")
	}
	ccRule, _ := NewRule("r", "n", RuleFieldCc, RuleOpEndsWith, "@other.com", RuleFlag)
	if !ccRule.Matches(msg) {
		t.Error("cc rule should match on the cc list")
	}
	toMiss, _ := NewRule("r", "n", RuleFieldTo, RuleOpContains, "carol", RuleFlag)
	if toMiss.Matches(msg) {
		t.Error("to rule should not match a cc-only recipient")
	}
	// "Does not contain" over an empty recipient list is vacuously true.
	noCc := ruleMessageWithRecipients(t, []EmailAddress{ruleAddress(t, "Dan", "dan@team.com")}, nil)
	ccNone, _ := NewRule("r", "n", RuleFieldCc, RuleOpNotContains, "anyone", RuleFlag)
	if !ccNone.Matches(noCc) {
		t.Error("not-contains over an empty cc list should match")
	}
}

func TestApplyRules(t *testing.T) {
	msg := ruleMessage(t, "News", "news@example.com", "Weekly digest")

	same := ApplyRules([]MessageSummary{msg}, nil)
	if same[0].IsRead() || same[0].IsFlagged() {
		t.Error("no rules should not change flags")
	}

	markRead, _ := NewRule("r1", "read news", RuleFieldFrom, RuleOpContains, "news@", RuleMarkRead)
	flag, _ := NewRule("r2", "flag digest", RuleFieldSubject, RuleOpContains, "digest", RuleFlag)
	out := ApplyRules([]MessageSummary{msg}, []Rule{markRead, flag})
	if !out[0].IsRead() {
		t.Error("markRead rule should set Seen")
	}
	if !out[0].IsFlagged() {
		t.Error("flag rule should set Flagged")
	}
	if msg.IsRead() || msg.IsFlagged() {
		t.Error("ApplyRules must not mutate the input message")
	}

	other := ruleMessage(t, "Friend", "friend@example.com", "lunch?")
	res := ApplyRules([]MessageSummary{other}, []Rule{markRead, flag})
	if res[0].IsRead() || res[0].IsFlagged() {
		t.Error("non-matching message should be unchanged")
	}
}
