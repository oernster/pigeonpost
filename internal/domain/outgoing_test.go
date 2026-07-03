package domain

import (
	"errors"
	"testing"
)

func mustAddr(t *testing.T, address string) EmailAddress {
	t.Helper()
	a, err := NewEmailAddress("", address)
	if err != nil {
		t.Fatalf("address %q: %v", address, err)
	}
	return a
}

func TestNewOutgoingMessage(t *testing.T) {
	msg, err := NewOutgoingMessage(OutgoingMessageInput{
		From:    mustAddr(t, "me@example.com"),
		To:      []EmailAddress{mustAddr(t, "a@example.com"), mustAddr(t, "b@example.com")},
		Cc:      []EmailAddress{mustAddr(t, "c@example.com")},
		Subject: "  Hello  ",
		Body:    "Body text",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.From().Address() != "me@example.com" {
		t.Errorf("From = %q", msg.From().Address())
	}
	if len(msg.To()) != 2 || len(msg.Cc()) != 1 {
		t.Errorf("recipients wrong: to=%d cc=%d", len(msg.To()), len(msg.Cc()))
	}
	if msg.Subject() != "Hello" {
		t.Errorf("Subject = %q, want trimmed Hello", msg.Subject())
	}
	if msg.Body() != "Body text" {
		t.Errorf("Body = %q", msg.Body())
	}
	if len(msg.Recipients()) != 3 {
		t.Errorf("Recipients = %d, want 3", len(msg.Recipients()))
	}
}

func TestNewOutgoingMessageInvalid(t *testing.T) {
	valid := mustAddr(t, "ok@example.com")
	cases := map[string]struct {
		in   OutgoingMessageInput
		want error
	}{
		"no sender":     {OutgoingMessageInput{To: []EmailAddress{valid}}, ErrNoSender},
		"no recipients": {OutgoingMessageInput{From: valid}, ErrNoRecipients},
		"zero in to":    {OutgoingMessageInput{From: valid, To: []EmailAddress{{}}}, ErrNoRecipients},
		"zero in cc":    {OutgoingMessageInput{From: valid, To: []EmailAddress{valid}, Cc: []EmailAddress{{}}}, ErrNoRecipients},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewOutgoingMessage(tc.in); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestOutgoingMessageGettersCopy(t *testing.T) {
	msg, err := NewOutgoingMessage(OutgoingMessageInput{
		From: mustAddr(t, "me@example.com"),
		To:   []EmailAddress{mustAddr(t, "a@example.com")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := msg.To()
	got[0] = EmailAddress{}
	if msg.To()[0].IsZero() {
		t.Error("To() must return a copy, not the internal slice")
	}
}
