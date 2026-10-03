package storage

// schemaV58 records the UIDVALIDITY each folder was last synced under. A cached message's id is its
// folder plus its UID, so when a server renumbers a mailbox every message comes back under a new id and
// the filter rules would read the whole backlog as new mail. A changed UIDVALIDITY is how IMAP says
// that happened; keeping the last value lets the sync spot it and rebaseline the folder instead.
//
// It is a table of its own rather than a column on the folder row, because SaveFolders clears and
// rewrites every folder on each sync and would take the value with it; nor does it share
// folder_baseline, which marks the Inbox only while this is kept for every folder synced. No row is
// written here: an existing folder's first sync after the update records its value as a first sight,
// which changes nothing. The step is idempotent, since steps run outside a transaction.
const schemaV58 = `
CREATE TABLE IF NOT EXISTS folder_uidvalidity (
    folder_id    TEXT PRIMARY KEY,
    uid_validity INTEGER NOT NULL
);
`
