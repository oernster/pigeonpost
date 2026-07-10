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

// migrations is the ordered list of schema steps. Index i upgrades the database from version i to
// version i+1, so a fresh database applies them all and an existing one applies only what it lacks.
var migrations = []string{schemaV1, schemaV2, schemaV3, schemaV4, schemaV5, schemaV6, schemaV7, schemaV8, schemaV9, schemaV10, schemaV11, schemaV12, schemaV13, schemaV14, schemaV15, schemaV16, schemaV17, schemaV18, schemaV19, schemaV20, schemaV21, schemaV22, schemaV23, schemaV24, schemaV25, schemaV26, schemaV27, schemaV28, schemaV29, schemaV30}
