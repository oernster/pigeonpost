package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewDraftRecovery(t *testing.T) {
	saved := time.Date(2026, time.July, 6, 10, 30, 0, 0, time.UTC)
	rec, err := NewDraftRecovery(DraftRecoveryInput{
		AccountID: " acc-1 ",
		To:        "a@example.com, b@examp",
		Cc:        "c@example.com",
		Bcc:       "d@example.com",
		Subject:   "Half a subject",
		BodyHTML:  "<p>Still typing</p>",
	}, saved)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.AccountID() != "acc-1" {
		t.Errorf("account id not trimmed: %q", rec.AccountID())
	}
	if rec.To() != "a@example.com, b@examp" {
		t.Errorf("to = %q", rec.To())
	}
	if rec.Cc() != "c@example.com" {
		t.Errorf("cc = %q", rec.Cc())
	}
	if rec.Bcc() != "d@example.com" {
		t.Errorf("bcc = %q", rec.Bcc())
	}
	if rec.Subject() != "Half a subject" {
		t.Errorf("subject = %q", rec.Subject())
	}
	if rec.BodyHTML() != "<p>Still typing</p>" {
		t.Errorf("body = %q", rec.BodyHTML())
	}
	if !rec.SavedAt().Equal(saved) {
		t.Errorf("savedAt = %v, want %v", rec.SavedAt(), saved)
	}
}

func TestNewDraftRecoveryAllowsEmptyContent(t *testing.T) {
	// Only the account is required; a snapshot of an otherwise-blank compose is valid because the point
	// is to preserve whatever the user had, however little.
	rec, err := NewDraftRecovery(DraftRecoveryInput{AccountID: "acc-1"}, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.To() != "" || rec.Cc() != "" || rec.Bcc() != "" || rec.Subject() != "" || rec.BodyHTML() != "" {
		t.Error("empty content fields should round-trip as empty")
	}
}

func TestNewDraftRecoveryRequiresAccount(t *testing.T) {
	for name, id := range map[string]string{"empty": "", "whitespace": "   "} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewDraftRecovery(DraftRecoveryInput{AccountID: id}, time.Time{}); !errors.Is(err, ErrEmptyAccountID) {
				t.Errorf("error = %v, want ErrEmptyAccountID", err)
			}
		})
	}
}
