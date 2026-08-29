package errlog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fixedClock stamps every entry with the same instant, so an assertion can name the timestamp it
// expects rather than matching a pattern.
type fixedClock struct{ at time.Time }

// Now reports the fixed instant.
func (c fixedClock) Now() time.Time { return c.at }

// stamp is the instant fixedClock reports, chosen so its RFC3339 form is unmistakable in a file.
var stamp = time.Date(2026, time.August, 29, 14, 30, 0, 0, time.UTC)

// newRecorder builds a recorder over a fresh temporary directory, returning it with its path.
func newRecorder(t *testing.T, maxBytes int64) (*Recorder, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mail-errors.log")
	return New(path, fixedClock{at: stamp}, maxBytes), path
}

// read returns the log's contents, failing the test if it cannot be read.
func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return string(data)
}

func TestRecordWritesTheRawTextWithATimestamp(t *testing.T) {
	t.Parallel()
	recorder, path := newRecorder(t, DefaultMaxBytes)
	recorder.Record(errors.New(`imap: xoauth2 "a@b.com": imap: NO User is authenticated but not connected.`))
	got := read(t, path)
	if !strings.Contains(got, "2026-08-29T14:30:00Z") {
		t.Fatalf("entry %q carries no timestamp", got)
	}
	// The whole point is that the server's own words survive translation, so they are the contract.
	if !strings.Contains(got, "authenticated but not connected") {
		t.Fatalf("entry %q does not carry the server's own words", got)
	}
}

func TestRecordKeepsEachFailureOnOneLine(t *testing.T) {
	t.Parallel()
	recorder, path := newRecorder(t, DefaultMaxBytes)
	recorder.Record(errors.New("first line\nsecond line"))
	recorder.Record(errors.New("another failure"))
	lines := strings.Split(strings.TrimSuffix(read(t, path), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want one per failure however many lines the server sent: %q", len(lines), lines)
	}
}

func TestRecordIgnoresNil(t *testing.T) {
	t.Parallel()
	recorder, path := newRecorder(t, DefaultMaxBytes)
	recorder.Record(nil)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a nil error created %q; an installation that never fails should never grow a log", path)
	}
}

func TestRecordRollsOverAndKeepsThePreviousGeneration(t *testing.T) {
	t.Parallel()
	// A limit of a few bytes is reached by the first entry, so the second triggers the rollover.
	const tinyLimit = 8
	recorder, path := newRecorder(t, tinyLimit)
	recorder.Record(errors.New("the older failure"))
	recorder.Record(errors.New("the newer failure"))

	current := read(t, path)
	if !strings.Contains(current, "the newer failure") {
		t.Fatalf("current log %q lost the entry that triggered the rollover", current)
	}
	previous := read(t, path+previousSuffix)
	if !strings.Contains(previous, "the older failure") {
		t.Fatalf("previous generation %q lost what came before; rolling over must not truncate", previous)
	}
}

func TestRecordSurvivesAnUnwritableLocation(t *testing.T) {
	t.Parallel()
	// A path whose parent is a file, not a directory, cannot be opened. Recording is a side errand of
	// failing at something else, so it must not panic or otherwise disturb the caller.
	blocked := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	recorder := New(filepath.Join(blocked, "mail-errors.log"), fixedClock{at: stamp}, DefaultMaxBytes)
	recorder.Record(errors.New("boom"))
}

func TestRecordIsSafeFromSeveralGoroutines(t *testing.T) {
	t.Parallel()
	const writers = 20
	recorder, path := newRecorder(t, DefaultMaxBytes)
	var wg sync.WaitGroup
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			recorder.Record(errors.New("concurrent failure"))
		}()
	}
	wg.Wait()
	lines := strings.Split(strings.TrimSuffix(read(t, path), "\n"), "\n")
	if len(lines) != writers {
		t.Fatalf("got %d lines from %d concurrent writers, want one each", len(lines), writers)
	}
}

func TestPathReportsWhereEntriesGo(t *testing.T) {
	t.Parallel()
	recorder, path := newRecorder(t, DefaultMaxBytes)
	if recorder.Path() != path {
		t.Fatalf("Path() = %q, want %q", recorder.Path(), path)
	}
}
