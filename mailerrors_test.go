package main

import (
	"errors"
	"fmt"
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
	result := bulkResult([]string{"a", "b"}, nil, nil, err)
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
	result := bulkResult([]string{"a"}, []string{"a"}, nil, err)
	if result.Offline {
		t.Fatal("bulkResult.Offline = true for a non-offline error, want false")
	}
	if result.Error != err.Error() {
		t.Fatalf("bulkResult.Error = %q, want the raw detail preserved", result.Error)
	}
}

func TestBulkResultNoErrorIsClean(t *testing.T) {
	t.Parallel()
	result := bulkResult([]string{"a"}, []string{"a"}, map[string]string{}, nil)
	if result.Offline || result.Error != "" {
		t.Fatalf("bulkResult with no error = {Offline:%v Error:%q}, want clean", result.Offline, result.Error)
	}
}
