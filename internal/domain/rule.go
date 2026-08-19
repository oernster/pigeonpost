package domain

import "strings"

// RuleField is the part of a message a rule condition matches against. The values of the first four
// are frozen: they are stored as integers in the rule database and rules written before multi-condition
// support carry them.
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
	// RuleFieldAnyRecipient matches any To or Cc recipient's display name or address. Bcc is not
	// modelled: the sending server strips it, so a received message never carries one.
	RuleFieldAnyRecipient
	// RuleFieldSenderDomain matches the part of the sender's address after the @, so a rule can name a
	// whole domain without matching a local part that happens to contain it.
	RuleFieldSenderDomain
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
	case RuleFieldAnyRecipient:
		return "anyRecipient"
	case RuleFieldSenderDomain:
		return "senderDomain"
	default:
		return "unknown"
	}
}

// Valid reports whether the field is one a condition can match.
func (f RuleField) Valid() bool { return f >= RuleFieldFrom && f <= RuleFieldSenderDomain }

// RuleOperator is how a condition compares a message field against its match text.
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

// Valid reports whether the operator is one a condition can use.
func (o RuleOperator) Valid() bool { return o >= RuleOpContains && o <= RuleOpEndsWith }

// RuleMatchMode is how a rule combines its conditions. The zero value is "all", so a rule written
// before multi-condition support (which had exactly one condition) reads back unchanged.
type RuleMatchMode int

const (
	// RuleMatchAll requires every condition to match.
	RuleMatchAll RuleMatchMode = iota
	// RuleMatchAny requires at least one condition to match.
	RuleMatchAny
)

// String returns a stable identifier for the match mode.
func (m RuleMatchMode) String() string {
	switch m {
	case RuleMatchAll:
		return "all"
	case RuleMatchAny:
		return "any"
	default:
		return "unknown"
	}
}

// Valid reports whether the match mode is one a rule can use.
func (m RuleMatchMode) Valid() bool { return m == RuleMatchAll || m == RuleMatchAny }

// RuleCondition is one field-operator-text test within a rule. It is immutable once constructed.
type RuleCondition struct {
	field    RuleField
	operator RuleOperator
	text     string
}

// NewRuleCondition validates and constructs a condition. The match text must be non-empty and the
// field and operator must be recognised.
func NewRuleCondition(field RuleField, operator RuleOperator, text string) (RuleCondition, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return RuleCondition{}, ErrEmptyRuleMatch
	}
	if !field.Valid() {
		return RuleCondition{}, ErrInvalidRuleField
	}
	if !operator.Valid() {
		return RuleCondition{}, ErrInvalidRuleOperator
	}
	return RuleCondition{field: field, operator: operator, text: text}, nil
}

// Field returns the matched field.
func (c RuleCondition) Field() RuleField { return c.field }

// Operator returns how the field is compared against the match text.
func (c RuleCondition) Operator() RuleOperator { return c.operator }

// Text returns the match text.
func (c RuleCondition) Text() string { return c.text }

// Matches reports whether the message satisfies this condition. A field can contribute several
// candidate strings (a recipient's display name and address, for instance); the condition matches when
// any candidate satisfies the operator, except "does not contain", which matches only when no candidate
// contains the text. A field with no candidates at all (a message with no Cc, say) does not contain the
// text, so "does not contain" holds and every positive operator fails.
func (c RuleCondition) Matches(m MessageSummary) bool {
	needle := strings.ToLower(c.text)
	candidates := c.candidates(m)
	if c.operator == RuleOpNotContains {
		for _, s := range candidates {
			if strings.Contains(strings.ToLower(s), needle) {
				return false
			}
		}
		return true
	}
	for _, s := range candidates {
		if matchOperator(c.operator, strings.ToLower(s), needle) {
			return true
		}
	}
	return false
}

// candidates returns the strings this condition's field compares against.
func (c RuleCondition) candidates(m MessageSummary) []string {
	switch c.field {
	case RuleFieldSubject:
		return []string{m.Subject()}
	case RuleFieldTo:
		return addressStrings(m.To())
	case RuleFieldCc:
		return addressStrings(m.Cc())
	case RuleFieldAnyRecipient:
		return append(addressStrings(m.To()), addressStrings(m.Cc())...)
	case RuleFieldSenderDomain:
		return senderDomain(m.From())
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

// senderDomain returns just the domain part of the sender's address, so a domain condition never
// matches against a local part that happens to contain the text. A message with no sender at all
// contributes no candidate, so no positive operator can match it.
func senderDomain(from EmailAddress) []string {
	if from.Domain() == "" {
		return nil
	}
	return []string{from.Domain()}
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
