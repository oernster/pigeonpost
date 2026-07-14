package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// bm25 column weights for ranking, one per message_search column in declaration order (message_id is
// unindexed and weighted zero). Subject and sender outrank the body so the obvious hit surfaces first;
// the snippet is a weak body proxy so it sits between them.
const (
	searchWeightSubject    = 4.0
	searchWeightSnippet    = 1.5
	searchWeightSender     = 3.0
	searchWeightRecipients = 2.0
	searchWeightBody       = 1.0
	searchWeightFilenames  = 2.0
)

// searchSnippetTokens is how many tokens of context snippet() returns around the matched terms.
const searchSnippetTokens = 12

// searchInsertSQL (re)indexes messages from the message_searchable_text view, the single definition of
// a message's searchable text. Every insert site appends its own WHERE over the view's columns, so the
// indexed shape can never drift between the sync path, the body-cache path and the schema backfill.
const searchInsertSQL = `INSERT INTO message_search (message_id, subject, snippet, sender, recipients, body, filenames)
	SELECT message_id, subject, snippet, sender, recipients, body, filenames FROM message_searchable_text`

// ftsColumns maps the field-constrained search operators to their index columns. FieldAny is absent:
// an unconstrained term carries no column filter and matches every indexed column.
var ftsColumns = map[domain.SearchField]string{
	domain.FieldFrom:     "sender",
	domain.FieldTo:       "recipients",
	domain.FieldSubject:  "subject",
	domain.FieldFilename: "filenames",
}

// SearchMessages returns the cached messages matching the query, most relevant first, capped at limit.
// The matched-term snippet of each hit wraps matches in the application's search match markers.
func (s *Store) SearchMessages(ctx context.Context, query domain.SearchQuery, limit int) ([]application.SearchHit, error) {
	sqlText, args := buildSearchSQL(query, limit)
	return queryRows(ctx, s.db, "search results", sqlText, scanSearchHit, args...)
}

// buildSearchSQL translates a SearchQuery into one SQL statement. Positive text becomes a single FTS5
// MATCH; negated terms become one NOT IN sub-match; the structural predicates (status flags,
// attachment flag, folder and account scope, date bounds) stay relational so they need no index
// maintenance of their own. A query with no positive text skips the index entirely and filters the
// message table directly, ordered by date instead of rank.
func buildSearchSQL(q domain.SearchQuery, limit int) (string, []any) {
	var sb strings.Builder
	args := make([]any, 0, 8)
	sb.WriteString(`SELECT m.id, m.folder_id, m.uid, m.message_id, m.from_display, m.from_address,
		m.to_json, m.cc_json, m.subject, m.date_ms, m.size, m.flags, m.has_attachments, m.snippet, `)
	if q.HasText() {
		// char(1)/char(2) are the application.SearchMatchStart/SearchMatchEnd markers.
		fmt.Fprintf(&sb, "snippet(message_search, -1, char(1), char(2), '…', %d)", searchSnippetTokens)
		sb.WriteString(" FROM message_search JOIN message m ON m.id = message_search.message_id")
	} else {
		sb.WriteString("'' FROM message m")
	}
	sb.WriteString(" JOIN folder f ON f.id = m.folder_id")
	if q.AccountName() != "" {
		sb.WriteString(" JOIN account a ON a.id = f.account_id")
	}
	sb.WriteString(" WHERE 1 = 1")
	if q.HasText() {
		sb.WriteString(" AND message_search MATCH ?")
		args = append(args, ftsMatchExpr(q.Groups()))
	}
	if excluded := q.Excluded(); len(excluded) > 0 {
		sb.WriteString(" AND m.id NOT IN (SELECT message_id FROM message_search WHERE message_search MATCH ?)")
		args = append(args, ftsExcludeExpr(excluded))
	}
	if q.HasAttachment() {
		sb.WriteString(" AND m.has_attachments = 1")
	}
	if q.Unread() {
		fmt.Fprintf(&sb, " AND (m.flags & %d) = 0", int(domain.FlagSeen))
	}
	if q.Read() {
		fmt.Fprintf(&sb, " AND (m.flags & %d) <> 0", int(domain.FlagSeen))
	}
	if q.Flagged() {
		fmt.Fprintf(&sb, " AND (m.flags & %d) <> 0", int(domain.FlagFlagged))
	}
	if !q.After().IsZero() {
		sb.WriteString(" AND m.date_ms >= ?")
		args = append(args, q.After().UnixMilli())
	}
	if !q.Before().IsZero() {
		sb.WriteString(" AND m.date_ms < ?")
		args = append(args, q.Before().UnixMilli())
	}
	if q.FolderPath() != "" {
		sb.WriteString(" AND LOWER(f.path) = LOWER(?)")
		args = append(args, q.FolderPath())
	}
	if q.AccountName() != "" {
		sb.WriteString(" AND (LOWER(a.display_name) = LOWER(?) OR LOWER(a.email) = LOWER(?))")
		args = append(args, q.AccountName(), q.AccountName())
	}
	if q.ScopeFolderID() != "" {
		sb.WriteString(" AND m.folder_id = ?")
		args = append(args, q.ScopeFolderID())
	}
	if q.ScopeAccountID() != "" {
		sb.WriteString(" AND f.account_id = ?")
		args = append(args, q.ScopeAccountID())
	}
	if q.HasText() {
		fmt.Fprintf(&sb, " ORDER BY bm25(message_search, 0, %g, %g, %g, %g, %g, %g), m.date_ms DESC",
			searchWeightSubject, searchWeightSnippet, searchWeightSender,
			searchWeightRecipients, searchWeightBody, searchWeightFilenames)
	} else {
		sb.WriteString(" ORDER BY m.date_ms DESC")
	}
	sb.WriteString(" LIMIT ?")
	args = append(args, limit)
	return sb.String(), args
}

