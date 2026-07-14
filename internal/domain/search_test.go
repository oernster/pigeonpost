package domain

import (
	"errors"
	"testing"
	"time"
)

func mkTerm(t *testing.T, text string, field SearchField, phrase, negated bool) SearchTerm {
	t.Helper()
	term, err := NewSearchTerm(text, field, phrase, negated)
	if err != nil {
		t.Fatalf("NewSearchTerm(%q): %v", text, err)
	}
	return term
}

func TestNewSearchTermValid(t *testing.T) {
	term := mkTerm(t, "  report  ", FieldSubject, true, true)
	if term.Text() != "report" {
		t.Errorf("Text = %q, want trimmed report", term.Text())
	}
	if term.Field() != FieldSubject || !term.IsPhrase() || !term.IsNegated() {
		t.Errorf("term fields not kept: %+v", term)
	}
}

func TestNewSearchTermEmpty(t *testing.T) {
	if _, err := NewSearchTerm("   ", FieldAny, false, false); !errors.Is(err, ErrEmptySearchTerm) {
		t.Errorf("error = %v, want ErrEmptySearchTerm", err)
	}
}

func TestNewSearchQueryValid(t *testing.T) {
	after := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	q, err := NewSearchQuery(SearchQueryInput{
		Groups: [][]SearchTerm{
			{mkTerm(t, "alpha", FieldAny, false, false), mkTerm(t, "beta", FieldAny, false, false)},
			{mkTerm(t, "report", FieldSubject, true, false)},
		},
		Excluded:      []SearchTerm{mkTerm(t, "spam", FieldAny, false, true)},
		HasAttachment: true,
		Unread:        true,
		Read:          true,
		Flagged:       true,
		FolderPath:    " INBOX ",
		AccountName:   " Work ",
		After:         after,
		Before:        before,
		Degraded:      true,
	})
	if err != nil {
		t.Fatalf("NewSearchQuery: %v", err)
	}
	groups := q.Groups()
	if len(groups) != 2 || len(groups[0]) != 2 || groups[0][0].Text() != "alpha" || groups[1][0].Text() != "report" {
		t.Errorf("groups not kept: %+v", groups)
	}
	if excluded := q.Excluded(); len(excluded) != 1 || excluded[0].Text() != "spam" {
		t.Errorf("excluded not kept: %+v", excluded)
	}
	if !q.HasAttachment() || !q.Unread() || !q.Read() || !q.Flagged() {
		t.Error("status predicates not kept")
	}
	if q.FolderPath() != "INBOX" || q.AccountName() != "Work" {
		t.Errorf("scopes not trimmed: %q %q", q.FolderPath(), q.AccountName())
	}
	if !q.After().Equal(after) || !q.Before().Equal(before) {
		t.Errorf("date bounds not kept: %v %v", q.After(), q.Before())
	}
	if !q.IsDegraded() || !q.HasText() || q.IsEmpty() {
		t.Error("degraded/text/empty flags wrong")
	}
}

func TestNewSearchQueryValidation(t *testing.T) {
	positive := mkTerm(t, "x", FieldAny, false, false)
	negated := mkTerm(t, "x", FieldAny, false, true)
	cases := []struct {
		name string
		in   SearchQueryInput
		want error
	}{
		{"empty group", SearchQueryInput{Groups: [][]SearchTerm{{}}}, ErrEmptySearchGroup},
		{"zero term in group", SearchQueryInput{Groups: [][]SearchTerm{{{}}}}, ErrEmptySearchTerm},
		{"negated term in group", SearchQueryInput{Groups: [][]SearchTerm{{negated}}}, ErrNegatedTermInGroup},
		{"zero term excluded", SearchQueryInput{Excluded: []SearchTerm{{}}}, ErrEmptySearchTerm},
		{"positive term excluded", SearchQueryInput{Excluded: []SearchTerm{positive}}, ErrPositiveTermExcluded},
	}
	for _, c := range cases {
		if _, err := NewSearchQuery(c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: error = %v, want %v", c.name, err, c.want)
		}
	}
}

func TestSearchQueryGroupsAndExcludedAreCopies(t *testing.T) {
	original := mkTerm(t, "keep", FieldAny, false, false)
	q, err := NewSearchQuery(SearchQueryInput{
		Groups:   [][]SearchTerm{{original}},
		Excluded: []SearchTerm{mkTerm(t, "out", FieldAny, false, true)},
	})
	if err != nil {
		t.Fatalf("NewSearchQuery: %v", err)
	}
	q.Groups()[0][0] = mkTerm(t, "mutated", FieldAny, false, false)
	if q.Groups()[0][0].Text() != "keep" {
		t.Error("Groups returned a shared slice")
	}
	q.Excluded()[0] = mkTerm(t, "mutated", FieldAny, false, true)
	if q.Excluded()[0].Text() != "out" {
		t.Error("Excluded returned a shared slice")
	}
}

func TestSearchQueryScopes(t *testing.T) {
	q, err := NewSearchQuery(SearchQueryInput{})
	if err != nil {
		t.Fatalf("NewSearchQuery: %v", err)
	}
	scoped := q.WithFolderScope(" f1 ").WithAccountScope(" a1 ")
	if scoped.ScopeFolderID() != "f1" || scoped.ScopeAccountID() != "a1" {
		t.Errorf("scopes = %q %q, want f1 a1", scoped.ScopeFolderID(), scoped.ScopeAccountID())
	}
	if q.ScopeFolderID() != "" || q.ScopeAccountID() != "" {
		t.Error("With* mutated the original")
	}
	// UI scope alone does not make a query non-empty.
	if !scoped.IsEmpty() || scoped.HasText() {
		t.Error("a scoped empty query must stay empty")
	}
}

func TestSearchQueryIsEmpty(t *testing.T) {
	cases := []struct {
		name string
		in   SearchQueryInput
	}{
		{"excluded", SearchQueryInput{Excluded: []SearchTerm{mkTerm(t, "x", FieldAny, false, true)}}},
		{"has attachment", SearchQueryInput{HasAttachment: true}},
		{"unread", SearchQueryInput{Unread: true}},
		{"read", SearchQueryInput{Read: true}},
		{"flagged", SearchQueryInput{Flagged: true}},
		{"folder", SearchQueryInput{FolderPath: "INBOX"}},
		{"account", SearchQueryInput{AccountName: "Work"}},
		{"after", SearchQueryInput{After: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
		{"before", SearchQueryInput{Before: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
	}
	for _, c := range cases {
		q, err := NewSearchQuery(c.in)
		if err != nil {
			t.Fatalf("%s: NewSearchQuery: %v", c.name, err)
		}
		if q.IsEmpty() {
			t.Errorf("%s: IsEmpty = true, want false", c.name)
		}
		if q.HasText() {
			t.Errorf("%s: HasText = true for a structural-only query", c.name)
		}
	}
	empty, err := NewSearchQuery(SearchQueryInput{})
	if err != nil {
		t.Fatalf("NewSearchQuery: %v", err)
	}
	if !empty.IsEmpty() {
		t.Error("a blank query must be empty")
	}
}
