// Package storage schema, steps 15 to 29: the calendar, the address book's later columns, the
// draft-recovery slot, the message attachments and the account ordering. They sit apart from
// schema.go and migrations.go only so each file stays within the module-size limit; the ordered list
// they belong to is in migrations.go.
package storage

// schemaV15 adds the calendar: calendars and their events. Times are stored as Unix milliseconds;
// end_ms is 0 when an event has no end; all_day marks whole-day events.
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
// millisecond values; recurrence_id holds the original start (Unix milliseconds, 0 when not an
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

// schemaV23 stores an event's meeting organiser and attendee list (as JSON) so a meeting created or
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
// the id is fixed so a save replaces the previous snapshot; the recipient columns hold the raw text
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
