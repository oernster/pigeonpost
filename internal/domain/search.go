package domain

import (
	"strings"
	"time"
)

// SearchField constrains a search term to one indexed text field of a message. FieldAny matches across
// every indexed field (subject, body preview and cached body, sender, recipients and attachment
// filenames), which is what a bare term with no operator means.
type SearchField int

const (
	// FieldAny matches a term against every indexed text field.
	FieldAny SearchField = iota
	// FieldFrom matches the sender display name and address (the from: operator).
	FieldFrom
	// FieldTo matches the To and Cc recipients (the to: operator).
	FieldTo
	// FieldSubject matches the subject only (the subject: operator).
	FieldSubject
	// FieldFilename matches cached attachment filenames (the filename: operator).
	FieldFilename
)

// SearchTerm is one text atom of a search query: a word or an exact phrase, optionally constrained to
// a field and optionally negated (a matching message is excluded). A word is matched as a prefix by
// the index so typing feels instant; a phrase is matched exactly.
type SearchTerm struct {
	text    string
	field   SearchField
	phrase  bool
	negated bool
}

// NewSearchTerm validates and constructs a search term. text must be non-blank.
func NewSearchTerm(text string, field SearchField, phrase, negated bool) (SearchTerm, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return SearchTerm{}, ErrEmptySearchTerm
	}
	return SearchTerm{text: text, field: field, phrase: phrase, negated: negated}, nil
}

// Text returns the term's text without quotes.
func (t SearchTerm) Text() string { return t.text }

// Field returns the field the term is constrained to (FieldAny for a bare term).
func (t SearchTerm) Field() SearchField { return t.field }

// IsPhrase reports whether the term is an exact phrase (it was quoted) rather than a prefix word.
func (t SearchTerm) IsPhrase() bool { return t.phrase }

// IsNegated reports whether a matching message is excluded rather than required.
func (t SearchTerm) IsNegated() bool { return t.negated }

// SearchQueryInput carries the parsed parts of a query into NewSearchQuery. Groups is the positive
// text: the outer slice is ANDed, the terms inside one group are ORed (a single-term group is a plain
// required term). Excluded holds the negated terms; every one must fail to match. The remaining fields
// are the structural predicates. After is an inclusive lower bound on the message date and Before an
// exclusive upper bound; either may be zero for unbounded. Degraded records that the raw input could
// not be parsed structurally and was fallen back to plain free text, so the UI can hint at it.
type SearchQueryInput struct {
	Groups        [][]SearchTerm
	Excluded      []SearchTerm
	HasAttachment bool
	Unread        bool
	Read          bool
	Flagged       bool
	FolderPath    string
	AccountName   string
	After         time.Time
	Before        time.Time
	Degraded      bool
}

// SearchQuery is the parsed, validated representation of a search: what text must (and must not)
// match, in which fields, and the structural predicates that scope it. It is pure data; translating it
// to an index query is the storage adapter's job and deciding result policy is the application's.
type SearchQuery struct {
	groups        [][]SearchTerm
	excluded      []SearchTerm
	hasAttachment bool
	unread        bool
	read          bool
	flagged       bool
	folderPath    string
	accountName   string
	after         time.Time
	before        time.Time
	scopeFolder   string
	scopeAccount  string
	degraded      bool
}

// NewSearchQuery validates and constructs a query from its parsed parts. Empty groups are rejected, as
// is any zero-value term smuggled in without NewSearchTerm; negated terms belong in Excluded and
// positive terms in Groups, and a term on the wrong side is rejected too.
func NewSearchQuery(in SearchQueryInput) (SearchQuery, error) {
	groups := make([][]SearchTerm, 0, len(in.Groups))
	for _, group := range in.Groups {
		if len(group) == 0 {
			return SearchQuery{}, ErrEmptySearchGroup
		}
		copied := make([]SearchTerm, 0, len(group))
		for _, term := range group {
			if term.text == "" {
				return SearchQuery{}, ErrEmptySearchTerm
			}
			if term.negated {
				return SearchQuery{}, ErrNegatedTermInGroup
			}
			copied = append(copied, term)
		}
		groups = append(groups, copied)
	}
	excluded := make([]SearchTerm, 0, len(in.Excluded))
	for _, term := range in.Excluded {
		if term.text == "" {
			return SearchQuery{}, ErrEmptySearchTerm
		}
		if !term.negated {
			return SearchQuery{}, ErrPositiveTermExcluded
		}
		excluded = append(excluded, term)
	}
	return SearchQuery{
		groups:        groups,
		excluded:      excluded,
		hasAttachment: in.HasAttachment,
		unread:        in.Unread,
		read:          in.Read,
		flagged:       in.Flagged,
		folderPath:    strings.TrimSpace(in.FolderPath),
		accountName:   strings.TrimSpace(in.AccountName),
		after:         in.After,
		before:        in.Before,
		degraded:      in.Degraded,
	}, nil
}

