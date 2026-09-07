package storage

// The ordered migration list and the newest migration steps live here, apart from the historical step
// definitions in schema.go, schema_steps15.go and migrations_steps30.go, so each file stays within the
// module-size limit. New migration steps are declared in this file.

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
// plus the plain body and the attachment filenames where the lazy caches hold them. The
// message_searchable_text view is the single definition of a message's searchable text: every index
// insert and the rebuild select from it, so the indexed shape cannot drift between sites. The index is
// a self-contained FTS5 table rather than external-content: the text spans three tables (message,
// message_body, message_attachment); external content requires every delete to reproduce the exact
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

// schemaV43 adds the send hold to queued outbox items: hold_until_ms is the instant (Unix
// milliseconds) an item may leave; 0 marks an ordinary offline-queued item with no hold. While the
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

// schemaV45 clears the parsed-body cache. Bodies cached by earlier versions were parsed before the
// reader linkified bare URLs, so they would render with dead link text forever; dropping the rows
// makes each message refetch and re-parse on its next open, which the cache exists to allow.
const schemaV45 = `
DELETE FROM message_body;
`

// schemaV46 adds the pending flag-operations table: a local record of a flag change (read, starred,
// answered, forwarded) not yet confirmed on the server, mirroring message_tag_pending. Some servers
// (Outlook.com among them) accept a flag STORE then report the old value on the next fetch; some drop
// the STORE outright. Without this record the sync would faithfully write that stale view over the local
// change, un-reading a message the user just viewed. Each row is the intended state of one (message,
// flag) pair; a row is cleared once a sync sees the server agree with it and is otherwise replayed to
// the server on each sync.
const schemaV46 = `
CREATE TABLE IF NOT EXISTS message_flag_pending (
    message_id TEXT NOT NULL,
    flag       INTEGER NOT NULL,
    value      INTEGER NOT NULL,
    PRIMARY KEY (message_id, flag)
);
CREATE INDEX IF NOT EXISTS idx_message_flag_pending_message ON message_flag_pending(message_id);
`

// schemaV47 clears the parsed-body cache again. Bodies cached by earlier versions had every remote CSS
// url() discarded rather than parked, so their background images were gone for good and no amount of
// asking for images could bring one back; dropping the rows makes each message refetch and re-parse on
// its next open, which is what the cache exists to allow.
const schemaV47 = `
DELETE FROM message_body;
`

// schemaV48 adds the iMIP calendar part to queued outbox items, so a meeting REPLY, REQUEST or CANCEL
// queued while the server was unreachable is replayed with its text/calendar payload intact rather
// than degrading to a plain email. The method column holds the iTIP METHOD (REPLY, REQUEST, CANCEL);
// the ics column holds the raw text/calendar payload; both empty for an ordinary message.
const schemaV48 = `
ALTER TABLE outbox ADD COLUMN calendar_method TEXT NOT NULL DEFAULT '';
ALTER TABLE outbox ADD COLUMN calendar_ics TEXT NOT NULL DEFAULT '';
`

// schemaV49 adds the per-account folder display state: the user's local sibling order for custom
// folders and the set of collapsed folder paths, each stored as a JSON array of folder paths. This
// state previously lived only in the WebView's localStorage, a browser-managed profile outside the
// application's own data directory that does not reliably survive an update or reinstall; the
// database is the durable home.
const schemaV49 = `
CREATE TABLE IF NOT EXISTS folder_ui_state (
    account_id     TEXT PRIMARY KEY,
    order_json     TEXT NOT NULL,
    collapsed_json TEXT NOT NULL
);
`

// schemaV50 rebuilds filter rules as a real rule set: a rule now carries several conditions combined by
// a match mode (all or any) and several actions, rather than exactly one of each. The conditions and
// actions move into their own child tables, keyed by rule id and ordered by position; the rule row
// gains enabled, position (its evaluation priority), match_mode and stop_processing. Existing rules are
// carried over verbatim: each becomes a one-condition, one-action rule with the same behaviour, taken
// from the legacy columns before those are dropped. The stored field, operator and action integers keep
// their historical values, so no value in a carried-over rule is reinterpreted.
//
// The step is short DDL plus a backfill over a handful of rows, in the manner of the earlier ADD COLUMN
// steps: the ALTERs are not re-runnable, so a crash inside it fails the next start loudly rather than
// migrating a half-built shape.
const schemaV50 = `
ALTER TABLE rule ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1;
ALTER TABLE rule ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
ALTER TABLE rule ADD COLUMN match_mode INTEGER NOT NULL DEFAULT 0;
ALTER TABLE rule ADD COLUMN stop_processing INTEGER NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS rule_condition (
    rule_id    TEXT NOT NULL,
    position   INTEGER NOT NULL,
    field      INTEGER NOT NULL,
    operator   INTEGER NOT NULL,
    match_text TEXT NOT NULL,
    PRIMARY KEY (rule_id, position)
);
CREATE TABLE IF NOT EXISTS rule_action (
    rule_id   TEXT NOT NULL,
    position  INTEGER NOT NULL,
    kind      INTEGER NOT NULL,
    folder_id TEXT NOT NULL,
    PRIMARY KEY (rule_id, position)
);
INSERT OR IGNORE INTO rule_condition (rule_id, position, field, operator, match_text)
    SELECT id, 0, field, operator, contains FROM rule;
INSERT OR IGNORE INTO rule_action (rule_id, position, kind, folder_id)
    SELECT id, 0, action, '' FROM rule;
ALTER TABLE rule DROP COLUMN field;
ALTER TABLE rule DROP COLUMN operator;
ALTER TABLE rule DROP COLUMN contains;
ALTER TABLE rule DROP COLUMN action;
`

