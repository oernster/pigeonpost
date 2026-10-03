package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// The store must satisfy the whole MailStore port, RehomeMessages included; a drifted signature is a
// build failure here rather than one reported from wherever the store is next wired.
var _ application.MailStore = (*Store)(nil)

// RehomeMessages files messages PigeonPost has just moved into their destination folder's cache (each
// under the id it carries there) then indexes them for search, all in one transaction. It is part of
// the application.MailStore port.
//
// Unlike SaveMessages it touches only the rows it is given: it must not replace the destination's whole
// set, which a move into a large folder would turn into a rewrite of every row. A row the destination
// already holds (a sync that got there first; a repeated call) is kept as it is, so the call is
// idempotent. Without this the destination's next sync read the moved message as an arrival; the inbox
// rules could then destroy mail the user had just moved there.
func (s *Store) RehomeMessages(ctx context.Context, messages []domain.MessageSummary) error {
	if len(messages) == 0 {
		return nil
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for _, m := range messages {
			added, err := insertMessageRow(ctx, tx, m, true)
			if err != nil {
				return err
			}
			if !added {
				continue
			}
			if _, err := tx.ExecContext(ctx, searchInsertSQL+" WHERE message_id = ?;", m.ID()); err != nil {
				return fmt.Errorf("index moved message %q: %w", m.ID(), err)
			}
		}
		return nil
	})
}
