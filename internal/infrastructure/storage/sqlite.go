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
const schemaVersion = 11

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

// schemaV5 adds the offline outbox: outgoing sends and drafts queued while the server was unreachable,
// held until they can be replayed. Recipient lists are stored as JSON so the row is self-contained.
const schemaV5 = `
CREATE TABLE IF NOT EXISTS outbox (
    id           TEXT PRIMARY KEY,
    account_id   TEXT NOT NULL,
    kind         INTEGER NOT NULL,
    from_display TEXT NOT NULL,
    from_address TEXT NOT NULL,
    to_json      TEXT NOT NULL,
    cc_json      TEXT NOT NULL,
    subject      TEXT NOT NULL,
    body         TEXT NOT NULL,
    html_body    TEXT NOT NULL,
    created_ms   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_outbox_created ON outbox(created_ms);
`

// schemaV6 adds the To and Cc recipient lists to cached message summaries (stored as JSON arrays), so
// reply-all can address the whole conversation from the local cache. Existing rows default to empty
// lists until the next sync refills them.
const schemaV6 = `
ALTER TABLE message ADD COLUMN to_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE message ADD COLUMN cc_json TEXT NOT NULL DEFAULT '[]';
`

// schemaV7 adds the filter-rule table: user-defined rules that mark-read or flag arriving messages
// whose sender or subject contains a given text.
const schemaV7 = `
CREATE TABLE IF NOT EXISTS rule (
    id       TEXT PRIMARY KEY,
    name     TEXT NOT NULL,
    field    INTEGER NOT NULL,
    contains TEXT NOT NULL,
    action   INTEGER NOT NULL
);
`

// schemaV8 adds the blind-carbon-copy recipient list to queued outbox items, so a message sent while
// offline preserves its Bcc recipients when it is replayed. Existing rows default to an empty list.
const schemaV8 = `
ALTER TABLE outbox ADD COLUMN bcc_json TEXT NOT NULL DEFAULT '[]';
`

// schemaV9 adds the attachment list to queued outbox items (filename, content type and base64 bytes as
// JSON), so a message sent while offline keeps its attachments when it is replayed. Existing rows
// default to an empty list.
const schemaV9 = `
ALTER TABLE outbox ADD COLUMN attachments_json TEXT NOT NULL DEFAULT '[]';
`

// schemaV10 records each folder's server hierarchy delimiter, so the leaf name and a rename path are
// derived correctly on servers that do not use "/" (StartMail uses "."). Existing rows default to "/"
// until the next sync refills the real delimiter.
const schemaV10 = `
ALTER TABLE folder ADD COLUMN separator TEXT NOT NULL DEFAULT '/';
`

// schemaV11 widens message.uid from INTEGER to TEXT so it can hold an opaque server handle: an IMAP
// UID as a decimal string, or a POP3 UIDL. SQLite cannot change a column type in place, so the message
// table is rebuilt and existing integer uids are cast to their text form. The body, tag and FTS tables
// key on the string message id, not uid, so they are left untouched.
const schemaV11 = `
CREATE TABLE message_new (
    id              TEXT PRIMARY KEY,
    folder_id       TEXT NOT NULL,
    uid             TEXT NOT NULL,
    message_id      TEXT NOT NULL,
    from_display    TEXT NOT NULL,
    from_address    TEXT NOT NULL,
    subject         TEXT NOT NULL,
    date_ms         INTEGER NOT NULL,
    size            INTEGER NOT NULL,
    flags           INTEGER NOT NULL,
    has_attachments INTEGER NOT NULL,
    snippet         TEXT NOT NULL,
    to_json         TEXT NOT NULL DEFAULT '[]',
    cc_json         TEXT NOT NULL DEFAULT '[]'
);
INSERT INTO message_new (id, folder_id, uid, message_id, from_display, from_address, subject, date_ms,
        size, flags, has_attachments, snippet, to_json, cc_json)
    SELECT id, folder_id, CAST(uid AS TEXT), message_id, from_display, from_address, subject, date_ms,
        size, flags, has_attachments, snippet, to_json, cc_json FROM message;
DROP TABLE message;
ALTER TABLE message_new RENAME TO message;
CREATE INDEX IF NOT EXISTS idx_message_folder ON message(folder_id);
`

// migrations is the ordered list of schema steps. Index i upgrades the database from version i to
// version i+1, so a fresh database applies them all and an existing one applies only what it lacks.
var migrations = []string{schemaV1, schemaV2, schemaV3, schemaV4, schemaV5, schemaV6, schemaV7, schemaV8, schemaV9, schemaV10, schemaV11}

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
