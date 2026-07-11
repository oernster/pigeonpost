// Package storage schema: the ordered SQLite migration steps applied by Open. Kept separate from
// the store logic in sqlite.go so each file stays within the module-size limit.
package storage

// schemaVersion is the current on-disk schema version, tracked via SQLite's PRAGMA user_version.
const schemaVersion = 32

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

// schemaV12 adds the comparison operator to filter rules (contains, does-not-contain, equals,
// starts-with, ends-with). Existing rows default to 0, which is "contains", preserving their meaning.
const schemaV12 = `
ALTER TABLE rule ADD COLUMN operator INTEGER NOT NULL DEFAULT 0;
`

// schemaV13 clears the cached message bodies so each is re-fetched and re-parsed once. Bodies cached
// before the parser learned to drop sender-hidden preheader text still hold the old HTML, in which that
// text duplicates the visible content once the sanitiser strips the style that hid it. A body is a
// cache of server data, so dropping it loses nothing that cannot be fetched again.
const schemaV13 = `
DELETE FROM message_body;
`

// schemaV14 adds the address book: contacts with their labelled emails and phones, and groups (mailing
// lists) linking to contacts by id. Emails, phones and members keep an explicit position so their order
// is preserved on round-trip (the first email is the contact's primary).
const schemaV14 = `
CREATE TABLE IF NOT EXISTS contact (
    id             TEXT PRIMARY KEY,
    uid            TEXT NOT NULL,
    formatted_name TEXT NOT NULL,
    given_name     TEXT NOT NULL,
    family_name    TEXT NOT NULL,
    organization   TEXT NOT NULL,
    title          TEXT NOT NULL,
    note           TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS contact_email (
    contact_id TEXT NOT NULL,
    position   INTEGER NOT NULL,
    label      TEXT NOT NULL,
    address    TEXT NOT NULL,
    PRIMARY KEY (contact_id, position)
);
CREATE TABLE IF NOT EXISTS contact_phone (
    contact_id TEXT NOT NULL,
    position   INTEGER NOT NULL,
    label      TEXT NOT NULL,
    number     TEXT NOT NULL,
    PRIMARY KEY (contact_id, position)
);
CREATE TABLE IF NOT EXISTS contact_group (
    id   TEXT PRIMARY KEY,
    name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS contact_group_member (
    group_id   TEXT NOT NULL,
    contact_id TEXT NOT NULL,
    position   INTEGER NOT NULL,
    PRIMARY KEY (group_id, contact_id)
);
CREATE INDEX IF NOT EXISTS idx_contact_email_contact ON contact_email(contact_id);
CREATE INDEX IF NOT EXISTS idx_contact_phone_contact ON contact_phone(contact_id);
CREATE INDEX IF NOT EXISTS idx_contact_group_member_group ON contact_group_member(group_id);
`

// schemaV15 adds the calendar: calendars and their events. Times are stored as Unix milliseconds;
// end_ms is 0 when an event has no end, and all_day marks whole-day events.
const schemaV15 = `
CREATE TABLE IF NOT EXISTS calendar (
    id     TEXT PRIMARY KEY,
    name   TEXT NOT NULL,
    colour TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS event (
    id          TEXT PRIMARY KEY,
    uid         TEXT NOT NULL,
    calendar_id TEXT NOT NULL,
    summary     TEXT NOT NULL,
    description TEXT NOT NULL,
    location    TEXT NOT NULL,
    start_ms    INTEGER NOT NULL,
    end_ms      INTEGER NOT NULL,
    all_day     INTEGER NOT NULL,
    recurrence  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_event_calendar ON event(calendar_id);
CREATE INDEX IF NOT EXISTS idx_event_start ON event(start_ms);
`

// schemaV16 records a permanent send failure on an outbox row. A replay that fails for a
// non-transient reason (the account is gone, the message was rejected) keeps the item and stamps the
// reason here, rather than dropping it silently, so the user can see it in the outbox and act. Existing
// rows default to ”, meaning not failed.
const schemaV16 = `
ALTER TABLE outbox ADD COLUMN failure TEXT NOT NULL DEFAULT '';
`

// schemaV17 stores the original ICS VEVENT text on an event so import and export do not strip the
// properties PigeonPost does not model yet (categories, status, alarms and the rest). Existing rows
// default to ”, meaning the event carries no preserved ICS.
const schemaV17 = `
ALTER TABLE event ADD COLUMN extra TEXT NOT NULL DEFAULT '';
`

// schemaV18 models the rest of an event's recurrence set so it can be expanded into concrete
// occurrences: rdate and exdate hold the added and excluded occurrence starts as comma-separated Unix
// millisecond values, and recurrence_id holds the original start (Unix milliseconds, 0 when not an
// override) of the single occurrence an override event replaces. Existing rows default to no extra
// dates and not an override.
const schemaV18 = `
ALTER TABLE event ADD COLUMN rdate TEXT NOT NULL DEFAULT '';
ALTER TABLE event ADD COLUMN exdate TEXT NOT NULL DEFAULT '';
ALTER TABLE event ADD COLUMN recurrence_id INTEGER NOT NULL DEFAULT 0;
`

