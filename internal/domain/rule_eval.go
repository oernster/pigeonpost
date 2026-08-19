package domain

import "sort"

// RuleOutcome is what the rules decided for one message. Message carries any flag actions already
// applied, so a caller that only stores flags can use it directly. MoveToFolderID and Destroy describe
// side effects the domain cannot perform: the caller executes them against the server.
type RuleOutcome struct {
	// Message is the message with every matching flag action applied.
	Message MessageSummary
	// MoveToFolderID is the folder the message should be moved into, empty when no rule moved it.
	MoveToFolderID string
	// Destroy reports that the message should be expunged outright, with no Trash hop and no local copy.
	Destroy bool
}

// Acted reports whether any rule decided a side effect for this message, so a caller can skip the
// messages the rules left alone.
func (o RuleOutcome) Acted() bool { return o.Destroy || o.MoveToFolderID != "" }

// EvaluateRules returns one outcome per message, in the same order, deciding what the rules want done.
// It performs no side effects: moves and destructions are described, not carried out, because they act
// on a remote server and the domain does no I/O.
//
// Rules are evaluated in position order, lowest first, so the order the user gives them is their
// priority. Within one message: flag actions accumulate; the FIRST move wins, so a higher rule's
// destination is not overridden by a lower one; a destroy ends evaluation immediately, because nothing
// further can be done to a message that will not exist. A rule with StopProcessing set ends evaluation
// for a message it matched.
func EvaluateRules(messages []MessageSummary, rules []Rule) []RuleOutcome {
	out := make([]RuleOutcome, len(messages))
	ordered := orderedRules(rules)
	for i, m := range messages {
		out[i] = evaluateOne(m, ordered)
	}
	return out
}

// orderedRules returns the enabled rules sorted by position, lowest first, without disturbing the
// caller's slice. Rules sharing a position keep their given order, so the sort is stable.
func orderedRules(rules []Rule) []Rule {
	ordered := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if r.Enabled() {
			ordered = append(ordered, r)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].position < ordered[j].position })
	return ordered
}

// evaluateOne applies the ordered rules to one message and returns its outcome.
func evaluateOne(m MessageSummary, ordered []Rule) RuleOutcome {
	outcome := RuleOutcome{Message: m}
	flags := m.Flags()
	for _, r := range ordered {
		if !r.Matches(m) {
			continue
		}
		for _, a := range r.actions {
			switch a.kind {
			case RuleMarkRead:
				flags = flags.With(FlagSeen)
			case RuleFlag:
				flags = flags.With(FlagFlagged)
			case RuleMoveTo:
				if outcome.MoveToFolderID == "" {
					outcome.MoveToFolderID = a.folderID
				}
			case RuleDestroy:
				outcome.Destroy = true
			}
		}
		if outcome.Destroy || r.stopProcessing {
			break
		}
	}
	outcome.Message = m.WithFlags(flags)
	return outcome
}

// ApplyRuleFlags returns copies of the messages with only the rules' flag actions applied, ignoring
// every side-effecting action. It is the pure path for callers that are not in a position to talk to
// the server; it is stable across repeated application because setting a flag twice is a no-op.
func ApplyRuleFlags(messages []MessageSummary, rules []Rule) []MessageSummary {
	if len(rules) == 0 {
		return messages
	}
	outcomes := EvaluateRules(messages, rules)
	out := make([]MessageSummary, len(outcomes))
	for i, o := range outcomes {
		out[i] = o.Message
	}
	return out
}
