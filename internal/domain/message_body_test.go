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

func TestMessageBodyHasNoInviteByDefault(t *testing.T) {
	body, err := NewMessageBody("m1", "plain", "")
	if err != nil {
		t.Fatalf("NewMessageBody: %v", err)
	}
	if body.HasInvite() {
		t.Errorf("a body built with no invite should report HasInvite false")
	}
	if body.Invite() != nil {
		t.Errorf("Invite() = %v, want nil", body.Invite())
	}
}

func TestMessageBodyWithInvite(t *testing.T) {
	body, err := NewMessageBody("m1", "plain", "")
	if err != nil {
		t.Fatalf("NewMessageBody: %v", err)
	}
	payload := []byte("BEGIN:VCALENDAR\r\nMETHOD:REQUEST\r\nEND:VCALENDAR\r\n")
	withInvite := body.WithInvite(payload)
	if !withInvite.HasInvite() {
		t.Errorf("WithInvite should mark the body as carrying an invite")
	}
	if string(withInvite.Invite()) != string(payload) {
		t.Errorf("Invite() = %q, want the payload", withInvite.Invite())
	}
	// The original body is unchanged: WithInvite copies.
	if body.HasInvite() {
		t.Errorf("WithInvite mutated the receiver")
	}
	// The stored bytes must not alias the caller's slice.
	payload[0] = 'X'
	if string(withInvite.Invite())[0] == 'X' {
		t.Errorf("WithInvite shares backing storage with the caller's slice")
	}
	// A returned slice must not alias the body's storage.
	got := withInvite.Invite()
	got[0] = 'Y'
	if string(withInvite.Invite())[0] == 'Y' {
		t.Errorf("Invite() returns a slice aliasing the body's storage")
	}
}

func TestMessageBodyWithInviteEmptyClears(t *testing.T) {
	body, err := NewMessageBody("m1", "plain", "")
	if err != nil {
		t.Fatalf("NewMessageBody: %v", err)
	}
	cleared := body.WithInvite(nil)
	if cleared.HasInvite() {
		t.Errorf("WithInvite(nil) should leave the body with no invite")
	}
}
