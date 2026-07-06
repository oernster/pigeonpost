package application

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/ics"
)

// TestSendRequestWithRealCodecReachesTransport proves the actual organizer send path end to end: given a
// meeting with attendees, SendRequest builds a REQUEST with the real ICS codec (not the fake) and hands
// the transport a message addressed to the attendees carrying a text/calendar REQUEST part. It exercises
// the real encoder and message construction the fake-codec tests skip.
func TestSendRequestWithRealCodecReachesTransport(t *testing.T) {
	accounts := newFakeAccountStore()
	accounts.accounts["a1"] = testAccount(t, "a1")
	transport := &fakeMailTransport{}
	svc := NewSchedulingService(ics.New(), &fakeCalendarStore{}, newFakeMailStore(), accounts, transport)

	meeting := schedMeeting(t, "uid-1", "user@example.com", time.Time{},
		"guest1@example.com", "guest2@example.com")

	if err := svc.SendRequest(context.Background(), "a1", []domain.Event{meeting}); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if len(transport.sent) != 1 {
		t.Fatalf("transport received %d messages, want 1", len(transport.sent))
	}
	sent := transport.sent[0]

	recipients := make(map[string]bool)
	for _, r := range sent.Recipients() {
		recipients[r.Address()] = true
	}
	for _, want := range []string{"guest1@example.com", "guest2@example.com"} {
		if !recipients[want] {
			t.Errorf("recipient %q missing; recipients = %v", want, recipients)
		}
	}

	part := sent.Calendar()
	if part.IsZero() {
		t.Fatal("sent message carries no calendar part")
	}
	if part.Method() != domain.MethodRequest {
		t.Errorf("calendar method = %q, want REQUEST", part.Method())
	}
	payload := string(part.Content())
	if !strings.Contains(payload, "METHOD:REQUEST") {
		t.Errorf("payload missing METHOD:REQUEST:\n%s", payload)
	}
	if !strings.Contains(payload, "guest1@example.com") {
		t.Errorf("payload missing attendee guest1:\n%s", payload)
	}
}
