// Package runlog keeps the log a run leaves: a line naming when the run started, then everything the run
// writes to its error output, in run.log beside the database. It is ported from Bridge Talk's runlog.
//
// A windowed Windows program is started with no error output, so until this existed every log line
// PigeonPost wrote went nowhere; so did the Go runtime's own report of a crash. That is how a full sync of
// a Hotmail account stalled for six weeks with nothing anywhere to say where (2026-10-08).
package runlog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

const (
	// FileName names the log inside the application's data folder.
	FileName = "run.log"
	// MaxBytes is the size past which a run starts the log afresh, so it never grows without end. A
	// sync's progress lines run to a few kilobytes; a crash report to tens.
	MaxBytes = 1 << 20

	startedLayout = "2006-01-02 15:04:05"
	folderPerm    = 0o755
	filePerm      = 0o644
)

// Open opens the log at path for a run of app started at started, making its folder where there is
// none, then writes the run's start line. What the log holds is kept unless it is over MaxBytes, when
// it is started afresh.
func Open(path, app string, started time.Time) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), folderPerm); err != nil {
		return nil, fmt.Errorf("runlog: make the folder for %s: %w", path, err)
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if info, err := os.Stat(path); err == nil && info.Size() > MaxBytes {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(path, flags, filePerm)
	if err != nil {
		return nil, fmt.Errorf("runlog: open %s: %w", path, err)
	}
	if _, err := fmt.Fprintf(file, "%s started %s\n", app, started.Format(startedLayout)); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("runlog: write to %s: %w", path, err)
	}
	return file, nil
}

// Keep sends what the run reports to file for the rest of the run. The log package's lines always go
// there. Where the run was given no error output (a windowed program started from a shortcut) the error
// output itself goes there too, so the runtime's crash report lands in the file. A run that has an error output keeps
// it and has the crash report copied to file as well.
func Keep(file *os.File) error {
	log.SetOutput(file)
	if !hasErrorOutput() {
		return sendAll(file)
	}
	if err := debug.SetCrashOutput(file, debug.CrashOptions{}); err != nil {
		return fmt.Errorf("runlog: copy crash reports to %s: %w", file.Name(), err)
	}
	return nil
}
