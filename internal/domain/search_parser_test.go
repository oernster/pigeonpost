package domain

import (
	"testing"
	"time"
)

// wantTerm is a compact expected-term literal for the parser tables.
type wantTerm struct {
	text    string
	field   SearchField
	phrase  bool
	negated bool
}

func assertTerms(t *testing.T, name string, got []SearchTerm, want []wantTerm) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d terms %+v, want %d", name, len(got), got, len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.Text() != w.text || g.Field() != w.field || g.IsPhrase() != w.phrase || g.IsNegated() != w.negated {
			t.Errorf("%s[%d] = {%q %v %v %v}, want {%q %v %v %v}", name, i,
				g.Text(), g.Field(), g.IsPhrase(), g.IsNegated(), w.text, w.field, w.phrase, w.negated)
		}
	}
}

func flatGroups(q SearchQuery) [][]SearchTerm { return q.Groups() }

func TestParseBareTermsAndPhrases(t *testing.T) {
	q := ParseSearchQuery(`alpha  "two words" beta`, time.UTC)
	groups := flatGroups(q)
	if len(groups) != 3 {
		t.Fatalf("groups = %+v, want 3", groups)
	}
	assertTerms(t, "g0", groups[0], []wantTerm{{"alpha", FieldAny, false, false}})
	assertTerms(t, "g1", groups[1], []wantTerm{{"two words", FieldAny, true, false}})
	assertTerms(t, "g2", groups[2], []wantTerm{{"beta", FieldAny, false, false}})
	if q.IsDegraded() {
		t.Error("well-formed input must not be degraded")
	}
}

func TestParseFieldOperators(t *testing.T) {
	q := ParseSearchQuery(`from:bob TO:alice subject:"quarterly report" filename:invoice.pdf`, time.UTC)
	groups := flatGroups(q)
	if len(groups) != 4 {
		t.Fatalf("groups = %+v, want 4", groups)
	}
	assertTerms(t, "from", groups[0], []wantTerm{{"bob", FieldFrom, false, false}})
	assertTerms(t, "to", groups[1], []wantTerm{{"alice", FieldTo, false, false}})
	assertTerms(t, "subject", groups[2], []wantTerm{{"quarterly report", FieldSubject, true, false}})
	assertTerms(t, "filename", groups[3], []wantTerm{{"invoice.pdf", FieldFilename, false, false}})
}

func TestParseNegation(t *testing.T) {
	q := ParseSearchQuery(`keep -noise -from:bob -"bad phrase"`, time.UTC)
	groups := flatGroups(q)
	if len(groups) != 1 {
		t.Fatalf("groups = %+v, want just keep", groups)
	}
	assertTerms(t, "excluded", q.Excluded(), []wantTerm{
		{"noise", FieldAny, false, true},
		{"bob", FieldFrom, false, true},
		{"bad phrase", FieldAny, true, true},
	})
}

func TestParseOrGroups(t *testing.T) {
	q := ParseSearchQuery(`urgent OR critical OR blocker beta`, time.UTC)
	groups := flatGroups(q)
	if len(groups) != 2 {
		t.Fatalf("groups = %+v, want or-group plus beta", groups)
	}
	assertTerms(t, "or-group", groups[0], []wantTerm{
		{"urgent", FieldAny, false, false},
		{"critical", FieldAny, false, false},
		{"blocker", FieldAny, false, false},
	})
	assertTerms(t, "beta", groups[1], []wantTerm{{"beta", FieldAny, false, false}})
}

func TestParseOrJoinsFieldTerms(t *testing.T) {
	q := ParseSearchQuery(`from:bob OR to:alice`, time.UTC)
	groups := flatGroups(q)
	if len(groups) != 1 {
		t.Fatalf("groups = %+v, want one or-group", groups)
	}
	assertTerms(t, "or-group", groups[0], []wantTerm{
		{"bob", FieldFrom, false, false},
		{"alice", FieldTo, false, false},
	})
}

func TestParseDanglingOrIsLiteral(t *testing.T) {
	cases := []struct {
		name, raw string
		groups    []wantTerm // flattened single-term groups, in order
	}{
		{"leading", "OR alpha", []wantTerm{{"OR", FieldAny, false, false}, {"alpha", FieldAny, false, false}}},
		{"trailing", "alpha OR", []wantTerm{{"alpha", FieldAny, false, false}, {"OR", FieldAny, false, false}}},
		{"before negated", "alpha OR -beta", []wantTerm{{"alpha", FieldAny, false, false}, {"OR", FieldAny, false, false}}},
		{"before structural", "alpha OR is:unread", []wantTerm{{"alpha", FieldAny, false, false}, {"OR", FieldAny, false, false}}},
	}
	for _, c := range cases {
		q := ParseSearchQuery(c.raw, time.UTC)
		groups := flatGroups(q)
		if len(groups) != len(c.groups) {
			t.Fatalf("%s: groups = %+v, want %d single terms", c.name, groups, len(c.groups))
		}
		for i, w := range c.groups {
			assertTerms(t, c.name, groups[i], []wantTerm{w})
		}
	}
}

