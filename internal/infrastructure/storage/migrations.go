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

// migrations is the ordered list of schema steps. Index i upgrades the database from version i to
// version i+1, so a fresh database applies them all and an existing one applies only what it lacks.
var migrations = []string{schemaV1, schemaV2, schemaV3, schemaV4, schemaV5, schemaV6, schemaV7, schemaV8, schemaV9, schemaV10, schemaV11, schemaV12, schemaV13, schemaV14, schemaV15, schemaV16, schemaV17, schemaV18, schemaV19, schemaV20, schemaV21, schemaV22, schemaV23, schemaV24, schemaV25, schemaV26, schemaV27, schemaV28, schemaV29, schemaV30, schemaV31, schemaV32, schemaV33, schemaV34, schemaV35, schemaV36, schemaV37, schemaV38, schemaV39}
