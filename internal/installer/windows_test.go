//go:build windows

package installer

import (
	"os"
	"path/filepath"
	"testing"
)

// The running test process is, by definition, running, so detecting it by its own executable name
// exercises the snapshot walk and the match path deterministically.
func TestIsProcessRunningDetectsThisProcess(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	name := filepath.Base(exe)
	if !isProcessRunning(name) {
		t.Errorf("the running test process %q was not detected", name)
	}
}

// A name that cannot correspond to any real process must walk the whole snapshot and report false.
func TestIsProcessRunningRejectsPhantom(t *testing.T) {
	if isProcessRunning("pigeonpost-not-a-real-process-zzzz.exe") {
		t.Error("a non-existent process was reported as running")
	}
}
