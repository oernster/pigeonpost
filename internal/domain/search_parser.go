package domain

import (
	"strings"
	"time"
	"unicode"
)

// isoDateLayout is the date form the before:/after:/on: operators accept.
const isoDateLayout = "2006-01-02"

// searchFieldOps maps the text-field operators to their fields.
var searchFieldOps = map[string]SearchField{
	"from":     FieldFrom,
	"to":       FieldTo,
	"subject":  FieldSubject,
	"filename": FieldFilename,
}

// ParseSearchQuery parses raw user input into a SearchQuery. loc gives the calendar the date operators
// (before:/after:/on:) are interpreted in; it is injected so the domain never reads the wall clock or
// the machine locale itself.
//
// The parser never fails. Input whose structure cannot be tokenized (an unclosed quote) degrades to a
// plain free-text search over the whole input, marked Degraded. An unknown operator prefix, an
// unrecognised operator operand (has:x, is:x, a malformed date) and a dangling or misplaced OR are all
// kept as literal search text rather than errors, so a query can only ever search for something.
func ParseSearchQuery(raw string, loc *time.Location) SearchQuery {
	tokens, ok := scanSearchTokens(raw)
	if !ok {
		return degradedSearchQuery(raw)
	}
	return assembleSearchQuery(tokens, loc)
}

// degradedSearchQuery treats the whole input as bare free-text words, stripping quote characters so
// the unbalanced quote that forced the fallback cannot leak into a term.
func degradedSearchQuery(raw string) SearchQuery {
	groups := make([][]SearchTerm, 0)
	for _, word := range strings.Fields(raw) {
		term, err := NewSearchTerm(strings.ReplaceAll(word, `"`, ""), FieldAny, false, false)
		if err != nil {
			continue
		}
		groups = append(groups, []SearchTerm{term})
	}
	query, _ := NewSearchQuery(SearchQueryInput{Groups: groups, Degraded: true})
	return query
}

// searchToken is one scanned unit of the raw input: an optional negation, an optional operator prefix
// and a value that was either a bare word or a quoted phrase. A bare, unnegated, un-prefixed OR is
// marked as the OR keyword.
type searchToken struct {
	negated bool
	op      string
	value   string
	phrase  bool
	or      bool
}

// literal reconstructs the token as plain search text, used when its operator or operand is not
// recognised: the user searched for that text, whatever it was.
func (t searchToken) literal() string {
	text := t.value
	if t.op != "" {
		text = t.op + ":" + text
	}
	if t.negated {
		text = "-" + text
	}
	return text
}

// scanSearchTokens splits raw input into tokens. It reports ok false only when a quote is left
// unclosed, which is the one shape that cannot be tokenized reliably.
func scanSearchTokens(raw string) ([]searchToken, bool) {
	runes := []rune(raw)
	tokens := make([]searchToken, 0)
	i := 0
	for i < len(runes) {
		if unicode.IsSpace(runes[i]) {
			i++
			continue
		}
		var t searchToken
		if runes[i] == '-' && i+1 < len(runes) && !unicode.IsSpace(runes[i+1]) {
			t.negated = true
			i++
		}
		// An operator is a run of ASCII letters ending at a colon, with the value following on directly.
		// Anything else (a dot in the prefix, a mid-word colon later on) is part of a bare word.
		if op, rest, ok := scanSearchOperator(runes, i); ok {
			t.op = op
			i = rest
		}
		if i < len(runes) && runes[i] == '"' {
			value, rest, ok := scanQuoted(runes, i)
			if !ok {
				return nil, false
			}
			t.value, t.phrase = value, true
			i = rest
		} else {
			start := i
			for i < len(runes) && !unicode.IsSpace(runes[i]) {
				i++
			}
			t.value = string(runes[start:i])
		}
		if t.value == "OR" && !t.negated && t.op == "" && !t.phrase {
			t.or = true
		}
		tokens = append(tokens, t)
	}
	return tokens, true
}

// scanSearchOperator reads a leading run of ASCII letters followed by a colon at position i, returning
// the lowercased operator name and the position after the colon. It reports ok false when the shape
// does not match, in which case scanning resumes at i unchanged.
func scanSearchOperator(runes []rune, i int) (op string, rest int, ok bool) {
	j := i
	for j < len(runes) && isASCIILetter(runes[j]) {
		j++
	}
	if j == i || j >= len(runes) || runes[j] != ':' {
		return "", i, false
	}
	return strings.ToLower(string(runes[i:j])), j + 1, true
}

