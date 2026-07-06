// Package storage implements the application AccountStore and MailStore interfaces on top of a
// local, pure-Go SQLite database (modernc.org/sqlite, no CGO).
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

const driverName = "sqlite"

// Store is the SQLite-backed implementation of the application storage ports.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the database at path and applies pending migrations. Use ":memory:" for a
// transient in-memory database in tests.
func Open(ctx context.Context, path string) (*Store, error) {
	// The pragmas live in the DSN so the driver applies them to every pooled connection, not just the
	// first. busy_timeout and foreign_keys are per-connection settings: run once via Exec they leave the
	// pool's other connections without them, which is what made a concurrent write fail immediately with
	// SQLITE_BUSY instead of waiting. The full set gives every connection the same ACID behaviour:
	//   - busy_timeout(5000): a writer blocked by another waits up to 5s rather than failing (isolation).
	//   - journal_mode(WAL): readers see a consistent snapshot while one writer proceeds (isolation).
	//   - synchronous(NORMAL): commits are durable across an application crash, the common case, while
	//     avoiding an fsync on every write (durability).
	//   - foreign_keys(ON): referential constraints are enforced so no operation can leave orphan rows
	//     (consistency).
	//   - _txlock=immediate: every transaction takes the write lock at BEGIN, so two writers serialise
	//     cleanly instead of both starting as readers and deadlocking on the upgrade (atomicity).
	dsn := path + "?" + strings.Join([]string{
		"_pragma=busy_timeout(5000)",
		"_pragma=journal_mode(WAL)",
		"_pragma=synchronous(NORMAL)",
		"_pragma=foreign_keys(ON)",
		"_txlock=immediate",
	}, "&")
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	store := &Store{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version;").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version >= schemaVersion {
		return nil
	}
	for step := version; step < schemaVersion; step++ {
		if _, err := s.db.ExecContext(ctx, migrations[step]); err != nil {
			return fmt.Errorf("apply schema step %d: %w", step+1, err)
		}
	}
	// PRAGMA user_version does not accept a bound parameter, so the trusted constant is formatted in.
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d;", schemaVersion)); err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}
	return nil
}
