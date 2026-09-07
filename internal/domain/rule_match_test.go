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
	// excludes names text the message HAS, so the exclusion is violated; permits names text it does
	// not have, so the exclusion is satisfied.
	excludes := mustCondition(t, RuleFieldFrom, RuleOpNotContains, "acme")
	permits := mustCondition(t, RuleFieldFrom, RuleOpNotContains, "mediamonkey")
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
		// A negative condition is an exclusion, so it is required under any as well as under all.
		// Without this an exclusion is one arm of an or; it is satisfied by every message the rule was
		// never about, so the rule matches the whole mailbox.
		{"any positive hits but the exclusion bites", RuleMatchAny, []RuleCondition{hit, excludes}, false},
		{"any positive hits and the exclusion allows", RuleMatchAny, []RuleCondition{hit, permits}, true},
		{"any exclusion alone does not widen the rule", RuleMatchAny, []RuleCondition{miss, permits}, false},
		// Only negatives: meeting them all is the whole rule, since there is no positive to meet.
		{"any only exclusions, all met", RuleMatchAny, []RuleCondition{permits, permits}, true},
		{"any only exclusions, one bites", RuleMatchAny, []RuleCondition{permits, excludes}, false},
		// Under all a negative is required exactly as it always was.
		{"all with an exclusion that bites", RuleMatchAll, []RuleCondition{hit, excludes}, false},
		{"all with an exclusion that allows", RuleMatchAll, []RuleCondition{hit, permits}, true},
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

// TestNegatedConditionsCoverEveryOperator pins the capability the not-contains operator could not
// give: every comparison has its opposite. A negated condition holds exactly when the plain one does
// not, on the same message and the same field.
func TestNegatedConditionsCoverEveryOperator(t *testing.T) {
	m := ruleMessage(t, "Acme News", "news@acme.com", "Weekly Digest")
	cases := []struct {
		name     string
		field    RuleField
		operator RuleOperator
		text     string
		want     bool
	}{
		{"not contains, text present", RuleFieldSubject, RuleOpContains, "Weekly", false},
		{"not contains, text absent", RuleFieldSubject, RuleOpContains, "Invoice", true},
		{"is not, equal", RuleFieldSubject, RuleOpEquals, "Weekly Digest", false},
		{"is not, different", RuleFieldSubject, RuleOpEquals, "Weekly", true},
		{"does not start with, it does", RuleFieldSubject, RuleOpStartsWith, "Weekly", false},
		{"does not start with, it does not", RuleFieldSubject, RuleOpStartsWith, "Digest", true},
		{"does not end with, it does", RuleFieldSubject, RuleOpEndsWith, "Digest", false},
		{"does not end with, it does not", RuleFieldSubject, RuleOpEndsWith, "Weekly", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			negated, err := NewRuleConditionFull(c.field, c.operator, c.text, false, true)
			if err != nil {
				t.Fatalf("condition: %v", err)
			}
			if got := negated.Matches(m); got != c.want {
				t.Errorf("negated: got %v, want %v", got, c.want)
			}
			plain, err := NewRuleConditionFull(c.field, c.operator, c.text, false, false)
			if err != nil {
				t.Fatalf("condition: %v", err)
			}
			if plain.Matches(m) == negated.Matches(m) {
				t.Errorf("negating the condition did not change what it matches")
			}
		})
	}
}

// A negation has to be read against the whole field: "does not contain" is false when ANY of the
// field's candidate strings contains the text, never merely when some other candidate does not. The
// From field carries both a display name and an address, which is where a per-candidate reading would
// go wrong: the address holds the text while the display name does not.
func TestNegationAppliesToTheWholeField(t *testing.T) {
	m := ruleMessage(t, "Acme News", "news@acme.com", "Weekly Digest")
	c, err := NewRuleConditionFull(RuleFieldFrom, RuleOpContains, "acme.com", false, true)
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	if c.Matches(m) {
		t.Error("a negated condition held while part of the field satisfied the comparison")
	}
}

// A rule stored before negation became its own flag carries the retired not-contains operator. It is
// folded into contains-and-negated on construction, so it means what it always meant and there is one
// spelling of a negation from there on.
func TestRetiredNotContainsFoldsIntoANegatedCondition(t *testing.T) {
	c, err := NewRuleCondition(RuleFieldSubject, RuleOpNotContains, "invoice")
	if err != nil {
		t.Fatalf("condition: %v", err)
	}
	if !c.Negated() {
		t.Error("the retired operator did not read back as a negation")
	}
	if c.Operator() != RuleOpContains {
		t.Errorf("operator is %v, want contains", c.Operator())
	}
	m := ruleMessage(t, "Acme News", "news@acme.com", "Weekly Digest")
	if !c.Matches(m) {
		t.Error("the folded condition stopped matching a message that does not contain the text")
	}
}

// TestAnyModeRuleWithSeveralExclusions encodes a whole rule as the editor writes it, rather than one
// condition at a time: three plain conditions under "any" plus three negated ones. It is the shape a
// rule takes in use; it is what says, in a form that can be re-run, how such a rule reads:
//
//	(contains "7digital" OR contains "boomkat" OR contains "flac")
//	AND NOT contains "mediamonkey" AND NOT contains "peter" AND NOT contains "barbara"
func TestAnyModeRuleWithSeveralExclusions(t *testing.T) {
	positive := func(text string) RuleCondition {
		t.Helper()
		return mustCondition(t, RuleFieldAll, RuleOpContains, text)
	}
	excluded := func(text string) RuleCondition {
		t.Helper()
		c, err := NewRuleConditionFull(RuleFieldAll, RuleOpContains, text, false, true)
		if err != nil {
			t.Fatalf("condition: %v", err)
		}
		return c
	}
	rule := mustRule(t, RuleSpec{
		Enabled: true, MatchMode: RuleMatchAny,
		Conditions: []RuleCondition{
			positive("7digital"), positive("boomkat"), positive("flac"),
			excluded("mediamonkey"), excluded("peter"), excluded("barbara"),
		},
		Actions: []RuleAction{mustAction(t, RuleFlag, "")},
	})
	cases := []struct {
		name    string
		subject string
		want    bool
	}{
		{"one alternative, no exclusion", "Your 7digital receipt", true},
		{"another alternative", "boomkat order", true},
		{"third alternative", "your flac download", true},
		{"an alternative and an exclusion", "7digital for mediamonkey", false},
		{"an alternative and a different exclusion", "boomkat order from peter", false},
		{"an alternative and the last exclusion", "flac for barbara", false},
		{"no alternative at all", "an unrelated message", false},
		{"no alternative and no exclusion either", "nothing to see", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := ruleMessage(t, "Someone", "someone@example.com", c.subject)
			if got := rule.Matches(m); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}
