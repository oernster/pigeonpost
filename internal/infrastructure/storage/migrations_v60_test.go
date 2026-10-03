package storage

import (
	"context"
	"strings"
	"testing"
)

// preSequenceVersion is the version schemaV60 upgrades FROM, fixed rather than derived from schemaVersion
// so later steps cannot move this test off the step it covers.
const preSequenceVersion = eventSequenceSchemaVersion - 1

// An event saved before the upgrade must come through it as revision 0; the upgraded table must take a
// revision from then on.
func TestSequenceMigrationReadsExistingEventsAsRevisionZero(t *testing.T) {
	if migrations[eventSequenceSchemaVersion-1] != schemaV60 {
		t.Fatalf("schemaV60 is not in slot %d of the migrations list", eventSequenceSchemaVersion)
	}
	ctx := context.Background()
	path, db := migrateRawTo(t, preSequenceVersion)
	// The pre-upgrade row is every column but the new last one, encoded exactly as the store encodes it.
	legacy := sequencedMeeting(t, 0).WithID("old")
	args, err := eventInsertArgs(legacy)
	if err != nil {
		t.Fatalf("encode legacy event: %v", err)
	}
	columns := eventColumnList[:len(eventColumnList)-1]
	insert := "INSERT INTO event (" + strings.Join(columns, ", ") + ") VALUES (" +
		strings.TrimSuffix(strings.Repeat("?, ", len(columns)), ", ") + ");"
	if _, err := db.ExecContext(ctx, insert, args[:len(columns)]...); err != nil {
		t.Fatalf("insert legacy event: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open upgraded store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	old, err := store.GetEvent(ctx, "old")
	if err != nil || old.Sequence() != 0 {
		t.Fatalf("legacy event = revision %d (%v), want 0", old.Sequence(), err)
	}
	if err := store.SaveEvent(ctx, sequencedMeeting(t, 2)); err != nil {
		t.Fatalf("SaveEvent after upgrade: %v", err)
	}
	if got, err := store.GetEvent(ctx, "uid-m"); err != nil || got.Sequence() != 2 {
		t.Errorf("upgraded store read revision %d (%v), want 2", got.Sequence(), err)
	}
}

// The step records its own version inside its transaction, so a database the step has run on is at 60
// even before the runner's final bump; reopening it does not re-run the non-idempotent ALTER.
func TestSequenceMigrationRecordsItsOwnVersion(t *testing.T) {
	ctx := context.Background()
	_, db := migrateRawTo(t, preSequenceVersion)
	if _, err := db.ExecContext(ctx, schemaV60); err != nil {
		t.Fatalf("apply schemaV60: %v", err)
	}
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version;").Scan(&version); err != nil {
		t.Fatalf("read version: %v", err)
	}
	if version != eventSequenceSchemaVersion {
		t.Errorf("user_version = %d after the step, want %d", version, eventSequenceSchemaVersion)
	}
}
