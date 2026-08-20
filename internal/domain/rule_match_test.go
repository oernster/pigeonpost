package domain

import "testing"

func TestRuleConditionOperators(t *testing.T) {
	m := ruleMessage(t, "Acme News", "news@acme.com", "Weekly Digest")
	cases := []struct {
		name     string
		field    RuleField
		operator RuleOperator
		text     string
		want     bool
	}{
		{"contains address", RuleFieldFrom, RuleOpContains, "ACME", true},
		{"contains display", RuleFieldFrom, RuleOpContains, "acme news", true},
		{"contains miss", RuleFieldFrom, RuleOpContains, "zzz", false},
		{"not contains hit", RuleFieldFrom, RuleOpNotContains, "zzz", true},
		{"not contains miss", RuleFieldFrom, RuleOpNotContains, "acme", false},
		{"equals address", RuleFieldFrom, RuleOpEquals, "News@Acme.com", true},
		{"equals miss", RuleFieldFrom, RuleOpEquals, "acme.com", false},
		{"starts with", RuleFieldFrom, RuleOpStartsWith, "news@", true},
		{"starts with display", RuleFieldFrom, RuleOpStartsWith, "acme n", true},
		{"starts with miss", RuleFieldFrom, RuleOpStartsWith, "@acme", false},
		{"ends with", RuleFieldFrom, RuleOpEndsWith, "acme.com", true},
		{"ends with miss", RuleFieldFrom, RuleOpEndsWith, "news@", false},
		{"subject contains", RuleFieldSubject, RuleOpContains, "digest", true},
		{"subject miss", RuleFieldSubject, RuleOpContains, "invoice", false},
		{"sender domain equals", RuleFieldSenderDomain, RuleOpEquals, "acme.com", true},
		{"sender domain not local part", RuleFieldSenderDomain, RuleOpContains, "news", false},
		{"sender domain ends with", RuleFieldSenderDomain, RuleOpEndsWith, ".com", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mustCondition(t, c.field, c.operator, c.text).Matches(m); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestRuleConditionRecipients(t *testing.T) {
	to := []EmailAddress{ruleAddress(t, "Alice", "alice@example.com")}
	cc := []EmailAddress{ruleAddress(t, "Team List", "team@lists.example.com")}
	m := ruleMessageWithRecipients(t, to, cc)
	cases := []struct {
		name  string
		field RuleField
		text  string
		want  bool
	}{
		{"to hit", RuleFieldTo, "alice@", true},
		{"to misses a cc", RuleFieldTo, "team@", false},
		{"cc hit", RuleFieldCc, "Team List", true},
		{"cc misses a to", RuleFieldCc, "alice@", false},
		{"any recipient sees to", RuleFieldAnyRecipient, "alice@", true},
		{"any recipient sees cc", RuleFieldAnyRecipient, "team@", true},
		{"any recipient miss", RuleFieldAnyRecipient, "bob@", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mustCondition(t, c.field, RuleOpContains, c.text).Matches(m); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// TestRuleConditionEmptyField pins the behaviour of a field with no candidates at all: every positive
// operator fails and "does not contain" holds, so a rule keyed on Cc does not fire on a message with
// none while its negation does.
func TestRuleConditionEmptyField(t *testing.T) {
	m := ruleMessageWithRecipients(t, nil, nil)
	if mustCondition(t, RuleFieldCc, RuleOpContains, "x").Matches(m) {
		t.Errorf("contains matched a message with no Cc")
	}
	if !mustCondition(t, RuleFieldCc, RuleOpNotContains, "x").Matches(m) {
		t.Errorf("does-not-contain failed on a message with no Cc")
	}
}

// TestRuleConditionSenderDomainAbsent covers a message carrying no sender: the condition has nothing
// to compare, so no positive operator matches and the negation holds.
func TestRuleConditionSenderDomainAbsent(t *testing.T) {
	m, err := NewMessageSummary(MessageSummaryInput{
		ID: "m1", FolderID: "f1", UID: "1", Subject: "s", Size: 1, Flags: NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	if mustCondition(t, RuleFieldSenderDomain, RuleOpContains, "a").Matches(m) {
		t.Errorf("domain condition matched a message with no sender")
	}
	if !mustCondition(t, RuleFieldSenderDomain, RuleOpNotContains, "a").Matches(m) {
		t.Errorf("does-not-contain failed on a message with no sender")
	}
}

func TestRuleMatchModes(t *testing.T) {
	m := ruleMessage(t, "Acme News", "news@acme.com", "Weekly Digest")
	hit := mustCondition(t, RuleFieldFrom, RuleOpContains, "acme")
	miss := mustCondition(t, RuleFieldSubject, RuleOpContains, "invoice")
	action := []RuleAction{mustAction(t, RuleFlag, "")}
	cases := []struct {
		name       string
		mode       RuleMatchMode
		conditions []RuleCondition
		want       bool
	}{
		{"all both hit", RuleMatchAll, []RuleCondition{hit, hit}, true},
		{"all one misses", RuleMatchAll, []RuleCondition{hit, miss}, false},
		{"any one hits", RuleMatchAny, []RuleCondition{miss, hit}, true},
		{"any none hit", RuleMatchAny, []RuleCondition{miss, miss}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := mustRule(t, RuleSpec{Enabled: true, MatchMode: c.mode, Conditions: c.conditions, Actions: action})
			if got := r.Matches(m); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestRuleDisabledNeverMatches(t *testing.T) {
	m := ruleMessage(t, "Acme News", "news@acme.com", "Weekly Digest")
	r := mustRule(t, RuleSpec{
		Enabled:    false,
		Conditions: []RuleCondition{mustCondition(t, RuleFieldFrom, RuleOpContains, "acme")},
		Actions:    []RuleAction{mustAction(t, RuleFlag, "")},
	})
	if r.Matches(m) {
		t.Errorf("a disabled rule matched")
	}
}

// mustCasedCondition builds a case-sensitive condition the test expects to be valid.
func mustCasedCondition(t *testing.T, field RuleField, op RuleOperator, text string) RuleCondition {
	t.Helper()
	c, err := NewRuleConditionCased(field, op, text, true)
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	return c
}

// TestRuleConditionCaseSensitivity pins both halves of the flag: the default ignores case, which is what
// every rule written before the flag existed did; turning it on makes the comparison exact.
func TestRuleConditionCaseSensitivity(t *testing.T) {
	m := ruleMessage(t, "Acme News", "news@acme.com", "Weekly Digest")
	cases := []struct {
		name          string
		operator      RuleOperator
		field         RuleField
		text          string
		insensitive   bool
		caseSensitive bool
	}{
		{"wrong case contains", RuleOpContains, RuleFieldSubject, "DIGEST", true, false},
		{"right case contains", RuleOpContains, RuleFieldSubject, "Digest", true, true},
		{"wrong case equals", RuleOpEquals, RuleFieldSubject, "weekly digest", true, false},
		{"right case equals", RuleOpEquals, RuleFieldSubject, "Weekly Digest", true, true},
		{"wrong case starts with", RuleOpStartsWith, RuleFieldSubject, "WEEKLY", true, false},
		{"wrong case ends with", RuleOpEndsWith, RuleFieldSubject, "DIGEST", true, false},
		// A negation flips with the flag too: the wrong case does not contain the text once case counts.
		{"wrong case does not contain", RuleOpNotContains, RuleFieldSubject, "DIGEST", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mustCondition(t, c.field, c.operator, c.text).Matches(m); got != c.insensitive {
				t.Errorf("case-insensitive: got %v, want %v", got, c.insensitive)
			}
			if got := mustCasedCondition(t, c.field, c.operator, c.text).Matches(m); got != c.caseSensitive {
				t.Errorf("case-sensitive: got %v, want %v", got, c.caseSensitive)
			}
		})
	}
}

// TestRuleConditionCaseSensitiveAccessor pins that the flag survives construction, since the store
// reads it back off the condition.
func TestRuleConditionCaseSensitiveAccessor(t *testing.T) {
	if mustCondition(t, RuleFieldFrom, RuleOpContains, "a").CaseSensitive() {
		t.Errorf("the default condition is case-sensitive")
	}
	if !mustCasedCondition(t, RuleFieldFrom, RuleOpContains, "a").CaseSensitive() {
		t.Errorf("the cased condition lost its flag")
	}
}

// TestRuleConditionAllFields pins the default field: one condition reaches the sender, every recipient,
// the subject and the sender's domain, so a rule can say "anywhere in this message".
func TestRuleConditionAllFields(t *testing.T) {
	to := []EmailAddress{ruleAddress(t, "Alice", "alice@example.com")}
	cc := []EmailAddress{ruleAddress(t, "Team List", "team@lists.example.com")}
	m, err := NewMessageSummary(MessageSummaryInput{
		ID: "m1", FolderID: "f1", UID: "1", From: ruleAddress(t, "Acme News", "news@acme.com"),
		To: to, Cc: cc, Subject: "Weekly Digest", Size: 1, Flags: NewFlags(0),
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	for _, text := range []string{"acme news", "news@acme", "digest", "alice@", "team list", "acme.com"} {
		if !mustCondition(t, RuleFieldAll, RuleOpContains, text).Matches(m) {
			t.Errorf("all-fields condition missed %q", text)
		}
	}
	if mustCondition(t, RuleFieldAll, RuleOpContains, "nowhere").Matches(m) {
		t.Errorf("all-fields condition matched text the message does not carry")
	}
}
