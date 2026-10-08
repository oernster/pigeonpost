package runlog

import (
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

var started = time.Date(2026, 10, 8, 9, 30, 0, 0, time.UTC)

// Open makes the folder, keeps what an earlier run wrote and adds this run's start line.
func TestOpenAppendsTheStartLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "made", FileName)
	for range 2 {
		file, err := Open(path, "PigeonPost", started)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		_ = file.Close()
	}
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := strings.Count(string(text), "PigeonPost started 2026-10-08 09:30:00\n"); got != 2 {
		t.Errorf("start lines = %d, want 2 in %q", got, text)
	}
}

// A log past MaxBytes is started afresh rather than growing without end.
func TestOpenStartsAnOversizedLogAfresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, make([]byte, MaxBytes+1), filePerm); err != nil {
		t.Fatalf("seed: %v", err)
	}
	file, err := Open(path, "PigeonPost", started)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = file.Close()
	if info, _ := os.Stat(path); info.Size() >= MaxBytes {
		t.Errorf("size = %d, want the log started afresh", info.Size())
	}
}

// Open says what failed when the folder cannot be made.
func TestOpenReportsAFolderItCannotMake(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, filePerm); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := Open(filepath.Join(blocker, FileName), "PigeonPost", started); err == nil {
		t.Error("Open under a file answered no error")
	}
}

// After Keep, a log line lands in the file. The test run has an error output, so the crash report is
// copied rather than the output moved; the test's own output is left alone.
func TestKeepSendsLogLinesToTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	// Registered after TempDir so it runs first: the crash output holds its own handle on the file,
	// which Windows will not let the temporary folder be removed past.
	previous := log.Writer()
	t.Cleanup(func() {
		log.SetOutput(previous)
		_ = debug.SetCrashOutput(nil, debug.CrashOptions{})
	})
	file, err := Open(path, "PigeonPost", started)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = file.Close() }()
	if err := Keep(file); err != nil {
		t.Fatalf("Keep: %v", err)
	}
	log.Print("sync: a progress line")
	text, _ := os.ReadFile(path)
	if !strings.Contains(string(text), "sync: a progress line") {
		t.Errorf("log = %q, want the progress line", text)
	}
}
