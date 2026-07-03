package domain

import "strings"

// RuleField is the part of a message a rule matches against.
type RuleField int

const (
	// RuleFieldFrom matches the sender's display name and address.
	RuleFieldFrom RuleField = iota
	// RuleFieldSubject matches the subject line.
	RuleFieldSubject
)

// String returns a stable identifier for the field.
func (f RuleField) String() string {
	switch f {
	case RuleFieldFrom:
		return "from"
	case RuleFieldSubject:
		return "subject"
	default:
		return "unknown"
	}
}

// Valid reports whether the field is one a rule can match.
func (f RuleField) Valid() bool { return f == RuleFieldFrom || f == RuleFieldSubject }

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

// Rule is a user-defined filter: when a message's chosen field contains the given text (case
// insensitive), the action is applied. It is immutable once constructed.
type Rule struct {
	id       string
	name     string
	field    RuleField
	contains string
	action   RuleAction
}

// NewRule validates and constructs a rule. The id, name and match text must be non-empty, and the
// field and action must be recognised.
func NewRule(id, name string, field RuleField, contains string, action RuleAction) (Rule, error) {
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
	if !action.Valid() {
		return Rule{}, ErrInvalidRuleAction
	}
	return Rule{id: id, name: name, field: field, contains: contains, action: action}, nil
}

// ID returns the rule identifier.
func (r Rule) ID() string { return r.id }

// Name returns the rule name.
func (r Rule) Name() string { return r.name }

// Field returns the matched field.
func (r Rule) Field() RuleField { return r.field }

// Contains returns the match text.
func (r Rule) Contains() string { return r.contains }

// Action returns the action applied on a match.
func (r Rule) Action() RuleAction { return r.action }

// Matches reports whether the message satisfies this rule.
func (r Rule) Matches(m MessageSummary) bool {
	var hay string
	switch r.field {
	case RuleFieldSubject:
		hay = m.Subject()
	default:
		hay = m.From().Display() + " " + m.From().Address()
	}
	return strings.Contains(strings.ToLower(hay), strings.ToLower(r.contains))
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
