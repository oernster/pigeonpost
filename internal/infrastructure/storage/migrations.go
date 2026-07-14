package storage

// The ordered migration list and the newest migration steps live here, apart from the historical step
// definitions in schema.go, so each file stays within the module-size limit. New migration steps are
// declared in this file.

// schemaV30 clears the cached message bodies again so each is re-fetched and re-parsed once, now that
// the parser resolves an inline cid: image to the message's own embedded bytes as a data: URI instead
// of parking it as if it were remote. Bodies cached before that change still hold cid: references the
// webview cannot load. A body is a cache of server data, so dropping it loses nothing that cannot be
// fetched again.
const schemaV30 = `
DELETE FROM message_body;
`

// schemaV31 adds the message-template table: reusable {name, subject, body} skeletons the user inserts
// while composing. The body is HTML.
const schemaV31 = `
CREATE TABLE IF NOT EXISTS template (
    id      TEXT PRIMARY KEY,
    name    TEXT NOT NULL,
    subject TEXT NOT NULL,
    body    TEXT NOT NULL
);
`

// schemaV32 adds a contact's birthday (vCard BDAY, kept as free text; existing rows default to ”,
// meaning none) and its labelled postal addresses. Addresses keep an explicit position so their order
// is preserved on round-trip, mirroring the contact_email and contact_phone tables.
const schemaV32 = `
ALTER TABLE contact ADD COLUMN birthday TEXT NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS contact_address (
    contact_id  TEXT NOT NULL,
    position    INTEGER NOT NULL,
    label       TEXT NOT NULL,
    street      TEXT NOT NULL,
    locality    TEXT NOT NULL,
    region      TEXT NOT NULL,
    postal_code TEXT NOT NULL,
    country     TEXT NOT NULL,
    PRIMARY KEY (contact_id, position)
);
CREATE INDEX IF NOT EXISTS idx_contact_address_contact ON contact_address(contact_id);
`

// schemaV33 adds a composite index on (folder_id, date_ms, id) so the reading list can page a folder's
// messages by keyset (date and id) at any depth without a full scan. The list orders newest first, which
// SQLite serves by walking this index in reverse; the ascending sort walks it forward. The pre-existing
// idx_message_folder stays for the many single-folder lookups that do not order by date.
const schemaV33 = `
CREATE INDEX IF NOT EXISTS idx_message_folder_date ON message(folder_id, date_ms, id);
`

// schemaV34 clears the cached message bodies so each is re-fetched and re-parsed once with the corrected
// HTML preparation: font-size:0 layout wrappers are no longer mistaken for hidden preheaders (which had
// blanked MJML-built messages such as the Claude sign-in email) and mso-hide:all content is kept. A body
// is a cache of server data, so dropping it loses nothing that cannot be fetched again on next open.
const schemaV34 = `
DELETE FROM message_body;
`

// schemaV35 clears the cached message bodies again so each is re-parsed once with the relaxed sanitiser
// that now keeps the message's own inline styles and <style> blocks (rendered inside the sandboxed iframe).
// Bodies cached before this change had their styling stripped, so they would render unstyled until re-fetched.
// A body is a cache of server data, so dropping it loses nothing that cannot be fetched again on next open.
const schemaV35 = `
DELETE FROM message_body;
`

// schemaV36 adds the pending tag-keyword operations table: a local record of a tag assignment or removal
// that has not yet been confirmed on the server, so an assignment made while offline is not undone by the
// next sync before its keyword STORE lands. Each row is the intended state of one (message, tag) pair:
// assigned is 1 for an intended assignment and 0 for an intended removal. A row is cleared once a sync sees
// the server agree with it.
const schemaV36 = `
CREATE TABLE IF NOT EXISTS message_tag_pending (
    message_id TEXT NOT NULL,
    tag_id     TEXT NOT NULL,
    assigned   INTEGER NOT NULL,
    PRIMARY KEY (message_id, tag_id)
);
CREATE INDEX IF NOT EXISTS idx_message_tag_pending_message ON message_tag_pending(message_id);
`

