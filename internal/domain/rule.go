package domain

import "strings"

// RuleField is the part of a message a rule matches against.
type RuleField int

const (
	// RuleFieldFrom matches the sender's display name and address.
	RuleFieldFrom RuleField = iota
	// RuleFieldSubject matches the subject line.
	RuleFieldSubject
	// RuleFieldTo matches any To recipient's display name or address.
	RuleFieldTo
	// RuleFieldCc matches any Cc recipient's display name or address.
	RuleFieldCc
)

// String returns a stable identifier for the field.
func (f RuleField) String() string {
	switch f {
	case RuleFieldFrom:
		return "from"
	case RuleFieldSubject:
		return "subject"
	case RuleFieldTo:
		return "to"
	case RuleFieldCc:
		return "cc"
	default:
		return "unknown"
	}
}

// Valid reports whether the field is one a rule can match.
func (f RuleField) Valid() bool { return f >= RuleFieldFrom && f <= RuleFieldCc }

// RuleOperator is how a rule compares a message field against its match text.
type RuleOperator int

const (
	// RuleOpContains matches when the field contains the text.
	RuleOpContains RuleOperator = iota
	// RuleOpNotContains matches when the field does not contain the text.
	RuleOpNotContains
	// RuleOpEquals matches when the field equals the text exactly.
	RuleOpEquals
	// RuleOpStartsWith matches when the field begins with the text.
	RuleOpStartsWith
	// RuleOpEndsWith matches when the field ends with the text.
	RuleOpEndsWith
)

// String returns a stable identifier for the operator.
func (o RuleOperator) String() string {
	switch o {
	case RuleOpContains:
		return "contains"
	case RuleOpNotContains:
		return "notContains"
	case RuleOpEquals:
		return "equals"
	case RuleOpStartsWith:
		return "startsWith"
	case RuleOpEndsWith:
		return "endsWith"
	default:
		return "unknown"
	}
}

// Valid reports whether the operator is one a rule can use.
func (o RuleOperator) Valid() bool { return o >= RuleOpContains && o <= RuleOpEndsWith }

// RuleAction is what a rule does to a matching message. v1 actions are non-destructive and idempotent,
// so they can be re-applied on every sync without harm; move and delete are deliberately not modelled.
type RuleAction int

const (
	// RuleMarkRead marks a matching message as read.
	RuleMarkRead RuleAction = iota
	// RuleFlag flags (stars) a matching message.
	RuleFlag
)

// String returns a stable identifier for the action.
func (a RuleAction) String() string {
	switch a {
	case RuleMarkRead:
		return "markRead"
	case RuleFlag:
		return "flag"
	default:
		return "unknown"
	}
}

// Valid reports whether the action is one a rule can apply.
func (a RuleAction) Valid() bool { return a == RuleMarkRead || a == RuleFlag }

// Rule is a user-defined filter: when a message's chosen field satisfies the operator against the
// given text (case insensitive), the action is applied. It is immutable once constructed.
type Rule struct {
	id       string
	name     string
	field    RuleField
	operator RuleOperator
	contains string
	action   RuleAction
}

// NewRule validates and constructs a rule. The id, name and match text must be non-empty, and the
// field, operator and action must be recognised.
func NewRule(id, name string, field RuleField, operator RuleOperator, contains string, action RuleAction) (Rule, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Rule{}, ErrEmptyRuleID
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Rule{}, ErrEmptyRuleName
	}
	contains = strings.TrimSpace(contains)
	if contains == "" {
		return Rule{}, ErrEmptyRuleMatch
	}
	if !field.Valid() {
		return Rule{}, ErrInvalidRuleField
	}
	if !operator.Valid() {
		return Rule{}, ErrInvalidRuleOperator
	}
	if !action.Valid() {
		return Rule{}, ErrInvalidRuleAction
	}
	return Rule{id: id, name: name, field: field, operator: operator, contains: contains, action: action}, nil
}

// ID returns the rule identifier.
func (r Rule) ID() string { return r.id }

// Name returns the rule name.
func (r Rule) Name() string { return r.name }

// Field returns the matched field.
func (r Rule) Field() RuleField { return r.field }

// Operator returns how the field is compared against the match text.
func (r Rule) Operator() RuleOperator { return r.operator }

// Contains returns the match text.
func (r Rule) Contains() string { return r.contains }

// Action returns the action applied on a match.
func (r Rule) Action() RuleAction { return r.action }

// Matches reports whether the message satisfies this rule. A field can contribute several candidate
// strings (a recipient's display name and address, for instance); the rule matches when any candidate
// satisfies the operator, except "does not contain", which matches only when no candidate contains the
// text.
func (r Rule) Matches(m MessageSummary) bool {
	needle := strings.ToLower(r.contains)
	candidates := r.candidates(m)
	if r.operator == RuleOpNotContains {
		for _, c := range candidates {
			if strings.Contains(strings.ToLower(c), needle) {
				return false
			}
		}
		return true
	}
	for _, c := range candidates {
		if matchOperator(r.operator, strings.ToLower(c), needle) {
			return true
		}
	}
	return false
}

// candidates returns the strings the rule's field compares against.
func (r Rule) candidates(m MessageSummary) []string {
	switch r.field {
	case RuleFieldSubject:
		return []string{m.Subject()}
	case RuleFieldTo:
		return addressStrings(m.To())
	case RuleFieldCc:
		return addressStrings(m.Cc())
	default:
		return []string{m.From().Display(), m.From().Address()}
	}
}

// addressStrings flattens addresses to their display names and addresses for matching.
func addressStrings(addrs []EmailAddress) []string {
	out := make([]string, 0, len(addrs)*2)
	for _, a := range addrs {
		out = append(out, a.Display(), a.Address())
	}
	return out
}

// matchOperator applies a positive operator (every operator except "does not contain") to one
// lower-cased candidate and needle.
func matchOperator(op RuleOperator, hay, needle string) bool {
	switch op {
	case RuleOpEquals:
		return hay == needle
	case RuleOpStartsWith:
		return strings.HasPrefix(hay, needle)
	case RuleOpEndsWith:
		return strings.HasSuffix(hay, needle)
	default:
		return strings.Contains(hay, needle)
	}
}

// ApplyRules returns copies of the messages with every matching rule's action applied. Actions only
// ever set flags, so applying the same rules again is a no-op: the result is stable across syncs.
func ApplyRules(messages []MessageSummary, rules []Rule) []MessageSummary {
	if len(rules) == 0 {
		return messages
	}
	out := make([]MessageSummary, len(messages))
	for i, m := range messages {
		flags := m.Flags()
		for _, r := range rules {
			if !r.Matches(m) {
				continue
			}
			switch r.action {
			case RuleMarkRead:
				flags = flags.With(FlagSeen)
			case RuleFlag:
				flags = flags.With(FlagFlagged)
			}
		}
		out[i] = m.WithFlags(flags)
	}
	return out
}
