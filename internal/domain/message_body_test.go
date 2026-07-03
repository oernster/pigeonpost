package domain

import (
	"errors"
	"testing"
)

func TestNewMessageBody(t *testing.T) {
	body, err := NewMessageBody("  m1  ", "plain text", "<p>html</p>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body.MessageID() != "m1" {
		t.Errorf("MessageID = %q, want trimmed m1", body.MessageID())
	}
	if body.Plain() != "plain text" {
		t.Errorf("Plain = %q", body.Plain())
	}
	if body.HTML() != "<p>html</p>" {
		t.Errorf("HTML = %q", body.HTML())
	}
}

func TestNewMessageBodyEmptyID(t *testing.T) {
	if _, err := NewMessageBody("   ", "p", "h"); !errors.Is(err, ErrEmptyMessageID) {
		t.Errorf("error = %v, want ErrEmptyMessageID", err)
	}
}
