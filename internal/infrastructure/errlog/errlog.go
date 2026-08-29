// Package errlog keeps the raw text of an error that is about to be replaced by one fit to read.
//
// A mail error crossing into the interface is translated: the server's own words ("authenticated but
// not connected", a tagged NO response, a dial failure) mean nothing to the person reading them, so a
// message naming the setting and the steps is shown instead. That translation asserts a cause; an
// asserted cause can be wrong. When it is, the reader is told to change a setting that is not their
// problem and there is nothing left anywhere to say otherwise, because the evidence was discarded at
// the moment it was replaced. This package is the copy that is kept.
package errlog

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// DefaultMaxBytes is the size at which the current file is rolled over. One previous generation is
// kept beside it, so the pair costs at most twice this on disk. It is sized to hold a long run of
// failures while staying small enough to attach to a bug report.
const DefaultMaxBytes int64 = 256 * 1024

// previousSuffix names the single kept generation. Rolling over rather than truncating means the
// entries written just before the limit survive, which are the ones a report is usually about.
const previousSuffix = ".old"

// filePerm keeps the log readable by its owner only. It carries mail server responses and the address
// they concern, so it is no more public than the database beside it.
const filePerm = 0o600

// Recorder appends error text to a file, one entry per line, rolling the file over once it grows past
// its limit. It is safe for concurrent use: mail actions fail from several goroutines at once.
type Recorder struct {
	path  string
	clock domain.Clock
	max   int64
	mu    sync.Mutex
}

// New constructs a recorder writing to path, stamping entries from clock and rolling over at maxBytes.
// The file is created on the first entry, so an installation that never fails never grows one.
func New(path string, clock domain.Clock, maxBytes int64) *Recorder {
	return &Recorder{path: path, clock: clock, max: maxBytes}
}

// Record appends err to the log. A nil error is ignored.
//
// It reports nothing and returns nothing. Recording is a side errand of failing at something else, so a
// log that cannot be written must not turn a mail error the caller can describe into a second error it
// cannot. A full disk or a read-only directory loses the entry; it does not lose the mail action.
func (r *Recorder) Record(err error) {
	if err == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rollOverIfLarge()
	file, openErr := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, filePerm)
	if openErr != nil {
		return
	}
	defer func() { _ = file.Close() }()
	// %q rather than %s: a server response can carry newlines and an entry that spans lines cannot be
	// read back as one failure per line.
	_, _ = fmt.Fprintf(file, "%s %q\n", r.clock.Now().UTC().Format(time.RFC3339), err.Error())
}

// Path reports the file entries are written to, so the interface can tell someone where to look.
func (r *Recorder) Path() string {
	return r.path
}

// rollOverIfLarge moves the current file aside once it reaches the limit, replacing any previous
// generation. A file that cannot be measured or moved is left alone and simply keeps growing, which is
// preferable to losing the entry that prompted the check.
func (r *Recorder) rollOverIfLarge() {
	info, statErr := os.Stat(r.path)
	if statErr != nil || info.Size() < r.max {
		return
	}
	_ = os.Rename(r.path, r.path+previousSuffix)
}
