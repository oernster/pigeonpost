package storage

import "fmt"

// outboxClaimSchemaVersion is the schema version schemaV59 brings the database to. The step records it
// itself (see selfVersionedStep), so it is named once here and checked against the step's slot in the
// migrations list by a test.
const outboxClaimSchemaVersion = 59

// schemaV59 gives every outbox row a send state so that exactly one replay can send it. A row is
// outboxStateQueued until a replay claims it with a single conditional UPDATE that moves it to
// outboxStateSending; only the replay whose UPDATE changed the row sends, so two syncs (or a sync and
// the send-later dispatcher) can no longer deliver the same message twice. A cancel deletes only a
// queued row, so a cancel that lands during the SMTP send is told the message left rather than told it
// was stopped. claim_owner names the process run that holds the claim, so the claims a crashed run left
// behind can be told apart from the ones a replay in this run holds right now.
//
// ALTER TABLE ADD COLUMN cannot be written idempotently in SQLite, so a crash between the two ALTERs (or
// between them and the runner's user_version bump) would leave a column that a re-run fails on as a
// duplicate. selfVersionedStep therefore commits the columns and the version together or not at all; a
// re-run starts after this step. Existing rows default to queued with no owner, which is what they were.
var schemaV59 = selfVersionedStep(outboxClaimSchemaVersion, fmt.Sprintf(`ALTER TABLE outbox ADD COLUMN send_state TEXT NOT NULL DEFAULT '%s';
ALTER TABLE outbox ADD COLUMN claim_owner TEXT NOT NULL DEFAULT '';
`, outboxStateQueued))
