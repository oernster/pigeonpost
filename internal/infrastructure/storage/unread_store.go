package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// The per-account unread aggregates that drive the sidebar badges and the new-mail attention cue. They
// share two exclusions. A message hidden by a snooze not yet due must not badge the folder it is hidden
// from, so each query excludes it at the given instant. The archive is excluded whole: archiving is the
// act of putting something out of the way, so it should not go on demanding attention at account level.
//
// On Gmail that second exclusion is load-bearing rather than a preference. Its archive is All Mail, which
// holds a copy of every labelled message, so without it one unread message in the inbox counted twice:
// once where it is filed and once in the archive. Deduplicating by Message-ID instead was measured and
// rejected: across two real accounts over 900 messages each share a Message-ID with another row, 601 of
// them inside a single folder, so it is not an identity key and deduplicating would undercount everywhere.
//
// The archive reports no unread on its own row either (see unreadCountExpr), so the rule is the same
// wherever a count is shown. It still reports its total, so the folder says what it holds.

// UnreadByAccount returns each account's unread message count summed across its folders other than the
// archive, keyed by account id. An account with no unread messages is absent from the map. Unread means
// the Seen bit is clear, matching the per-folder count. A message hidden by a snooze not yet due at
// visibleAt is not counted: hidden mail must not badge the folder it is hidden from.
func (s *Store) UnreadByAccount(ctx context.Context, visibleAt time.Time) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT f.account_id, COUNT(*) FROM message m JOIN folder f ON f.id = m.folder_id
		 WHERE (m.flags & %d) = 0
		   AND f.kind != %d
		   AND NOT EXISTS (SELECT 1 FROM message_snooze sn WHERE sn.message_id = m.id AND sn.until_ms > ?)
		 GROUP BY f.account_id;`, int(domain.FlagSeen), int(domain.FolderArchive)), visibleAt.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("query unread by account: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var (
			accountID string
			count     int
		)
		if err := rows.Scan(&accountID, &count); err != nil {
			return nil, fmt.Errorf("scan unread count: %w", err)
		}
		counts[accountID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unread counts: %w", err)
	}
	return counts, nil
}

// NewestUnreadByAccount returns each account's newest unread message date in Unix milliseconds,
// keyed by account id, over the same visible set as UnreadByAccount: the Seen bit is clear, no
// unexpired snooze hides the message at visibleAt and the archive is left out. An account with no unread messages is absent.
// The date is the message's own date rather than a local arrival instant, which is what the cache
// stores; for mail arriving through a sync the two are close enough to drive an attention cue.
func (s *Store) NewestUnreadByAccount(ctx context.Context, visibleAt time.Time) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT f.account_id, MAX(m.date_ms) FROM message m JOIN folder f ON f.id = m.folder_id
		 WHERE (m.flags & %d) = 0
		   AND f.kind != %d
		   AND NOT EXISTS (SELECT 1 FROM message_snooze sn WHERE sn.message_id = m.id AND sn.until_ms > ?)
		 GROUP BY f.account_id;`, int(domain.FlagSeen), int(domain.FolderArchive)), visibleAt.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("query newest unread by account: %w", err)
	}
	defer rows.Close()

	newest := make(map[string]int64)
	for rows.Next() {
		var (
			accountID string
			dateMs    int64
		)
		if err := rows.Scan(&accountID, &dateMs); err != nil {
			return nil, fmt.Errorf("scan newest unread: %w", err)
		}
		newest[accountID] = dateMs
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate newest unread: %w", err)
	}
	return newest, nil
}