// scanQuoted reads a double-quoted string starting at the opening quote, returning the unquoted value
// and the position after the closing quote. ok is false when the quote is never closed.
func scanQuoted(runes []rune, i int) (value string, rest int, ok bool) {
	j := i + 1
	for j < len(runes) && runes[j] != '"' {
		j++
	}
	if j >= len(runes) {
		return "", i, false
	}
	return string(runes[i+1 : j]), j + 1, true
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// assembleSearchQuery walks the scanned tokens and builds the query: text terms become groups (with OR
// joining adjacent positive terms into one group), negated text terms become exclusions and the
// structural operators set their predicates (last occurrence winning). Tokens that do not parse as
// what their operator promises are kept as literal text. Scanner tokens always assemble (a token that
// yields no searchable text, such as an empty pair of quotes, is simply skipped), so no error surfaces.
func assembleSearchQuery(tokens []searchToken, loc *time.Location) SearchQuery {
	in := SearchQueryInput{}
	// orPending records that the previous token was a positive text term followed by OR, so the next
	// positive text term joins its group instead of starting a new one.
	orPending := false
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.or {
			// OR is only meaningful between two positive text terms: one just emitted and one to come.
			if len(in.Groups) > 0 && !orPending && i+1 < len(tokens) && isPositiveTextToken(tokens[i+1]) {
				orPending = true
				continue
			}
			appendSearchTerm(&in, &orPending, t.value, FieldAny, false, false)
			continue
		}
		if t.op == "" || searchFieldOpKnown(t.op) {
			if t.value == "" {
				// A bare operator with no operand ("from:") is searched literally; empty quotes yield nothing.
				appendSearchTerm(&in, &orPending, t.literal(), FieldAny, false, false)
				continue
			}
			field := FieldAny
			if t.op != "" {
				field = searchFieldOps[t.op]
			}
			appendSearchTerm(&in, &orPending, t.value, field, t.phrase, t.negated)
			continue
		}
		if t.negated || !applyStructuralOp(&in, t, loc) {
			// A negated structural operator, an unknown operator or an unrecognised operand: literal text.
			appendSearchTerm(&in, &orPending, t.literal(), FieldAny, false, false)
		}
	}
	// The input is valid by construction (every term passed NewSearchTerm; groups are never empty), so
	// the constructor error is structurally impossible here and is discarded as in degradedSearchQuery.
	query, _ := NewSearchQuery(in)
	return query
}

func searchFieldOpKnown(op string) bool {
	_, ok := searchFieldOps[op]
	return ok
}

// isPositiveTextToken reports whether a token would assemble into a positive text term, which is what
// an OR may join.
func isPositiveTextToken(t searchToken) bool {
	return !t.or && !t.negated && t.value != "" && (t.op == "" || searchFieldOpKnown(t.op))
}

// appendSearchTerm adds one text term to the query being assembled: negated terms join the exclusions,
// positive terms start a new group or, when an OR is pending, join the previous one. Text that carries
// nothing searchable (an empty or whitespace-only pair of quotes) is skipped; a skipped term also
// consumes any pending OR, so the term after it starts its own group rather than inheriting the join
// ("a OR \" \" b" searches a and b, not a-or-b).
func appendSearchTerm(in *SearchQueryInput, orPending *bool, text string, field SearchField, phrase, negated bool) {
	term, err := NewSearchTerm(text, field, phrase, negated)
	if err != nil {
		*orPending = false
		return
	}
	if negated {
		in.Excluded = append(in.Excluded, term)
		return
	}
	if *orPending {
		last := len(in.Groups) - 1
		in.Groups[last] = append(in.Groups[last], term)
		*orPending = false
		return
	}
	in.Groups = append(in.Groups, []SearchTerm{term})
}

// applyStructuralOp applies one structural operator token to the query, reporting whether the operand
// was recognised. Each predicate keeps the last occurrence, which is the calm reading of a repeated
// operator.
func applyStructuralOp(in *SearchQueryInput, t searchToken, loc *time.Location) bool {
	switch t.op {
	case "in":
		if t.value == "" {
			return false
		}
		in.FolderPath = t.value
	case "account":
		if t.value == "" {
			return false
		}
		in.AccountName = t.value
	case "has":
		if !strings.EqualFold(t.value, "attachment") {
			return false
		}
		in.HasAttachment = true
	case "is":
		switch strings.ToLower(t.value) {
		case "unread":
			in.Unread = true
		case "read":
			in.Read = true
		case "flagged":
			in.Flagged = true
		default:
			return false
		}
	case "before", "after", "on":
		day, err := time.ParseInLocation(isoDateLayout, t.value, loc)
		if err != nil {
			return false
		}
		switch t.op {
		case "before":
			in.Before = day
		case "after":
			in.After = day
		default: // on: is sugar for that whole calendar day.
			in.After, in.Before = day, day.AddDate(0, 0, 1)
		}
	default:
		return false
	}
	return true
}
