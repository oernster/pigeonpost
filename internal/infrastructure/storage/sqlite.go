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
const schemaVersion = 4

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

// schemaV2 adds the coloured-tag tables: tags and their many-to-many link to messages.
const schemaV2 = `
CREATE TABLE IF NOT EXISTS tag (
    id     TEXT PRIMARY KEY,
    name   TEXT NOT NULL,
    colour TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS message_tag (
    message_id TEXT NOT NULL,
    tag_id     TEXT NOT NULL,
    PRIMARY KEY (message_id, tag_id)
);
CREATE INDEX IF NOT EXISTS idx_message_tag_message ON message_tag(message_id);
CREATE INDEX IF NOT EXISTS idx_message_tag_tag ON message_tag(tag_id);
`

// schemaV3 adds the local cache of full message bodies (plain plus original HTML).
const schemaV3 = `
CREATE TABLE IF NOT EXISTS message_body (
    message_id TEXT PRIMARY KEY,
    plain      TEXT NOT NULL,
    html       TEXT NOT NULL
);
`

// schemaV4 replaces the original contentless message_fts (which could not map matches back to
// messages) with a queryable FTS5 table keyed by an unindexed message_id, and backfills it from the
// messages already cached.
const schemaV4 = `
DROP TABLE IF EXISTS message_fts;
CREATE VIRTUAL TABLE message_fts USING fts5(message_id UNINDEXED, subject, snippet, from_address);
INSERT INTO message_fts(message_id, subject, snippet, from_address)
    SELECT id, subject, snippet, from_address FROM message;
`

// migrations is the ordered list of schema steps. Index i upgrades the database from version i to
// version i+1, so a fresh database applies them all and an existing one applies only what it lacks.
var migrations = []string{schemaV1, schemaV2, schemaV3, schemaV4}

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