// ftsMatchExpr renders the positive text as one FTS5 MATCH expression: groups are ANDed and the terms
// inside a group are ORed, mirroring the domain query shape.
func ftsMatchExpr(groups [][]domain.SearchTerm) string {
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		terms := make([]string, 0, len(group))
		for _, term := range group {
			terms = append(terms, ftsTermExpr(term))
		}
		if len(terms) == 1 {
			parts = append(parts, terms[0])
			continue
		}
		parts = append(parts, "("+strings.Join(terms, " OR ")+")")
	}
	return strings.Join(parts, " AND ")
}

// ftsExcludeExpr renders the negated terms as one FTS5 expression matching anything that must be
// excluded: a message matching any of them is filtered out.
func ftsExcludeExpr(excluded []domain.SearchTerm) string {
	terms := make([]string, 0, len(excluded))
	for _, term := range excluded {
		terms = append(terms, ftsTermExpr(term))
	}
	return strings.Join(terms, " OR ")
}

// ftsTermExpr renders one term as a safe FTS5 query item: the text is always string-quoted (so user
// punctuation can never be read as FTS syntax), a word gets a prefix star so typing matches as it
// goes, a phrase stays exact, and a field constraint becomes a column filter.
func ftsTermExpr(term domain.SearchTerm) string {
	expr := `"` + strings.ReplaceAll(term.Text(), `"`, `""`) + `"`
	if !term.IsPhrase() {
		expr += "*"
	}
	if column, ok := ftsColumns[term.Field()]; ok {
		expr = column + ":" + expr
	}
	return expr
}

// scanSearchHit reads one search result row: a full message summary plus the matched-term snippet.
func scanSearchHit(row scanner) (application.SearchHit, error) {
	var (
		r       messageRow
		snippet string
	)
	if err := row.Scan(append(r.scanFields(), &snippet)...); err != nil {
		return application.SearchHit{}, fmt.Errorf("scan search hit: %w", err)
	}
	summary, err := r.build()
	if err != nil {
		return application.SearchHit{}, err
	}
	return application.SearchHit{Summary: summary, Snippet: snippet}, nil
}
