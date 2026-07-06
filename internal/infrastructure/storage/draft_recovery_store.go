package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// draftRecoveryRowID is the fixed primary key of the single draft-recovery snapshot. Only one
// in-progress compose is recovered at a time, so a save replaces the row at this id.
const draftRecoveryRowID = 1

// SaveDraftRecovery stores the in-progress compose snapshot, replacing any previous one so only the most
// recent survives. The recipient fields are kept verbatim as the user typed them.
func (s *Store) SaveDraftRecovery(ctx context.Context, recovery domain.DraftRecovery) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO draft_recovery
		     (id, account_id, to_addrs, cc_addrs, bcc_addrs, subject, body_html, saved_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?);`,
		draftRecoveryRowID, recovery.AccountID(), recovery.To(), recovery.Cc(), recovery.Bcc(),
		recovery.Subject(), recovery.BodyHTML(), recovery.SavedAt().UnixMilli()); err != nil {
		return fmt.Errorf("save draft recovery: %w", err)
	}
	return nil
}

// GetDraftRecovery returns the stored snapshot. The boolean is false, with no error, when none is held.
func (s *Store) GetDraftRecovery(ctx context.Context) (domain.DraftRecovery, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT account_id, to_addrs, cc_addrs, bcc_addrs, subject, body_html, saved_ms
		 FROM draft_recovery WHERE id = ?;`, draftRecoveryRowID)
	var (
		accountID         string
		to, cc, bcc       string
		subject, bodyHTML string
		savedMS           int64
	)
	if err := row.Scan(&accountID, &to, &cc, &bcc, &subject, &bodyHTML, &savedMS); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.DraftRecovery{}, false, nil
		}
		return domain.DraftRecovery{}, false, fmt.Errorf("scan draft recovery: %w", err)
	}
	recovery, err := domain.NewDraftRecovery(domain.DraftRecoveryInput{
		AccountID: accountID, To: to, Cc: cc, Bcc: bcc, Subject: subject, BodyHTML: bodyHTML,
	}, time.UnixMilli(savedMS).UTC())
	if err != nil {
		return domain.DraftRecovery{}, false, fmt.Errorf("rebuild draft recovery: %w", err)
	}
	return recovery, true, nil
}

// ClearDraftRecovery removes the snapshot once the message has been sent, saved to the server, or the
// user has discarded it.
func (s *Store) ClearDraftRecovery(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM draft_recovery WHERE id = ?;", draftRecoveryRowID); err != nil {
		return fmt.Errorf("clear draft recovery: %w", err)
	}
	return nil
}
