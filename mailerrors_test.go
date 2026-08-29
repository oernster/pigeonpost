package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func TestFriendlyMailErrorNilStaysNil(t *testing.T) {
	t.Parallel()
	if err := friendlyMailError(nil); err != nil {
		t.Fatalf("friendlyMailError(nil) = %v, want nil", err)
	}
}

func TestFriendlyMailErrorTranslatesOffline(t *testing.T) {
	t.Parallel()
	// A connectivity failure is wrapped several layers deep, exactly as it reaches the facade.
	wrapped := fmt.Errorf("delete message %q on server: %w", "m1",
		fmt.Errorf("imap: dial mail.example.com:993: %w", domain.ErrOffline))
	got := friendlyMailError(wrapped)
	if !errors.Is(got, errOffline) {
		t.Fatalf("friendlyMailError did not translate an offline error, got %v", got)
	}
	if got.Error() != errOffline.Error() {
		t.Fatalf("friendlyMailError message = %q, want the plain offline message", got.Error())
	}
}

func TestFriendlyMailErrorTranslatesIMAPRefused(t *testing.T) {
	t.Parallel()
	// The shape the Microsoft sign-in produces: the server's own refusal, wrapped by the adapter and
	// again by the setup service, exactly as it reaches the facade.
	wrapped := fmt.Errorf("verify microsoft account %q: %w", "a@hotmail.com",
		fmt.Errorf("imap: xoauth2 %q: %w", "a@hotmail.com",
			errors.Join(errors.New("imap: NO User is authenticated but not connected."), domain.ErrIMAPRefused)))
	got := friendlyMailError(wrapped)
	if !errors.Is(got, errIMAPRefused) {
		t.Fatalf("friendlyMailError did not translate an IMAP-refused error, got %v", got)
	}
	// The point of the message is that it can be acted on, so the setting it names is part of the contract.
	// Both names the section goes by, since which one the reader sees depends on their Outlook version,
	// plus the two things measurement forced into it: that the server REFUSED rather than that a setting
	// is off, then that a new mailbox is refused anyway. A message asserting the switch is off was shown
	// to a mailbox whose switch was verifiably on, which reads as the app being broken rather than
	// blocked.
	for _, want := range []string{
		"outlook.com",
		"Sync email",
		"Forwarding and IMAP",
		"refused",
		"new account",
	} {
		if !strings.Contains(got.Error(), want) {
			t.Fatalf("message %q does not name %q, so it cannot be acted on", got.Error(), want)
		}
	}
}

func TestFriendlyMailErrorLeavesOtherErrorsUnchanged(t *testing.T) {
	t.Parallel()
	other := errors.New("mailbox does not exist")
	if got := friendlyMailError(other); got != other {
		t.Fatalf("friendlyMailError changed a non-offline error to %v", got)
	}
}

func TestIsOffline(t *testing.T) {
	t.Parallel()
	if isOffline(nil) {
		t.Fatal("isOffline(nil) = true, want false")
	}
	if isOffline(errors.New("other")) {
		t.Fatal("isOffline(other) = true, want false")
	}
	if !isOffline(fmt.Errorf("wrap: %w", domain.ErrOffline)) {
		t.Fatal("isOffline(wrapped ErrOffline) = false, want true")
	}
}

func TestBulkResultOfflineShowsFriendlyMessage(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("delete 2 messages in %q on server: %w", "inbox", domain.ErrOffline)
	result := (&App{}).bulkResult([]string{"a", "b"}, nil, nil, err)
	if !result.Offline {
		t.Fatal("bulkResult.Offline = false for an offline error, want true")
	}
	if result.Error != errOffline.Error() {
		t.Fatalf("bulkResult.Error = %q, want the plain offline message", result.Error)
	}
	if result.Failed != 2 {
		t.Fatalf("bulkResult.Failed = %d, want 2", result.Failed)
	}
}

func TestBulkResultNonOfflineKeepsDetail(t *testing.T) {
	t.Parallel()
	err := errors.New("locate folder \"x\": not found")
	result := (&App{}).bulkResult([]string{"a"}, []string{"a"}, nil, err)
	if result.Offline {
		t.Fatal("bulkResult.Offline = true for a non-offline error, want false")
	}
	if result.Error != err.Error() {
		t.Fatalf("bulkResult.Error = %q, want the raw detail preserved", result.Error)
	}
}

func TestBulkResultNoErrorIsClean(t *testing.T) {
	t.Parallel()
	result := (&App{}).bulkResult([]string{"a"}, []string{"a"}, map[string]string{}, nil)
	if result.Offline || result.Error != "" {
		t.Fatalf("bulkResult with no error = {Offline:%v Error:%q}, want clean", result.Offline, result.Error)
	}
}

// maxIMAPMessageChars caps the IMAP-refused message. An earlier draft of it spelled out every obstacle
// and reached ninety words, which is a wall of text in a dialog box: nobody reads it, so the guidance it
// carries is worth less than a shorter message they act on. The cap sits a little above the current
// length so ordinary rewording is free while a return to a paragraph is not.
const maxIMAPMessageChars = 280

func TestIMAPRefusedMessageStaysShort(t *testing.T) {
	t.Parallel()
	if got := len(errIMAPRefused.Error()); got > maxIMAPMessageChars {
		t.Fatalf("errIMAPRefused is %d characters, over the %d cap: shorten it or move the detail to the README", got, maxIMAPMessageChars)
	}
}

// recordingSpy is a hand-written mailErrorRecorder that keeps what it was handed, so a test can assert
// on the evidence rather than on a filesystem.
type recordingSpy struct {
	recorded []error
}

// Record keeps err for inspection.
func (s *recordingSpy) Record(err error) {
	s.recorded = append(s.recorded, err)
}

func TestMailErrorRecordsTheRawErrorItReplaces(t *testing.T) {
	t.Parallel()
	spy := &recordingSpy{}
	app := &App{mailErrors: spy}
	raw := fmt.Errorf("imap: xoauth2 %q: NO User is authenticated but not connected: %w", "a@b.com", domain.ErrIMAPRefused)

	got := app.mailError(raw)

	if got != errIMAPRefused {
		t.Fatalf("mailError returned %v, want the message fit to read", got)
	}
	if len(spy.recorded) != 1 || spy.recorded[0] != raw {
		t.Fatalf("recorded %v, want exactly the raw error: replacing it is what destroys the evidence", spy.recorded)
	}
}

func TestMailErrorDoesNotRecordAnErrorItPassesThrough(t *testing.T) {
	t.Parallel()
	spy := &recordingSpy{}
	app := &App{mailErrors: spy}
	// Nothing is asserted about this error's cause and its own detail reaches the user intact, so there
	// is nothing to preserve. Recording it too would bury the cases that matter in ordinary noise.
	raw := errors.New("locate folder \"x\": not found")

	if got := app.mailError(raw); got != raw {
		t.Fatalf("mailError returned %v, want the error unchanged", got)
	}
	if len(spy.recorded) != 0 {
		t.Fatalf("recorded %v for a pass-through error, want nothing", spy.recorded)
	}
}

func TestMailErrorLeavesNilAlone(t *testing.T) {
	t.Parallel()
	spy := &recordingSpy{}
	app := &App{mailErrors: spy}
	if got := app.mailError(nil); got != nil {
		t.Fatalf("mailError(nil) = %v, want nil", got)
	}
	if len(spy.recorded) != 0 {
		t.Fatalf("recorded %v for a nil error, want nothing", spy.recorded)
	}
}
