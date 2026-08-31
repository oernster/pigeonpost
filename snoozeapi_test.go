package main

import (
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// snoozedFixture builds one resurfaced message with the given id and subject, on the given account.
func snoozedFixture(t *testing.T, id, subject, accountID string) application.SnoozedMessage {
	t.Helper()
	from, err := domain.NewEmailAddress("", "sender@example.test")
	if err != nil {
		t.Fatalf("build sender: %v", err)
	}
	summary, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID:        id,
		FolderID:  "f1",
		UID:       "1",
		MessageID: "<" + id + "@example.test>",
		From:      from,
		Subject:   subject,
		Date:      time.Unix(0, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("build message %q: %v", id, err)
	}
	return application.SnoozedMessage{
		Summary:   summary,
		Until:     time.Unix(0, 0).UTC(),
		AccountID: accountID,
	}
}

func TestResurfacedNotificationNamesTheFirstAndCountsTheRest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		messages  []application.SnoozedMessage
		wantTitle string
		wantBody  string
		wantOK    bool
	}{
		{
			name:     "nothing to announce",
			messages: nil,
			wantOK:   false,
		},
		{
			name:      "one message",
			messages:  []application.SnoozedMessage{snoozedFixture(t, "m1", "Roof quote", "a1")},
			wantTitle: "Snoozed message is back",
			wantBody:  "Roof quote",
			wantOK:    true,
		},
		{
			name: "several messages",
			messages: []application.SnoozedMessage{
				snoozedFixture(t, "m1", "Roof quote", "a1"),
				snoozedFixture(t, "m2", "Second", "a1"),
				snoozedFixture(t, "m3", "Third", "a1"),
			},
			wantTitle: "Snoozed messages are back",
			wantBody:  "Roof quote and 2 more",
			wantOK:    true,
		},
		{
			name:      "a message with no subject",
			messages:  []application.SnoozedMessage{snoozedFixture(t, "m1", "", "a1")},
			wantTitle: "Snoozed message is back",
			wantBody:  noSubjectLabel,
			wantOK:    true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			title, body, ok := resurfacedNotification(c.messages)
			if ok != c.wantOK {
				t.Fatalf("ok = %t, want %t", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if title != c.wantTitle || body != c.wantBody {
				t.Errorf("notification = %q / %q, want %q / %q", title, body, c.wantTitle, c.wantBody)
			}
		})
	}
}

// TestResurfacedDTOsCarryTheOwningAccount pins what a clicked toast needs to open the message it names.
// The resurface pop spans every account, so a toast can be about a message in an account other than the
// one on screen; without the account id the front end has no way to switch to it first.
func TestResurfacedDTOsCarryTheOwningAccount(t *testing.T) {
	t.Parallel()
	dtos := resurfacedDTOs([]application.SnoozedMessage{
		snoozedFixture(t, "m1", "Roof quote", "a1"),
		snoozedFixture(t, "m2", "Second", "a2"),
	})
	if len(dtos) != 2 {
		t.Fatalf("dtos = %d, want 2", len(dtos))
	}
	for i, want := range []struct{ id, accountID, folderID string }{
		{"m1", "a1", "f1"},
		{"m2", "a2", "f1"},
	} {
		got := dtos[i]
		if got.ID != want.id || got.AccountID != want.accountID || got.FolderID != want.folderID {
			t.Errorf("dto %d = %q on account %q in folder %q, want %q / %q / %q",
				i, got.ID, got.AccountID, got.FolderID, want.id, want.accountID, want.folderID)
		}
	}
}

// TestResurfacedDTOsOfNothingIsEmptyRatherThanNil keeps the event payload an array on the wire. A nil
// slice marshals as JSON null, which the front end would have to guard on before iterating.
func TestResurfacedDTOsOfNothingIsEmptyRatherThanNil(t *testing.T) {
	t.Parallel()
	if got := resurfacedDTOs(nil); got == nil || len(got) != 0 {
		t.Errorf("dtos = %v, want an empty non-nil slice", got)
	}
}

// TestRecordOrphanedSnoozesLogsOnlyARealOrphan covers the diagnostic trail for the one expiry that has
// nothing to show. Without it, a snooze whose message had gone would be indistinguishable afterwards
// from the scheduler never having run, which is precisely the ambiguity that made this hard to place.
func TestRecordOrphanedSnoozesLogsOnlyARealOrphan(t *testing.T) {
	t.Parallel()
	recorder := &fakeMailErrorRecorder{}
	app := &App{mailErrors: recorder}

	app.recordOrphanedSnoozes(0)
	app.recordOrphanedSnoozes(-1)
	if len(recorder.recorded) != 0 {
		t.Fatalf("recorded %v, want nothing when no snooze was orphaned", recorder.recorded)
	}

	app.recordOrphanedSnoozes(2)
	if len(recorder.recorded) != 1 {
		t.Fatalf("recorded %d errors, want 1", len(recorder.recorded))
	}
	if got := recorder.recorded[0].Error(); got == "" {
		t.Error("an orphaned snooze must be recorded with text saying what happened")
	}
}

// TestRecordOrphanedSnoozesToleratesNoRecorder keeps the announcement path safe in a build wired without
// an error log: a missing diagnostic must never take down the scheduler goroutine.
func TestRecordOrphanedSnoozesToleratesNoRecorder(t *testing.T) {
	t.Parallel()
	(&App{}).recordOrphanedSnoozes(1)
}

// fakeMailErrorRecorder is a hand-written mailErrorRecorder that keeps what it was given.
type fakeMailErrorRecorder struct{ recorded []error }

func (f *fakeMailErrorRecorder) Record(err error) { f.recorded = append(f.recorded, err) }