// schemaV19 records the IANA time zone an event's wall-clock times are kept in, so a recurring event
// holds its local time across daylight-saving changes. Existing rows default to ”, a floating or UTC
// event.
const schemaV19 = `
ALTER TABLE event ADD COLUMN time_zone TEXT NOT NULL DEFAULT '';
`

// schemaV20 records an event's reminders as a comma-separated list of trigger offsets in seconds from the
// start (negative is before). Existing rows default to ”, meaning no reminders.
const schemaV20 = `
ALTER TABLE event ADD COLUMN alarms TEXT NOT NULL DEFAULT '';
`

// schemaV21 adds the passthrough table: VTODO and VJOURNAL components preserved verbatim so an imported
// calendar's to-dos and journal entries survive an export, keyed by UID so a re-import replaces them.
const schemaV21 = `
CREATE TABLE calendar_passthrough (
	uid  TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	raw  TEXT NOT NULL
);
`

// schemaV22 caches the raw text/calendar payload (an iMIP scheduling object such as a meeting invite)
// a message carried, so the reader offers its scheduling actions and renders the invite offline.
// Existing rows default to ”, meaning the message carried no calendar part.
const schemaV22 = `
ALTER TABLE message_body ADD COLUMN invite TEXT NOT NULL DEFAULT '';
`

// schemaV23 stores an event's meeting organizer and attendee list (as JSON) so a meeting created or
// received in the app keeps its ORGANIZER and ATTENDEE data, which the scheduling flow needs to send
// invites and to fold incoming replies back into the stored meeting. Existing rows default to ”, meaning
// the event is not a scheduled meeting.
const schemaV23 = `
ALTER TABLE event ADD COLUMN organizer TEXT NOT NULL DEFAULT '';
ALTER TABLE event ADD COLUMN attendees TEXT NOT NULL DEFAULT '';
`

// schemaV24 stores a per-account compose signature as HTML. Existing rows default to ”, meaning the
// account has no signature and nothing is inserted into a new message.
const schemaV24 = `
ALTER TABLE account ADD COLUMN signature TEXT NOT NULL DEFAULT '';
`

// schemaV25 adds the local draft-recovery slot: a single-row snapshot of the compose window still being
// written, kept so an accidental close or a crash does not lose it. It is local only and never synced;
// the id is fixed so a save replaces the previous snapshot, and the recipient columns hold the raw text
// as typed rather than validated addresses.
const schemaV25 = `
CREATE TABLE draft_recovery (
    id         INTEGER PRIMARY KEY,
    account_id TEXT NOT NULL,
    to_addrs   TEXT NOT NULL,
    cc_addrs   TEXT NOT NULL,
    bcc_addrs  TEXT NOT NULL,
    subject    TEXT NOT NULL,
    body_html  TEXT NOT NULL,
    saved_ms   INTEGER NOT NULL
);
`

// schemaV26 caches a message's attachment parts alongside its body, so received files can be listed and
// saved offline after the first open. Content is the raw bytes; position keeps the sender's ordering. The
// rows are keyed by message id so re-fetching a body replaces its attachment set.
const schemaV26 = `
CREATE TABLE message_attachment (
    message_id   TEXT NOT NULL,
    position     INTEGER NOT NULL,
    filename     TEXT NOT NULL,
    content_type TEXT NOT NULL,
    content      BLOB NOT NULL,
    PRIMARY KEY (message_id, position)
);
CREATE INDEX IF NOT EXISTS idx_message_attachment_message ON message_attachment(message_id);
`

// schemaV27 clears the cached message bodies so each is re-fetched once with the attachment-aware
// parser. A body cached before the parser learned to extract attachment parts holds no attachments, so
// dropping it lets the next open populate them (and the attachment cache with them). A body is a cache of
// server data, so this loses nothing that cannot be fetched again.
const schemaV27 = `
DELETE FROM message_body;
DELETE FROM message_attachment;
`

// schemaV28 stores an account's alternate sender identities (aliases it may send as, beyond its primary
// address) as a JSON array of {display, address} objects. Existing rows default to '[]', meaning the
// account can send only as its primary address.
const schemaV28 = `
ALTER TABLE account ADD COLUMN identities TEXT NOT NULL DEFAULT '[]';
`

// schemaV29 records the account's position in the sidebar so the user can order accounts by hand.
// Existing rows default to 0, so accounts keep their previous display-name order (the list is sorted by
// position then display_name) until the first manual reorder assigns each a distinct position.
const schemaV29 = `
ALTER TABLE account ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
`
