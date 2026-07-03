package domain

import (
	"errors"
	"testing"
)

func ruleMessage(t *testing.T, fromName, fromAddr, subject string) MessageSummary {
	t.Helper()
	from, err := NewEmailAddress(fromName, fromAddr)
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	m, err := NewMessageSummary(MessageSummaryInput{
		ID: "m1", FolderID: "f1", UID: 1, From: from, Subject: subject, Size: 1, Flags: NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	return m
}

func TestNewRule(t *testing.T) {
	r, err := NewRule("  r1  ", "  Newsletters  ", RuleFieldFrom, "  news@  ", RuleMarkRead)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID() != "r1" || r.Name() != "Newsletters" || r.Contains() != "news@" {
		t.Errorf("fields not trimmed: %+v", r)
	}
	if r.Field() != RuleFieldFrom || r.Action() != RuleMarkRead {
		t.Errorf("field/action wrong: %v / %v", r.Field(), r.Action())
	}
}

func TestNewRuleInvalid(t *testing.T) {
	cases := map[string]struct {
		id, name string
		field    RuleField
		match    string
		action   RuleAction
		want     error
	}{
		"empty id":       {"", "n", RuleFieldFrom, "x", RuleMarkRead, ErrEmptyRuleID},
		"empty name":     {"r", "", RuleFieldFrom, "x", RuleMarkRead, ErrEmptyRuleName},
		"empty match":    {"r", "n", RuleFieldFrom, "  ", RuleMarkRead, ErrEmptyRuleMatch},
		"invalid field":  {"r", "n", RuleField(9), "x", RuleMarkRead, ErrInvalidRuleField},
		"invalid action": {"r", "n", RuleFieldFrom, "x", RuleAction(9), ErrInvalidRuleAction},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewRule(tc.id, tc.name, tc.field, tc.match, tc.action); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestRuleFieldAndAction(t *testing.T) {
	if RuleFieldFrom.String() != "from" || RuleFieldSubject.String() != "subject" || RuleField(9).String() != "unknown" {
		t.Error("RuleField.String wrong")
	}
	if !RuleFieldFrom.Valid() || !RuleFieldSubject.Valid() || RuleField(9).Valid() {
		t.Error("RuleField.Valid wrong")
	}
	if RuleMarkRead.String() != "markRead" || RuleFlag.String() != "flag" || RuleAction(9).String() != "unknown" {
		t.Error("RuleAction.String wrong")
	}
	if !RuleMarkRead.Valid() || !RuleFlag.Valid() || RuleAction(9).Valid() {
		t.Error("RuleAction.Valid wrong")
	}
}

func TestRuleMatches(t *testing.T) {
	fromRule, _ := NewRule("r", "n", RuleFieldFrom, "BOSS", RuleFlag)
	subjRule, _ := NewRule("r", "n", RuleFieldSubject, "invoice", RuleMarkRead)
	msg := ruleMessage(t, "The Boss", "boss@corp.com", "Your Invoice is ready")

	if !fromRule.Matches(msg) {
		t.Error("from rule should match case-insensitively on sender")
	}
	if !subjRule.Matches(msg) {
		t.Error("subject rule should match case-insensitively on subject")
	}
	miss, _ := NewRule("r", "n", RuleFieldFrom, "nobody", RuleFlag)
	if miss.Matches(msg) {
		t.Error("non-matching rule should not match")
	}
}

func TestApplyRules(t *testing.T) {
	msg := ruleMessage(t, "News", "news@example.com", "Weekly digest")

	// No rules leaves the slice untouched.
	same := ApplyRules([]MessageSummary{msg}, nil)
	if same[0].IsRead() || same[0].IsFlagged() {
		t.Error("no rules should not change flags")
	}

	markRead, _ := NewRule("r1", "read news", RuleFieldFrom, "news@", RuleMarkRead)
	flag, _ := NewRule("r2", "flag digest", RuleFieldSubject, "digest", RuleFlag)
	out := ApplyRules([]MessageSummary{msg}, []Rule{markRead, flag})
	if !out[0].IsRead() {
		t.Error("markRead rule should set Seen")
	}
	if !out[0].IsFlagged() {
		t.Error("flag rule should set Flagged")
	}
	// The original is untouched (immutability).
	if msg.IsRead() || msg.IsFlagged() {
		t.Error("ApplyRules must not mutate the input message")
	}

	// A non-matching message is returned unchanged.
	other := ruleMessage(t, "Friend", "friend@example.com", "lunch?")
	res := ApplyRules([]MessageSummary{other}, []Rule{markRead, flag})
	if res[0].IsRead() || res[0].IsFlagged() {
		t.Error("non-matching message should be unchanged")
	}
}
