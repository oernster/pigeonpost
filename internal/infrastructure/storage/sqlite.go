// Package storage implements the application AccountStore and MailStore interfaces on top of a
// local, pure-Go SQLite database (modernc.org/sqlite, no CGO).
package storage

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// schemaVersion is the current on-disk schema version, tracked via SQLite's PRAGMA user_version.
const schemaVersion = 1

const driverName = "sqlite"

// schemaV1 is the initial schema. Statements are idempotent so re-running is safe.
const schemaV1 = `
CREATE TABLE IF NOT EXISTS account (
    id           TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    email        TEXT NOT NULL,
    protocol     INTEGER NOT NULL,
    in_host      TEXT NOT NULL,
    in_port      INTEGER NOT NULL,
    in_security  INTEGER NOT NULL,
    out_host     TEXT NOT NULL,
    out_port     INTEGER NOT NULL,
    out_security INTEGER NOT NULL,
    auth         INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS folder (
    id         TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    path       TEXT NOT NULL,
    kind       INTEGER NOT NULL,
    unread     INTEGER NOT NULL,
    total      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_folder_account ON folder(account_id);

CREATE TABLE IF NOT EXISTS message (
    id              TEXT PRIMARY KEY,
    folder_id       TEXT NOT NULL,
    uid             INTEGER NOT NULL,
    message_id      TEXT NOT NULL,
    from_display    TEXT NOT NULL,
    from_address    TEXT NOT NULL,
    subject         TEXT NOT NULL,
    date_ms         INTEGER NOT NULL,
    size            INTEGER NOT NULL,
    flags           INTEGER NOT NULL,
    has_attachments INTEGER NOT NULL,
    snippet         TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_message_folder ON message(folder_id);

CREATE VIRTUAL TABLE IF NOT EXISTS message_fts USING fts5(
    subject,
    snippet,
    from_address,
    content=''
);
`

// Store is the SQLite-backed implementation of the application storage ports.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the database at path and applies pending migrations. Use ":memory:" for a
// transient in-memory database in tests.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	// Local-first single process: one writer, WAL for concurrent readers, wait rather than fail on
	// a busy lock.
	for _, pragma := range []string{
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA foreign_keys = ON;",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("apply pragma %q: %w", pragma, err)
		}
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
	if _, err := s.db.ExecContext(ctx, schemaV1); err != nil {
		return fmt.Errorf("apply schema v1: %w", err)
	}
	// PRAGMA user_version does not accept a bound parameter, so the trusted constant is formatted in.
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d;", schemaVersion)); err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}
	return nil
}