func TestParseStructuralOperators(t *testing.T) {
	q := ParseSearchQuery(`in:INBOX account:Work has:attachment is:unread is:read is:flagged`, time.UTC)
	if q.FolderPath() != "INBOX" || q.AccountName() != "Work" {
		t.Errorf("scopes = %q %q", q.FolderPath(), q.AccountName())
	}
	if !q.HasAttachment() || !q.Unread() || !q.Read() || !q.Flagged() {
		t.Error("status predicates not set")
	}
	if q.HasText() || q.IsEmpty() {
		t.Error("structural-only query: HasText false, IsEmpty false expected")
	}
}

func TestParseStructuralLastOccurrenceWins(t *testing.T) {
	q := ParseSearchQuery(`in:INBOX in:Archive account:Home account:Work`, time.UTC)
	if q.FolderPath() != "Archive" || q.AccountName() != "Work" {
		t.Errorf("last occurrence must win: %q %q", q.FolderPath(), q.AccountName())
	}
}

func TestParseDates(t *testing.T) {
	zone := time.FixedZone("test", -5*3600)
	q := ParseSearchQuery(`after:2026-01-01 before:2026-02-01`, zone)
	wantAfter := time.Date(2026, 1, 1, 0, 0, 0, 0, zone)
	wantBefore := time.Date(2026, 2, 1, 0, 0, 0, 0, zone)
	if !q.After().Equal(wantAfter) || !q.Before().Equal(wantBefore) {
		t.Errorf("bounds = %v %v, want %v %v", q.After(), q.Before(), wantAfter, wantBefore)
	}

	on := ParseSearchQuery(`on:2026-03-10`, zone)
	dayStart := time.Date(2026, 3, 10, 0, 0, 0, 0, zone)
	if !on.After().Equal(dayStart) || !on.Before().Equal(dayStart.AddDate(0, 0, 1)) {
		t.Errorf("on: bounds = %v %v, want the whole day from %v", on.After(), on.Before(), dayStart)
	}
}

func TestParseUnrecognisedFallsBackToLiteralText(t *testing.T) {
	cases := []struct {
		name, raw, literal string
	}{
		{"unknown operator", "foo:bar", "foo:bar"},
		{"bad has value", "has:pigeons", "has:pigeons"},
		{"bad is value", "is:sleepy", "is:sleepy"},
		{"bad date", "before:soon", "before:soon"},
		{"negated structural", "-is:unread", "-is:unread"},
		{"empty in", "in:", "in:"},
		{"empty account", "account:", "account:"},
		{"bare field operator", "from:", "from:"},
		{"lone dash", "-", "-"},
		{"mid-word colon", "example.com:8080", "example.com:8080"},
	}
	for _, c := range cases {
		q := ParseSearchQuery(c.raw, time.UTC)
		groups := flatGroups(q)
		if len(groups) != 1 {
			t.Fatalf("%s: groups = %+v, want one literal", c.name, groups)
		}
		assertTerms(t, c.name, groups[0], []wantTerm{{c.literal, FieldAny, false, false}})
		if q.IsDegraded() {
			t.Errorf("%s: literal fallback must not degrade the whole query", c.name)
		}
	}
}

func TestParseUnclosedQuoteDegrades(t *testing.T) {
	q := ParseSearchQuery(`alpha "beta gamma`, time.UTC)
	if !q.IsDegraded() {
		t.Fatal("unclosed quote must degrade")
	}
	groups := flatGroups(q)
	if len(groups) != 3 {
		t.Fatalf("groups = %+v, want the three words", groups)
	}
	assertTerms(t, "g0", groups[0], []wantTerm{{"alpha", FieldAny, false, false}})
	assertTerms(t, "g1", groups[1], []wantTerm{{"beta", FieldAny, false, false}})
	assertTerms(t, "g2", groups[2], []wantTerm{{"gamma", FieldAny, false, false}})
}

func TestParseDegradeSkipsQuoteOnlyWords(t *testing.T) {
	q := ParseSearchQuery(`alpha " beta`, time.UTC)
	if !q.IsDegraded() {
		t.Fatal("unclosed quote must degrade")
	}
	groups := flatGroups(q)
	if len(groups) != 2 {
		t.Fatalf("groups = %+v, want alpha and beta only", groups)
	}
}

func TestParseEmptyAndBlankInput(t *testing.T) {
	for _, raw := range []string{"", "   ", `""`} {
		q := ParseSearchQuery(raw, time.UTC)
		if !q.IsEmpty() {
			t.Errorf("%q: query must be empty", raw)
		}
	}
}

func TestParseOrAcrossBlankPhraseDoesNotJoin(t *testing.T) {
	// The blank phrase after OR yields no term, so the pending OR is consumed with it: a and b stay
	// separate ANDed groups rather than being silently widened into (a OR b).
	q := ParseSearchQuery(`a OR " " b`, time.UTC)
	groups := flatGroups(q)
	if len(groups) != 2 {
		t.Fatalf("groups = %+v, want a and b separately", groups)
	}
	assertTerms(t, "a", groups[0], []wantTerm{{"a", FieldAny, false, false}})
	assertTerms(t, "b", groups[1], []wantTerm{{"b", FieldAny, false, false}})
}

func TestParseNegatedEmptyQuotesSearchesDash(t *testing.T) {
	// -"" carries no text; its literal reconstruction is just the dash, searched as a word.
	q := ParseSearchQuery(`-""`, time.UTC)
	groups := flatGroups(q)
	if len(groups) != 1 {
		t.Fatalf("groups = %+v, want the literal dash", groups)
	}
	assertTerms(t, "dash", groups[0], []wantTerm{{"-", FieldAny, false, false}})
}
