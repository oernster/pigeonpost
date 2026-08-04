package storage

import "testing"

// TestSchemaVersionMatchesMigrations pins schemaVersion to the migrations list, because migrate only
// applies steps below schemaVersion: a step appended without bumping the constant is silently
// unreachable (exactly what happened when schemaV47 was added against a version of 46).
func TestSchemaVersionMatchesMigrations(t *testing.T) {
	if schemaVersion != len(migrations) {
		t.Fatalf("schemaVersion = %d but there are %d migration steps; a trailing step will never apply",
			schemaVersion, len(migrations))
	}
}
