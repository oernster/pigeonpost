package imap

import (
	"log"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// traceStep logs the start of one step against the server (a folder list, a folder's fetch, a replay's
// push) and answers the function that logs its end with how many messages it carried and how it ended.
// The run log then shows where a sync is and what it is waiting on: a full sync of a Hotmail account
// stalled for six weeks with nothing to say on which folder or step (2026-10-08).
func traceStep(account domain.Account, step, folder string) func(count int, err error) {
	started := time.Now()
	log.Printf("imap %s: %s %q started", account.ID(), step, folder)
	return func(count int, err error) {
		log.Printf("imap %s: %s %q ended after %s: %d messages, error %v",
			account.ID(), step, folder, time.Since(started).Round(time.Millisecond), count, err)
	}
}
