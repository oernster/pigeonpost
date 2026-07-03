package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func draftTo(t *testing.T, address string) Draft {
	t.Helper()
	to, err := domain.NewEmailAddress("", address)
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	return Draft{To: []domain.EmailAddress{to}, Subject: "Hi", Body: "Hello"}
}

func TestComposeSend(t *testing.T) {
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	transport := &fakeMailTransport{}
	svc := NewComposeService(accounts, transport)

	if err := svc.Send(context.Background(), "a1", draftTo(t, "friend@example.com")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(transport.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(transport.sent))
	}
	if transport.sent[0].From().Address() != "user@example.com" {
		t.Errorf("From = %q, want the account address", transport.sent[0].From().Address())
	}
}

func TestComposeSendErrors(t *testing.T) {
	t.Run("account not found", func(t *testing.T) {
		accounts := newFakeAccountStore()
		svc := NewComposeService(accounts, &fakeMailTransport{})
		if err := svc.Send(context.Background(), "missing", draftTo(t, "f@example.com")); !errors.Is(err, ErrAccountNotFound) {
			t.Errorf("error = %v, want ErrAccountNotFound", err)
		}
	})

	t.Run("invalid draft", func(t *testing.T) {
		accounts := newFakeAccountStore()
		accounts.accounts["a1"] = testAccount(t, "a1")
		svc := NewComposeService(accounts, &fakeMailTransport{})
		// No recipients.
		if err := svc.Send(context.Background(), "a1", Draft{Subject: "x"}); !errors.Is(err, domain.ErrNoRecipients) {
			t.Errorf("error = %v, want ErrNoRecipients", err)
		}
	})

	t.Run("transport failure", func(t *testing.T) {
		accounts := newFakeAccountStore()
		accounts.accounts["a1"] = testAccount(t, "a1")
		transport := &fakeMailTransport{sendErr: errBoom}
		svc := NewComposeService(accounts, transport)
		if err := svc.Send(context.Background(), "a1", draftTo(t, "f@example.com")); !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want wrapped boom", err)
		}
	})
}
