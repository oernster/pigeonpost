package storage

import "fmt"

// eventSequenceSchemaVersion is the schema version schemaV60 brings the database to. The step records it
// itself (see selfVersionedStep), so it is named once here and checked against the step's slot in the
// migrations list by a test.
const eventSequenceSchemaVersion = 60

// selfVersionedStep wraps a migration step that cannot be written idempotently (ALTER TABLE ADD COLUMN
// fails as a duplicate when re-run) in its own transaction that also records the version the step brings
// the database to. Steps run outside a transaction and the runner bumps user_version only after all of
// them, so without this a crash after the step but before that bump would leave a change a re-run trips
// over. With it the change and the version commit together or not at all; a re-run starts after the step.
func selfVersionedStep(version int, statements string) string {
	return fmt.Sprintf("\nBEGIN;\n%sPRAGMA user_version = %d;\nCOMMIT;\n", statements, version)
}

// schemaV60 stores each event's organiser revision (RFC 5545 SEQUENCE), so an older invitation can be
// told from the newer meeting already saved and refused rather than written over it. Existing rows
// default to 0, the revision of an event never revised, which is what every one of them was read as.
var schemaV60 = selfVersionedStep(eventSequenceSchemaVersion, `
ALTER TABLE event ADD COLUMN sequence INTEGER NOT NULL DEFAULT 0;
`)
