package domain

import "strings"

// RuleActionKind is what a rule does to a matching message. The values of the first two are frozen:
// they are stored as integers in the rule database and rules written before move and delete support
// carry them.
type RuleActionKind int

const (
	// RuleMarkRead marks a matching message as read.
	RuleMarkRead RuleActionKind = iota
	// RuleFlag flags (stars) a matching message.
	RuleFlag
	// RuleMoveTo moves a matching message into the action's destination folder. The destination names
	// one concrete folder, so the action applies only to messages in that folder's own account.
	RuleMoveTo
	// RuleDestroy deletes a matching message outright: it is expunged from the server and never enters
	// the local cache. There is no Trash hop and nothing to tidy up afterwards, so it is irreversible.
	RuleDestroy
)

// String returns a stable identifier for the action kind.
func (k RuleActionKind) String() string {
	switch k {
	case RuleMarkRead:
		return "markRead"
	case RuleFlag:
		return "flag"
	case RuleMoveTo:
		return "moveTo"
	case RuleDestroy:
		return "destroy"
	default:
		return "unknown"
	}
}

// Valid reports whether the kind is one a rule action can use.
func (k RuleActionKind) Valid() bool { return k >= RuleMarkRead && k <= RuleDestroy }

// Destructive reports whether the kind removes the message from where the user would look for it.
// The UI uses this to warn before such a rule is saved, because a rule runs unattended and so cannot
// confirm each message it acts on.
func (k RuleActionKind) Destructive() bool { return k == RuleMoveTo || k == RuleDestroy }

// RuleAction is one thing a rule does to a matching message. It is immutable once constructed.
type RuleAction struct {
	kind     RuleActionKind
	folderID string
}

// NewRuleAction validates and constructs an action. A move requires a destination folder id; every
// other kind requires none and ignores one that is supplied.
func NewRuleAction(kind RuleActionKind, folderID string) (RuleAction, error) {
	if !kind.Valid() {
		return RuleAction{}, ErrInvalidRuleAction
	}
	folderID = strings.TrimSpace(folderID)
	if kind != RuleMoveTo {
		return RuleAction{kind: kind}, nil
	}
	if folderID == "" {
		return RuleAction{}, ErrMissingRuleFolder
	}
	return RuleAction{kind: kind, folderID: folderID}, nil
}

// Kind returns what the action does.
func (a RuleAction) Kind() RuleActionKind { return a.kind }

// FolderID returns the destination folder for a move, empty for every other kind.
func (a RuleAction) FolderID() string { return a.folderID }

// Rule is a user-defined filter: when a message satisfies the rule's conditions (combined by its match
// mode), every one of its actions is applied. It is immutable once constructed.
type Rule struct {
	id             string
	name           string
	enabled        bool
	position       int
	matchMode      RuleMatchMode
	stopProcessing bool
	conditions     []RuleCondition
	actions        []RuleAction
}

// RuleSpec carries the parts of a rule for construction, so the constructor does not take a long and
// easily transposed argument list.
type RuleSpec struct {
	ID             string
	Name           string
	Enabled        bool
	Position       int
	MatchMode      RuleMatchMode
	StopProcessing bool
	Conditions     []RuleCondition
	Actions        []RuleAction
}

// NewRule validates and constructs a rule. The id and name must be non-empty, the match mode must be
// recognised and there must be at least one condition and at least one action.
func NewRule(spec RuleSpec) (Rule, error) {
	id := strings.TrimSpace(spec.ID)
	if id == "" {
		return Rule{}, ErrEmptyRuleID
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return Rule{}, ErrEmptyRuleName
	}
	if !spec.MatchMode.Valid() {
		return Rule{}, ErrInvalidRuleMatchMode
	}
	if len(spec.Conditions) == 0 {
		return Rule{}, ErrNoRuleConditions
	}
	if len(spec.Actions) == 0 {
		return Rule{}, ErrNoRuleActions
	}
	return Rule{
		id:             id,
		name:           name,
		enabled:        spec.Enabled,
		position:       spec.Position,
		matchMode:      spec.MatchMode,
		stopProcessing: spec.StopProcessing,
		conditions:     append([]RuleCondition(nil), spec.Conditions...),
		actions:        append([]RuleAction(nil), spec.Actions...),
	}, nil
}

// ID returns the rule identifier.
func (r Rule) ID() string { return r.id }

// Name returns the rule name.
func (r Rule) Name() string { return r.name }

// Enabled reports whether the rule runs. A disabled rule is kept and shown but never evaluated.
func (r Rule) Enabled() bool { return r.enabled }

// Position returns the rule's place in the evaluation order, lowest first.
func (r Rule) Position() int { return r.position }

// MatchMode returns how the rule combines its conditions.
func (r Rule) MatchMode() RuleMatchMode { return r.matchMode }

// StopProcessing reports whether a match ends evaluation of later rules for that message.
func (r Rule) StopProcessing() bool { return r.stopProcessing }

// Conditions returns a copy of the rule's conditions.
func (r Rule) Conditions() []RuleCondition { return append([]RuleCondition(nil), r.conditions...) }

// Actions returns a copy of the rule's actions.
func (r Rule) Actions() []RuleAction { return append([]RuleAction(nil), r.actions...) }

// Destructive reports whether any of the rule's actions moves or destroys a message.
func (r Rule) Destructive() bool {
	for _, a := range r.actions {
		if a.kind.Destructive() {
			return true
		}
	}
	return false
}

// Matches reports whether the message satisfies the rule's conditions under its match mode. A disabled
// rule never matches.
func (r Rule) Matches(m MessageSummary) bool {
	if !r.enabled {
		return false
	}
	if r.matchMode == RuleMatchAny {
		for _, c := range r.conditions {
			if c.Matches(m) {
				return true
			}
		}
		return false
	}
	for _, c := range r.conditions {
		if !c.Matches(m) {
			return false
		}
	}
	return true
}
