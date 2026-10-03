package main

import (
	"bytes"
	"context"
	"log"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/infrastructure/storage"
)

// recoveryApp builds an App whose compose service runs over outbox and nothing else: recovery touches
// only the outbox store.
func recoveryApp(outbox application.OutboxStore) *App {
	return &App{
		ctx:     context.Background(),
		compose: application.NewComposeService(nil, nil, nil, nil, nil, outbox, nil, nil, nil),
	}
}

// captureLog redirects the standard logger for the test and returns what it wrote.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	return &buf
}

// A clean start has nothing to recover: no log line and no event (an event here would need the Wails
// runtime, which a test does not have, so reaching one would end the test).
func TestRecoverOutboxWithNothingInterruptedIsQuiet(t *testing.T) {
	logged := captureLog(t)
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "outbox.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	recoveryApp(store).recoverOutbox()
	if logged.Len() != 0 {
		t.Errorf("logged %q, want nothing", logged.String())
	}
}

// A recovery that cannot run is logged rather than dropped, since the dispatcher has no window to show
// it in. The store is closed before the recovery, so its query fails the way a lost database would.
func TestRecoverOutboxFailureIsLogged(t *testing.T) {
	logged := captureLog(t)
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "outbox.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	recoveryApp(store).recoverOutbox()
	if !strings.Contains(logged.String(), "recover interrupted outbox sends") {
		t.Errorf("logged %q, want the recovery failure", logged.String())
	}
}
