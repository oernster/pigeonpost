package storage

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
)

// The two send states an outbox row moves between (see schemaV59). A row is deleted once its operation
// has been delivered, so there is no third state for "sent".
const (
	outboxStateQueued  = "queued"
	outboxStateSending = "sending"
)

// outboxClaimOwner identifies this process run on the claims it takes. It is the process id plus the
// instant the package was initialised. That is unique per run without needing randomness; it never
// equals the owner a crashed earlier run stamped, which is what lets RecoverOutboxClaims release only
// the claims nobody alive holds.
var outboxClaimOwner = strconv.Itoa(os.Getpid()) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)

// ClaimOutbox moves a queued, unfailed item to sending and reports whether this call did so. It is one
// conditional UPDATE, so of any number of replays racing for the same item exactly one sees a changed
// row; every other one gets false and must leave the item alone. False also covers an item already
// gone (cancelled or delivered) and one marked failed, which no replay retries.
func (s *Store) ClaimOutbox(ctx context.Context, id string) (bool, error) {
	result, err := s.db.ExecContext(ctx,
		`UPDATE outbox SET send_state = ?, claim_owner = ?
		 WHERE id = ? AND send_state = ? AND failure = '';`,
		outboxStateSending, outboxClaimOwner, id, outboxStateQueued)
	if err != nil {
		return false, fmt.Errorf("claim outbox item %q: %w", id, err)
	}
	return changedOne(result, "claim outbox item", id)
}

// ReleaseOutbox returns a claimed item to queued, for a send that did not deliver: the server was
// unreachable or the operation failed (in which case its failure reason is already stamped, which keeps
// it from being claimed again until the user acts).
func (s *Store) ReleaseOutbox(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx,
		"UPDATE outbox SET send_state = ?, claim_owner = '' WHERE id = ?;", outboxStateQueued, id); err != nil {
		return fmt.Errorf("release outbox item %q: %w", id, err)
	}
	return nil
}

// CancelQueuedOutbox deletes an item only while it is still queued and reports whether it did. The
// state test and the delete are one statement, so a cancel can never remove an item a replay has
// already claimed. False means the message is being sent or has left; the user is told so.
func (s *Store) CancelQueuedOutbox(ctx context.Context, id string) (bool, error) {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM outbox WHERE id = ? AND send_state = ?;", id, outboxStateQueued)
	if err != nil {
		return false, fmt.Errorf("cancel outbox item %q: %w", id, err)
	}
	return changedOne(result, "cancel outbox item", id)
}

// RecoverOutboxClaims returns to queued every item left sending by an earlier run (one that crashed or
// was killed mid-send) and reports how many it released. Claims this run holds are untouched, so it is
// safe to call while a replay is in flight.
func (s *Store) RecoverOutboxClaims(ctx context.Context) (int, error) {
	result, err := s.db.ExecContext(ctx,
		`UPDATE outbox SET send_state = ?, claim_owner = ''
		 WHERE send_state = ? AND claim_owner != ?;`,
		outboxStateQueued, outboxStateSending, outboxClaimOwner)
	if err != nil {
		return 0, fmt.Errorf("recover outbox claims: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("recover outbox claims: %w", err)
	}
	return int(affected), nil
}

// rowsAffecter is the part of sql.Result the conditional writes read.
type rowsAffecter interface {
	RowsAffected() (int64, error)
}

// changedOne reports whether a conditional write by id changed a row, wrapping a failure to read the
// count in the operation's own words.
func changedOne(result rowsAffecter, what, id string) (bool, error) {
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("%s %q: %w", what, id, err)
	}
	return affected > 0, nil
}
