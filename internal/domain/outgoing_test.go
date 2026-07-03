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

func mustAttachment(t *testing.T, filename, contentType string, content []byte) Attachment {
	t.Helper()
	a, err := NewAttachment(filename, contentType, content)
	if err != nil {
		t.Fatalf("attachment %q: %v", filename, err)
	}
	return a
}

func TestNewOutgoingMessage(t *testing.T) {
	msg, err := NewOutgoingMessage(OutgoingMessageInput{
		From:     mustAddr(t, "me@example.com"),
		To:       []EmailAddress{mustAddr(t, "a@example.com"), mustAddr(t, "b@example.com")},
		Cc:       []EmailAddress{mustAddr(t, "c@example.com")},
		Bcc:      []EmailAddress{mustAddr(t, "d@example.com")},
		Subject:  "  Hello  ",
		Body:     "Body text",
		HTMLBody: "<p>Body text</p>",
		Attachments: []Attachment{
			mustAttachment(t, "note.txt", "text/plain", []byte("hi")),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg.Attachments()) != 1 || msg.Attachments()[0].Filename() != "note.txt" {
		t.Errorf("attachments wrong: %+v", msg.Attachments())
	}
	if msg.From().Address() != "me@example.com" {
		t.Errorf("From = %q", msg.From().Address())
	}
	if msg.HTMLBody() != "<p>Body text</p>" {
		t.Errorf("HTMLBody = %q", msg.HTMLBody())
	}
	if len(msg.To()) != 2 || len(msg.Cc()) != 1 || len(msg.Bcc()) != 1 {
		t.Errorf("recipients wrong: to=%d cc=%d bcc=%d", len(msg.To()), len(msg.Cc()), len(msg.Bcc()))
	}
	if msg.Subject() != "Hello" {
		t.Errorf("Subject = %q, want trimmed Hello", msg.Subject())
	}
	if msg.Body() != "Body text" {
		t.Errorf("Body = %q", msg.Body())
	}
	if len(msg.Recipients()) != 4 {
		t.Errorf("Recipients = %d, want 4", len(msg.Recipients()))
	}
}

func TestRecipientsDeduplicatesAcrossToAndCc(t *testing.T) {
	// The same mailbox in both To and Cc must yield one envelope recipient, compared case-insensitively,
	// so the transport does not issue a duplicate RCPT for it.
	msg, err := NewOutgoingMessage(OutgoingMessageInput{
		From: mustAddr(t, "me@example.com"),
		To:   []EmailAddress{mustAddr(t, "friend@example.com"), mustAddr(t, "other@example.com")},
		Cc:   []EmailAddress{mustAddr(t, "FRIEND@example.com")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := msg.Recipients()
	if len(got) != 2 {
		t.Fatalf("Recipients = %d, want 2 distinct", len(got))
	}
	if got[0].Address() != "friend@example.com" || got[1].Address() != "other@example.com" {
		t.Errorf("Recipients = %v, want To ordering with the Cc duplicate dropped", got)
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
		"zero in bcc":   {OutgoingMessageInput{From: valid, To: []EmailAddress{valid}, Bcc: []EmailAddress{{}}}, ErrNoRecipients},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewOutgoingMessage(tc.in); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewDraftMessage(t *testing.T) {
	// A draft may have no recipients and an empty body: the user is still composing it.
	msg, err := NewDraftMessage(OutgoingMessageInput{
		From:    mustAddr(t, "me@example.com"),
		Subject: "  Later  ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg.To()) != 0 || len(msg.Cc()) != 0 {
		t.Errorf("expected no recipients, got to=%d cc=%d", len(msg.To()), len(msg.Cc()))
	}
	if msg.Subject() != "Later" {
		t.Errorf("Subject = %q, want trimmed Later", msg.Subject())
	}

	// Recipients that ARE present are carried through.
	withTo, err := NewDraftMessage(OutgoingMessageInput{
		From: mustAddr(t, "me@example.com"),
		To:   []EmailAddress{mustAddr(t, "a@example.com")},
		Cc:   []EmailAddress{mustAddr(t, "c@example.com")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(withTo.To()) != 1 || len(withTo.Cc()) != 1 {
		t.Errorf("recipients wrong: to=%d cc=%d", len(withTo.To()), len(withTo.Cc()))
	}
}

func TestNewDraftMessageInvalid(t *testing.T) {
	valid := mustAddr(t, "ok@example.com")
	cases := map[string]struct {
		in   OutgoingMessageInput
		want error
	}{
		"no sender":   {OutgoingMessageInput{To: []EmailAddress{valid}}, ErrNoSender},
		"zero in to":  {OutgoingMessageInput{From: valid, To: []EmailAddress{{}}}, ErrNoRecipients},
		"zero in cc":  {OutgoingMessageInput{From: valid, Cc: []EmailAddress{{}}}, ErrNoRecipients},
		"zero in bcc": {OutgoingMessageInput{From: valid, Bcc: []EmailAddress{{}}}, ErrNoRecipients},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewDraftMessage(tc.in); !errors.Is(err, tc.want) {
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
