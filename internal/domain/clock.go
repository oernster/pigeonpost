package domain

import "time"

// Clock is the only way the application obtains the current time. The domain never reads the
// wall clock directly, so behaviour that depends on "now" stays deterministic under test.
type Clock interface {
	Now() time.Time
}
