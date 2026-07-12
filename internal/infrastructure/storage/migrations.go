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

// migrations is the ordered list of schema steps. Index i upgrades the database from version i to
// version i+1, so a fresh database applies them all and an existing one applies only what it lacks.
var migrations = []string{schemaV1, schemaV2, schemaV3, schemaV4, schemaV5, schemaV6, schemaV7, schemaV8, schemaV9, schemaV10, schemaV11, schemaV12, schemaV13, schemaV14, schemaV15, schemaV16, schemaV17, schemaV18, schemaV19, schemaV20, schemaV21, schemaV22, schemaV23, schemaV24, schemaV25, schemaV26, schemaV27, schemaV28, schemaV29, schemaV30, schemaV31, schemaV32, schemaV33, schemaV34}
