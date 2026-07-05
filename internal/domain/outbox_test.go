package domain

import (
	"errors"
	"testing"
	"time"
)

func outboxMessage(t *testing.T) OutgoingMessage {
	t.Helper()
	msg, err := NewOutgoingMessage(OutgoingMessageInput{
		From: mustAddr(t, "me@example.com"),
		To:   []EmailAddress{mustAddr(t, "a@example.com")},
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	return msg
}

func TestNewOutboxItem(t *testing.T) {
	created := time.Date(2026, time.July, 3, 9, 0, 0, 0, time.UTC)
	item, err := NewOutboxItem(" q1 ", " a1 ", OutboxSend, outboxMessage(t), created)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ID() != "q1" || item.AccountID() != "a1" {
		t.Errorf("id/account not trimmed: %q / %q", item.ID(), item.AccountID())
	}
	if item.Kind() != OutboxSend {
		t.Errorf("kind = %v, want OutboxSend", item.Kind())
	}
	if !item.CreatedAt().Equal(created) {
		t.Errorf("createdAt = %v, want %v", item.CreatedAt(), created)
	}
	if item.Message().From().Address() != "me@example.com" {
		t.Errorf("message sender = %q", item.Message().From().Address())
	}
}

func TestOutboxItemFailure(t *testing.T) {
	created := time.Date(2026, time.July, 3, 9, 0, 0, 0, time.UTC)
	item, err := NewOutboxItem("q1", "a1", OutboxSend, outboxMessage(t), created)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Failed() || item.Failure() != "" {
		t.Errorf("a fresh item should not be failed, got failed=%v reason=%q", item.Failed(), item.Failure())
	}

	failed := item.WithFailure("mailbox unavailable")
	if !failed.Failed() || failed.Failure() != "mailbox unavailable" {
		t.Errorf("WithFailure did not record the reason, got failed=%v reason=%q", failed.Failed(), failed.Failure())
	}
	if item.Failed() {
		t.Error("WithFailure must not mutate the original item")
	}
}

func TestNewOutboxItemInvalid(t *testing.T) {
	valid := outboxMessage(t)
	cases := map[string]struct {
		id, account string
		kind        OutboxKind
		msg         OutgoingMessage
		want        error
	}{
		"empty id":      {"", "a1", OutboxSend, valid, ErrEmptyOutboxID},
		"empty account": {"q1", "", OutboxSend, valid, ErrEmptyAccountID},
		"invalid kind":  {"q1", "a1", OutboxKind(99), valid, ErrInvalidOutboxKind},
		"no sender":     {"q1", "a1", OutboxSend, OutgoingMessage{}, ErrNoSender},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewOutboxItem(tc.id, tc.account, tc.kind, tc.msg, time.Time{}); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestOutboxKind(t *testing.T) {
	cases := map[OutboxKind]struct {
		str   string
		valid bool
	}{
		OutboxSend:     {"send", true},
		OutboxDraft:    {"draft", true},
		OutboxKind(99): {"unknown", false},
	}
	for kind, want := range cases {
		if kind.String() != want.str {
			t.Errorf("String(%d) = %q, want %q", kind, kind.String(), want.str)
		}
		if kind.Valid() != want.valid {
			t.Errorf("Valid(%d) = %v, want %v", kind, kind.Valid(), want.valid)
		}
	}
}