// schemaV37 stores each tag's stable IMAP keyword (its server-side label) so it can be frozen at creation
// and survive a rename: the keyword is derived from the name once here; thereafter the column is
// authoritative rather than the mutable name. Existing rows are backfilled with the derivation the Go
// code uses on create ("$PPtag_" followed by the name bytes as lower-case hex), so a tag keeps the keyword
// it would have had. The name is hex-encoded without lower-casing it: only the hex digits are lower-cased,
// so this matches KeywordForName byte for byte for every name including non-ASCII ones (SQLite's lower()
// is ASCII-only and would diverge from Go's Unicode-aware casing, freezing a keyword the server never
// stored and letting a later reconcile strip the tag). Names are already trimmed on save by NewTag. A
// rename never rewrites this column.
const schemaV37 = `
ALTER TABLE tag ADD COLUMN keyword TEXT NOT NULL DEFAULT '';
UPDATE tag SET keyword = '$PPtag_' || lower(hex(name)) WHERE keyword = '';
`

// schemaV38 gives events their own category column so a category set on an in-app event survives a
// reload. Before this the category rode only in the preserved ICS extra blob, which is empty for an
// event created in the app, so its category was dropped on the next load from the store. Existing rows
// default to an empty string (no category); an imported event keeps its own via the extra blob.
const schemaV38 = `
ALTER TABLE event ADD COLUMN category TEXT NOT NULL DEFAULT '';
`

// schemaV39 adds the calendar_account table: a stored CalDAV/CardDAV account (its display name, base URL,
// username and auth method). The password is never stored here; it lives in the OS keychain keyed by the
// account id, exactly as for a mail account.
const schemaV39 = `
CREATE TABLE IF NOT EXISTS calendar_account (
    id           TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    base_url     TEXT NOT NULL,
    username     TEXT NOT NULL,
    auth         INTEGER NOT NULL
);
`

// schemaV40 adds the CalDAV two-way write-back state. Each synced event gains its remote object's href and
// etag, so a write-back can target the object (its href) and guard the write (If-Match its etag); all events
// decoded from one object share that object's href and etag. The calendar (collection) gains account_id (the
// owning calendar_account, empty for a purely local calendar), href (the collection resource path) and ctag
// (the collectionserver CTag used to skip an unchanged collection on sync). calendar_pending is the pending
// write-intent table, mirroring message_tag_pending: one row per remote object that has an unpushed local
// change. op is create, update or delete (0, 1, 2); base_etag is empty for a create (driving If-None-Match:*)
// and the last-seen etag for an update or delete (driving If-Match). A delete row survives the local event's
// removal, so it also serves as the object's tombstone until the server delete is confirmed.
const schemaV40 = `
ALTER TABLE event ADD COLUMN href TEXT NOT NULL DEFAULT '';
ALTER TABLE event ADD COLUMN etag TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_event_href ON event(href);
ALTER TABLE calendar ADD COLUMN account_id TEXT NOT NULL DEFAULT '';
ALTER TABLE calendar ADD COLUMN href TEXT NOT NULL DEFAULT '';
ALTER TABLE calendar ADD COLUMN ctag TEXT NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS calendar_pending (
    calendar_id TEXT NOT NULL,
    href        TEXT NOT NULL,
    op          INTEGER NOT NULL,
    base_etag   TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (calendar_id, href)
);
CREATE INDEX IF NOT EXISTS idx_calendar_pending_calendar ON calendar_pending(calendar_id);
`

