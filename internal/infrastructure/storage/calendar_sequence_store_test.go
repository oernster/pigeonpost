package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// The organiser's revision number (RFC 5545 SEQUENCE) must survive the store. Without it the rule that an
// older invitation never replaces a newer meeting has nothing to compare against once a meeting is saved.

const sequenceOrganizer = "chair@example.com"

// sequencedMeeting builds a meeting organised by sequenceOrganizer at the given revision.
func sequencedMeeting(t *testing.T, sequence int) domain.Event {
	t.Helper()
	org, err := domain.NewOrganizer(mustAddress(t, sequenceOrganizer), "")
	if err != nil {
		t.Fatalf("organizer: %v", err)
	}
	ev, err := domain.NewEvent(domain.EventInput{
		ID: "uid-m", UID: "uid-m", Summary: "Review", Start: baseStart(), Organizer: org, Sequence: sequence,
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	return ev
}

func mustAddress(t *testing.T, address string) domain.EmailAddress {
	t.Helper()
	a, err := domain.NewEmailAddress("", address)
	if err != nil {
		t.Fatalf("address %q: %v", address, err)
	}
	return a
}

func TestEventSequenceSurvivesTheStore(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.SaveEvent(ctx, sequencedMeeting(t, 3)); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	got, err := store.GetEvent(ctx, "uid-m")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.Sequence() != 3 {
		t.Errorf("SaveEvent then GetEvent: sequence = %d, want 3", got.Sequence())
	}
	if err := store.SaveSyncedEvent(ctx, sequencedMeeting(t, 4), "/c/m.ics", "e1"); err != nil {
		t.Fatalf("SaveSyncedEvent: %v", err)
	}
	synced, err := store.EventsByHref(ctx, "/c/m.ics")
	if err != nil || len(synced) != 1 {
		t.Fatalf("EventsByHref = %d events, %v", len(synced), err)
	}
	if synced[0].Sequence() != 4 {
		t.Errorf("SaveSyncedEvent then EventsByHref: sequence = %d, want 4", synced[0].Sequence())
	}
}

// staleCodec decodes every invite to one scripted scheduling message; the send half is never reached.
type staleCodec struct{ sched domain.SchedulingMessage }

func (c staleCodec) DecodeScheduling([]byte) (domain.SchedulingMessage, error) { return c.sched, nil }
func (staleCodec) EncodeRequest([]domain.Event) ([]byte, error)                { return nil, nil }
func (staleCodec) EncodeCancel([]domain.Event) ([]byte, error)                 { return nil, nil }
func (staleCodec) EncodeReply(domain.Event, domain.EmailAddress, domain.ParticipationStatus) ([]byte, error) {
	return nil, nil
}

func TestRespondRefusesAnOlderInvitationAgainstTheRealStore(t *testing.T) {
	// The stored meeting is at revision 3; an invitation at revision 1 arrives. With the revision kept in
	// the store, answering it is refused and the stored meeting is left as it was.
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.SaveAccount(ctx, buildAccount(t, "a1")); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}
	saveFolder(t, store, "a1", "INBOX")
	folderID := domain.FolderIDFor("a1", "INBOX")
	if err := store.SaveMessages(ctx, folderID, []domain.MessageSummary{buildMessageIn(t, "m1", folderID, false)}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}
	body, err := domain.NewMessageBody("m1", "", "")
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	if err := store.SaveMessageBody(ctx, body.WithInvite([]byte("BEGIN:VCALENDAR"))); err != nil {
		t.Fatalf("SaveMessageBody: %v", err)
	}
	if err := store.SaveEvent(ctx, sequencedMeeting(t, 3)); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	sched, err := domain.NewSchedulingMessage(domain.MethodRequest, []domain.Event{sequencedMeeting(t, 1)})
	if err != nil {
		t.Fatalf("scheduling message: %v", err)
	}
	svc := application.NewSchedulingService(staleCodec{sched: sched}, store, store, store, store, nil, nil, nil, nil,
		func() string { return "unused" })

	if err := svc.Respond(ctx, "m1", domain.PartStatAccepted); !errors.Is(err, application.ErrStaleInvitation) {
		t.Fatalf("Respond err = %v, want ErrStaleInvitation", err)
	}
	got, err := store.GetEvent(ctx, "uid-m")
	if err != nil || got.Sequence() != 3 {
		t.Errorf("stored meeting = revision %d (%v), want 3 untouched", got.Sequence(), err)
	}
}
