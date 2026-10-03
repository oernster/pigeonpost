package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// claimRacers is how many replays race for one item in the exclusivity test: enough that a claim
// which was a read followed by a write would let more than one through.
const claimRacers = 8

// Of any number of replays racing to claim the same queued item, exactly one may win. That one claim is
// what stops two syncs (or a sync and the dispatcher) from delivering the message twice.
func TestClaimOutboxIsExclusive(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.EnqueueOutbox(ctx, outboxTestItem(t, "q1", domain.OutboxSend)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range claimRacers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claimed, err := store.ClaimOutbox(ctx, "q1")
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			if claimed {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := wins.Load(); got != 1 {
		t.Fatalf("claims won = %d, want exactly 1", got)
	}
	if claimed, err := store.ClaimOutbox(ctx, "missing"); err != nil || claimed {
		t.Errorf("claim of a missing item = %v, %v; want false, nil", claimed, err)
	}
}

// A failed item waits for the user; no replay may claim it and so retry it behind their back.
func TestClaimOutboxRefusesFailedItem(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.EnqueueOutbox(ctx, outboxTestItem(t, "q1", domain.OutboxSend)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := store.MarkOutboxFailed(ctx, "q1", "rejected"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	if claimed, err := store.ClaimOutbox(ctx, "q1"); err != nil || claimed {
		t.Errorf("claim of a failed item = %v, %v; want false, nil", claimed, err)
	}
}

// A cancel that arrives while the item is being sent must not delete it and report success: the
// message may already be on the wire. Once released (the send did not deliver) the cancel works again.
func TestCancelQueuedOutboxRefusesClaimedItem(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.EnqueueOutbox(ctx, outboxTestItem(t, "q1", domain.OutboxSend)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if claimed, err := store.ClaimOutbox(ctx, "q1"); err != nil || !claimed {
		t.Fatalf("claim = %v, %v; want true, nil", claimed, err)
	}
	if cancelled, err := store.CancelQueuedOutbox(ctx, "q1"); err != nil || cancelled {
		t.Fatalf("cancel of a claimed item = %v, %v; want false, nil", cancelled, err)
	}
	if items, err := store.ListOutbox(ctx); err != nil || len(items) != 1 {
		t.Fatalf("after a refused cancel the item must remain, got %d items, err %v", len(items), err)
	}
	if err := store.ReleaseOutbox(ctx, "q1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if cancelled, err := store.CancelQueuedOutbox(ctx, "q1"); err != nil || !cancelled {
		t.Fatalf("cancel of a released item = %v, %v; want true, nil", cancelled, err)
	}
	if cancelled, err := store.CancelQueuedOutbox(ctx, "q1"); err != nil || cancelled {
		t.Errorf("cancel of a gone item = %v, %v; want false, nil", cancelled, err)
	}
}

// A run that died mid-send leaves its item claimed for ever unless something releases it. Recovery
// releases only claims stamped by another run, so it cannot hand a claim this run is using to a second
// replay.
func TestRecoverOutboxClaimsReleasesOnlyEarlierRuns(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	for _, id := range []string{"ours", "orphan"} {
		if err := store.EnqueueOutbox(ctx, outboxTestItem(t, id, domain.OutboxSend)); err != nil {
			t.Fatalf("enqueue %s: %v", id, err)
		}
	}
	if claimed, err := store.ClaimOutbox(ctx, "ours"); err != nil || !claimed {
		t.Fatalf("claim = %v, %v; want true, nil", claimed, err)
	}
	if _, err := store.db.ExecContext(ctx,
		"UPDATE outbox SET send_state = ?, claim_owner = ? WHERE id = ?;",
		outboxStateSending, "an-earlier-run", "orphan"); err != nil {
		t.Fatalf("plant orphaned claim: %v", err)
	}

	released, err := store.RecoverOutboxClaims(ctx)
	if err != nil || released != 1 {
		t.Fatalf("recover = %d, %v; want 1, nil", released, err)
	}
	if claimed, err := store.ClaimOutbox(ctx, "orphan"); err != nil || !claimed {
		t.Errorf("the orphan must be claimable again, got %v, %v", claimed, err)
	}
	if claimed, err := store.ClaimOutbox(ctx, "ours"); err != nil || claimed {
		t.Errorf("this run's claim must survive recovery, got %v, %v", claimed, err)
	}
}

// preClaimVersion is the version schemaV59 upgrades FROM, fixed rather than derived from schemaVersion
// so later steps cannot move this test off the step it covers.
const preClaimVersion = outboxClaimSchemaVersion - 1

// migrateRawTo applies the migrations up to version on a fresh database file and returns its path.
func migrateRawTo(t *testing.T, version int) (string, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "upgrade.db")
	db, err := sql.Open(driverName, path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, step := range migrations[:version] {
		if _, err := db.ExecContext(context.Background(), step); err != nil {
			t.Fatalf("apply migrations up to %d: %v", version, err)
		}
	}
	if _, err := db.ExecContext(context.Background(), fmt.Sprintf("PRAGMA user_version = %d;", version)); err != nil {
		t.Fatalf("set version: %v", err)
	}
	return path, db
}

// An outbox row queued before the upgrade must come through it queued, so the first replay after the
// upgrade claims and sends it like any other.
func TestOutboxClaimMigrationKeepsExistingRowsQueued(t *testing.T) {
	if migrations[outboxClaimSchemaVersion-1] != schemaV59 {
		t.Fatalf("schemaV59 is not in slot %d of the migrations list", outboxClaimSchemaVersion)
	}
	ctx := context.Background()
	path, db := migrateRawTo(t, preClaimVersion)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO outbox (id, account_id, kind, from_display, from_address, to_json, cc_json, subject,
		        body, html_body, created_ms)
		 VALUES ('legacy', 'a1', 0, 'Me', 'me@example.com', '[{"display":"","address":"f@example.com"}]',
		        '[]', 'Old', 'hi', '', 1000);`); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open upgraded store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if claimed, err := store.ClaimOutbox(ctx, "legacy"); err != nil || !claimed {
		t.Errorf("a pre-upgrade row must be claimable, got %v, %v", claimed, err)
	}
}

// The step's two ALTERs and its version commit together: a step that fails part way (here the second
// column already exists) leaves neither the first column nor a bumped version behind, so nothing
// half-applied is left for a re-run to trip over.
func TestOutboxClaimMigrationIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	path, db := migrateRawTo(t, preClaimVersion)
	if _, err := db.ExecContext(ctx, "ALTER TABLE outbox ADD COLUMN claim_owner TEXT NOT NULL DEFAULT '';"); err != nil {
		t.Fatalf("plant clashing column: %v", err)
	}
	if _, err := db.ExecContext(ctx, schemaV59); err == nil {
		t.Fatal("the step must fail loudly on a clashing column")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}
	reopened, err := sql.Open(driverName, path)
	if err != nil {
		t.Fatalf("reopen raw db: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	var version, sendStateColumns int
	if err := reopened.QueryRowContext(ctx, "PRAGMA user_version;").Scan(&version); err != nil {
		t.Fatalf("read version: %v", err)
	}
	if err := reopened.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pragma_table_info('outbox') WHERE name = 'send_state';").Scan(&sendStateColumns); err != nil {
		t.Fatalf("read columns: %v", err)
	}
	if version != preClaimVersion || sendStateColumns != 0 {
		t.Errorf("after a failed step: version %d (want %d), send_state columns %d (want 0)",
			version, preClaimVersion, sendStateColumns)
	}
}