// schemaV41 replaces the subject/snippet/sender message_fts with message_search, the full-text search
// index over everything searchable about a message: subject, snippet, sender, recipients (To and Cc)
// and, where the lazy caches hold them, the plain body and the attachment filenames. The
// message_searchable_text view is the single definition of a message's searchable text: every index
// insert and the rebuild select from it, so the indexed shape cannot drift between sites. The index is
// a self-contained FTS5 table rather than external-content: the text spans three tables (message,
// message_body, message_attachment), and external content requires every delete to reproduce the exact
// values as indexed, which cross-table mutation ordering cannot guarantee; self-contained keeps every
// consistency path an idempotent DELETE or reinsert by message id, at the cost of the index holding its
// own copy of the text. The backfill indexes all already-cached mail, bodies included where cached.
// The step is idempotent (the drops make a partial earlier run harmless): migration steps run outside
// a transaction and user_version is bumped only after all steps, so a crash mid-step must leave a
// state the re-run can repair rather than trip over. The backfill here is the first step whose
// runtime grows with the whole mailbox, which widens that crash window.
const schemaV41 = `
DROP VIEW IF EXISTS message_searchable_text;
DROP TABLE IF EXISTS message_search;
CREATE VIEW message_searchable_text AS
SELECT m.rowid AS rowid, m.id AS message_id, m.subject AS subject, m.snippet AS snippet,
       TRIM(m.from_display || ' ' || m.from_address) AS sender,
       COALESCE((SELECT GROUP_CONCAT(COALESCE(json_extract(j.value, '$.display'), '') || ' ' ||
                                     COALESCE(json_extract(j.value, '$.address'), ''), ' ')
                 FROM json_each(m.to_json) j), '')
       || ' ' ||
       COALESCE((SELECT GROUP_CONCAT(COALESCE(json_extract(j.value, '$.display'), '') || ' ' ||
                                     COALESCE(json_extract(j.value, '$.address'), ''), ' ')
                 FROM json_each(m.cc_json) j), '') AS recipients,
       COALESCE(b.plain, '') AS body,
       COALESCE((SELECT GROUP_CONCAT(a.filename, ' ') FROM message_attachment a
                 WHERE a.message_id = m.id), '') AS filenames
FROM message m LEFT JOIN message_body b ON b.message_id = m.id;
CREATE VIRTUAL TABLE message_search USING fts5(message_id UNINDEXED, subject, snippet, sender, recipients, body, filenames);
INSERT INTO message_search (message_id, subject, snippet, sender, recipients, body, filenames)
    SELECT message_id, subject, snippet, sender, recipients, body, filenames FROM message_searchable_text;
DROP TABLE IF EXISTS message_fts;
`

// schemaV42 clears the cached message bodies once more so each is re-parsed with the href-normalising
// HTML preparation: an anchor whose href was wrapped across source lines (a tab, newline or encoded
// line break inside the URL, which bulk senders routinely emit) was deleted outright by the sanitiser,
// leaving the email's buttons styled but dead in the cached HTML. A body is a cache of server data, so
// dropping it loses nothing that cannot be fetched again on next open. The search index holds its own
// copy of indexed body text, so it is rebuilt from message_searchable_text (the single definition of a
// message's searchable text) after the clear: the view LEFT JOINs the now-empty body cache, so subject,
// snippet, sender, recipients and filenames stay searchable while body text drops out until a body is
// cached again, exactly as for a message never opened. Every statement is an idempotent re-run, as the
// migration crash-window rule for steps outside a transaction requires.
const schemaV42 = `
DELETE FROM message_body;
DELETE FROM message_search;
INSERT INTO message_search (message_id, subject, snippet, sender, recipients, body, filenames)
    SELECT message_id, subject, snippet, sender, recipients, body, filenames FROM message_searchable_text;
`

// schemaV43 adds the undo-send hold to queued outbox items: hold_until_ms is the instant (Unix
// milliseconds) an item may leave, or 0 for an ordinary offline-queued item with no hold. While the
// hold is in the future the item is cancellable; the dispatcher sends it once the hold elapses.
// Existing rows default to 0, keeping their replay-on-reconnect behaviour.
const schemaV43 = `
ALTER TABLE outbox ADD COLUMN hold_until_ms INTEGER NOT NULL DEFAULT 0;
`

// schemaV44 adds snooze: a message with a row here is hidden from its folder's listings until the
// until_ms instant (Unix milliseconds) passes, then reappears untouched. The row is local-only state
// (nothing goes to the server) and is deleted when it comes due or the user unsnoozes. The index lets
// the resurface scheduler find the earliest due time without a scan.
const schemaV44 = `
CREATE TABLE message_snooze (
    message_id TEXT PRIMARY KEY,
    until_ms   INTEGER NOT NULL
);
CREATE INDEX idx_message_snooze_until ON message_snooze (until_ms);
`

// migrations is the ordered list of schema steps. Index i upgrades the database from version i to
// version i+1, so a fresh database applies them all and an existing one applies only what it lacks.
var migrations = []string{schemaV1, schemaV2, schemaV3, schemaV4, schemaV5, schemaV6, schemaV7, schemaV8, schemaV9, schemaV10, schemaV11, schemaV12, schemaV13, schemaV14, schemaV15, schemaV16, schemaV17, schemaV18, schemaV19, schemaV20, schemaV21, schemaV22, schemaV23, schemaV24, schemaV25, schemaV26, schemaV27, schemaV28, schemaV29, schemaV30, schemaV31, schemaV32, schemaV33, schemaV34, schemaV35, schemaV36, schemaV37, schemaV38, schemaV39, schemaV40, schemaV41, schemaV42, schemaV43, schemaV44}