// Groups returns the positive text of the query: ANDed groups of ORed terms. The result is a copy.
func (q SearchQuery) Groups() [][]SearchTerm {
	out := make([][]SearchTerm, 0, len(q.groups))
	for _, group := range q.groups {
		out = append(out, append([]SearchTerm(nil), group...))
	}
	return out
}

// Excluded returns the negated terms; a message matching any of them is excluded. The result is a copy.
func (q SearchQuery) Excluded() []SearchTerm { return append([]SearchTerm(nil), q.excluded...) }

// HasAttachment reports whether results are restricted to messages with at least one attachment.
func (q SearchQuery) HasAttachment() bool { return q.hasAttachment }

// Unread reports whether results are restricted to unread messages (is:unread).
func (q SearchQuery) Unread() bool { return q.unread }

// Read reports whether results are restricted to read messages (is:read).
func (q SearchQuery) Read() bool { return q.read }

// Flagged reports whether results are restricted to flagged messages (is:flagged).
func (q SearchQuery) Flagged() bool { return q.flagged }

// FolderPath returns the folder path results are scoped to (the in: operator), or empty for all
// folders. Matching is by full path, case-insensitively.
func (q SearchQuery) FolderPath() string { return q.folderPath }

// AccountName returns the account results are scoped to (the account: operator), matched
// case-insensitively against the account's display name or email address, or empty for all accounts.
func (q SearchQuery) AccountName() string { return q.accountName }

// After returns the inclusive lower bound on the message date, or the zero time for unbounded.
func (q SearchQuery) After() time.Time { return q.after }

// Before returns the exclusive upper bound on the message date, or the zero time for unbounded.
func (q SearchQuery) Before() time.Time { return q.before }

// ScopeFolderID returns the exact folder id the UI scoped the search to, or empty for no scope. It is
// set by WithFolderScope, not by the parser: the scope selector speaks folder ids while the in:
// operator speaks folder paths.
func (q SearchQuery) ScopeFolderID() string { return q.scopeFolder }

// ScopeAccountID returns the exact account id the UI scoped the search to, or empty for no scope.
func (q SearchQuery) ScopeAccountID() string { return q.scopeAccount }

// IsDegraded reports whether the raw input failed structural parsing and was treated as plain free
// text, so the UI can hint that operators were ignored.
func (q SearchQuery) IsDegraded() bool { return q.degraded }

// WithFolderScope returns a copy scoped to the exact folder id (the UI's current-folder scope).
func (q SearchQuery) WithFolderScope(folderID string) SearchQuery {
	q.scopeFolder = strings.TrimSpace(folderID)
	return q
}

// WithAccountScope returns a copy scoped to the exact account id (the UI's one-account scope).
func (q SearchQuery) WithAccountScope(accountID string) SearchQuery {
	q.scopeAccount = strings.TrimSpace(accountID)
	return q
}

// HasText reports whether the query carries any positive text to match (the index is only consulted
// when it does; a purely structural query like "is:unread in:INBOX" filters without it).
func (q SearchQuery) HasText() bool { return len(q.groups) > 0 }

// IsEmpty reports whether the query asks for nothing at all: no text, no exclusions and no structural
// predicates. UI scope alone does not make a query non-empty, since scoping nothing is still nothing.
func (q SearchQuery) IsEmpty() bool {
	return len(q.groups) == 0 && len(q.excluded) == 0 &&
		!q.hasAttachment && !q.unread && !q.read && !q.flagged &&
		q.folderPath == "" && q.accountName == "" && q.after.IsZero() && q.before.IsZero()
}