// schemaV51 adds the per-condition case-sensitivity flag. Every condition written before it compared
// case-insensitively, so existing rows default to 0 and keep behaving exactly as they did.
const schemaV51 = `
ALTER TABLE rule_condition ADD COLUMN case_sensitive INTEGER NOT NULL DEFAULT 0;
`

// schemaV52 adds the per-rule account scope: the accounts a rule is limited to, as rows keyed by rule
// id. A rule with no row applies to every account, which is what every rule written before this step
// did, so nothing needs backfilling and a rule stays covering an account added later.
const schemaV52 = `
CREATE TABLE IF NOT EXISTS rule_account (
    rule_id    TEXT NOT NULL,
    account_id TEXT NOT NULL,
    PRIMARY KEY (rule_id, account_id)
);
`

// schemaV53 records which folders have had their baseline sync, the pass that establishes what a
// folder already holds so the rules do not treat an existing backlog as arrivals. It replaces the
// inference this decision used to rest on, "the local store holds no messages for this folder", which
// is equally true of an inbox the user has simply emptied: mail arriving into a filed-clean inbox was
// read as a first sight and exempted from every destructive rule action.
//
// It cannot live on the folder row, because SaveFolders clears and rewrites every folder for an
// account on each sync and would take the mark with it, so it is keyed by folder id here.
//
// Every folder that exists at this step is marked: those installs have already established their
// baseline through ordinary use. A folder written after this step, which is what a newly added account
// produces, has no row and so still gets its one protected pass.
const schemaV53 = `
CREATE TABLE IF NOT EXISTS folder_baseline (
    folder_id TEXT PRIMARY KEY
);
INSERT OR IGNORE INTO folder_baseline (folder_id) SELECT id FROM folder;
`

// migrations is the ordered list of schema steps. Index i upgrades the database from version i to
// version i+1, so a fresh database applies them all and an existing one applies only what it lacks.
// schemaV54 clears the cached messages whose folder no longer exists. Renaming or moving a folder
// replaces the account's folder set under new ids (a folder id is its account plus its path) while the
// message rows keep the id of the folder that has gone, so nothing lists them, nothing counts them and
// nothing deleted them; one account had 2,439 such rows across 18 dead folder ids. SaveFolders now
// sweeps them as part of replacing a folder set, which keeps it true from here on; this clears what
// earlier versions already left behind, without waiting for the next full account sync. A message row
// is a cache of server data, so dropping it costs a re-fetch and nothing else.
const schemaV54 = `
DELETE FROM message_body WHERE message_id IN (SELECT id FROM message m WHERE NOT EXISTS (SELECT 1 FROM folder f WHERE f.id = m.folder_id));
DELETE FROM message_attachment WHERE message_id IN (SELECT id FROM message m WHERE NOT EXISTS (SELECT 1 FROM folder f WHERE f.id = m.folder_id));
DELETE FROM message_tag WHERE message_id IN (SELECT id FROM message m WHERE NOT EXISTS (SELECT 1 FROM folder f WHERE f.id = m.folder_id));
DELETE FROM message_tag_pending WHERE message_id IN (SELECT id FROM message m WHERE NOT EXISTS (SELECT 1 FROM folder f WHERE f.id = m.folder_id));
DELETE FROM message_flag_pending WHERE message_id IN (SELECT id FROM message m WHERE NOT EXISTS (SELECT 1 FROM folder f WHERE f.id = m.folder_id));
DELETE FROM message_search WHERE message_id IN (SELECT id FROM message m WHERE NOT EXISTS (SELECT 1 FROM folder f WHERE f.id = m.folder_id));
DELETE FROM message_snooze WHERE message_id IN (SELECT id FROM message m WHERE NOT EXISTS (SELECT 1 FROM folder f WHERE f.id = m.folder_id));
DELETE FROM message WHERE NOT EXISTS (SELECT 1 FROM folder f WHERE f.id = message.folder_id);
`

// schemaV55 puts every one-condition rule on match mode "all". With a single condition the two modes
// are identical, so no rule changes behaviour here; what changes is what the rule does NEXT. A rule
// left on "any" reads harmlessly until a second condition is added; where that second condition is
// an exclusion the rule widens to nearly every message instead of narrowing. The editor now starts
// a new rule on "all"; this takes the same trap off the rules already written. A rule with two or more
// conditions is left exactly as it is: its mode is a choice someone made and it is load-bearing.
const schemaV55 = `
UPDATE rule SET match_mode = 0
WHERE (SELECT COUNT(*) FROM rule_condition c WHERE c.rule_id = rule.id) <= 1;
`

var migrations = []string{schemaV1, schemaV2, schemaV3, schemaV4, schemaV5, schemaV6, schemaV7, schemaV8, schemaV9, schemaV10, schemaV11, schemaV12, schemaV13, schemaV14, schemaV15, schemaV16, schemaV17, schemaV18, schemaV19, schemaV20, schemaV21, schemaV22, schemaV23, schemaV24, schemaV25, schemaV26, schemaV27, schemaV28, schemaV29, schemaV30, schemaV31, schemaV32, schemaV33, schemaV34, schemaV35, schemaV36, schemaV37, schemaV38, schemaV39, schemaV40, schemaV41, schemaV42, schemaV43, schemaV44, schemaV45, schemaV46, schemaV47, schemaV48, schemaV49, schemaV50, schemaV51, schemaV52, schemaV53, schemaV54, schemaV55}
