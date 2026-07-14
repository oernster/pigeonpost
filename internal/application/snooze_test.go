package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// newSnoozeFixture builds one inbox with two dated messages, the snooze service and the mailbox
// service reading the same fake store at the same clock, so hiding can be asserted through the real
// visible listings rather than against the fake's internals.
func newSnoozeFixture(t *testing.T) (*fakeMailStore, *SnoozeService, *MailboxService) {
	t.Helper()
	epoch := time.Unix(0, 0).UTC()
	mail := newFakeMailStore()
	mail.folders["a1"] = []domain.Folder{testFolder(t, "f1", "a1", "INBOX")}
	mail.messages["f1"] = []domain.MessageSummary{
		testDatedMessage(t, "m1", "f1", epoch.Add(time.Minute)),
		testDatedMessage(t, "m2", "f1", epoch.Add(2*time.Minute)),
	}
	clock := fakeClock{now: epoch}
	return mail, NewSnoozeService(mail, clock), NewMailboxService(mail, time.UTC, clock)
}

func messageIDsOf(messages []domain.MessageSummary) []string {
	out := make([]string, len(messages))
	for i, m := range messages {
		out[i] = m.ID()
	}
	return out
}

func TestSnoozeHidesFromVisibleListingsUntilUnsnoozed(t *testing.T) {
	_, snooze, mailbox := newSnoozeFixture(t)
	ctx := context.Background()
	until := time.Unix(0, 0).UTC().Add(time.Hour)

	if err := snooze.Snooze(ctx, "m2", until); err != nil {
		t.Fatalf("snooze: %v", err)
	}
	listed, err := mailbox.Messages(ctx, "f1")
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	if ids := messageIDsOf(listed); len(ids) != 1 || ids[0] != "m1" {
		t.Fatalf("visible messages = %v, want [m1]", ids)
	}
	page, err := mailbox.MessagesPage(ctx, "f1", false, 0, "", 10, false)
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if ids := messageIDsOf(page); len(ids) != 1 || ids[0] != "m1" {
		t.Fatalf("visible page = %v, want [m1]", ids)
	}
	threads, err := mailbox.Threads(ctx, "f1")
	if err != nil {
		t.Fatalf("threads: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("visible threads = %d, want 1", len(threads))
	}

	if err := snooze.Unsnooze(ctx, "m2"); err != nil {
		t.Fatalf("unsnooze: %v", err)
	}
	listed, err = mailbox.Messages(ctx, "f1")
	if err != nil {
		t.Fatalf("messages after unsnooze: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("messages after unsnooze = %v, want both", messageIDsOf(listed))
	}
}

func TestSnoozeRejectsAnInstantNotInTheFuture(t *testing.T) {
	mail, snooze, _ := newSnoozeFixture(t)
	if err := snooze.Snooze(context.Background(), "m2", time.Unix(0, 0).UTC()); !errors.Is(err, ErrSnoozeInPast) {
		t.Errorf("error = %v, want ErrSnoozeInPast", err)
	}
	if len(mail.snoozes) != 0 {
		t.Error("a rejected snooze must store nothing")
	}
}

func TestSnoozedListsSoonestFirst(t *testing.T) {
	_, snooze, _ := newSnoozeFixture(t)
	ctx := context.Background()
	epoch := time.Unix(0, 0).UTC()
	if err := snooze.Snooze(ctx, "m1", epoch.Add(2*time.Hour)); err != nil {
		t.Fatalf("snooze m1: %v", err)
	}
	if err := snooze.Snooze(ctx, "m2", epoch.Add(time.Hour)); err != nil {
		t.Fatalf("snooze m2: %v", err)
	}
	snoozed, err := snooze.Snoozed(ctx)
	if err != nil {
		t.Fatalf("snoozed: %v", err)
	}
	if len(snoozed) != 2 || snoozed[0].Summary.ID() != "m2" || snoozed[1].Summary.ID() != "m1" {
		t.Fatalf("snoozed order wrong: %+v", snoozed)
	}
	if !snoozed[0].Until.Equal(epoch.Add(time.Hour)) {
		t.Errorf("until = %v, want %v", snoozed[0].Until, epoch.Add(time.Hour))
	}
}

func TestSnoozePopDueReturnsAndClearsOnlyTheDue(t *testing.T) {
	mail, snooze, _ := newSnoozeFixture(t)
	ctx := context.Background()
	epoch := time.Unix(0, 0).UTC()
	// Seed directly so one snooze is already due at the fixture clock and one is not.
	mail.snoozes["m1"] = epoch.Add(-time.Minute)
	mail.snoozes["m2"] = epoch.Add(time.Hour)

	due, err := snooze.PopDue(ctx)
	if err != nil {
		t.Fatalf("pop due: %v", err)
	}
	if ids := messageIDsOf(due); len(ids) != 1 || ids[0] != "m1" {
		t.Fatalf("due = %v, want [m1]", ids)
	}
	again, err := snooze.PopDue(ctx)
	if err != nil {
		t.Fatalf("second pop: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("second pop = %v, want empty", messageIDsOf(again))
	}

	next, ok, err := snooze.NextDue(ctx)
	if err != nil {
		t.Fatalf("next due: %v", err)
	}
	if !ok || !next.Equal(epoch.Add(time.Hour)) {
		t.Errorf("next = %v ok = %t, want the remaining snooze", next, ok)
	}
}

func TestSnoozeNextDueWithNothingPending(t *testing.T) {
	_, snooze, _ := newSnoozeFixture(t)
	if _, ok, err := snooze.NextDue(context.Background()); err != nil || ok {
		t.Errorf("next due = ok %t err %v, want none and no error", ok, err)
	}
}

func TestSnoozeErrorsAreWrapped(t *testing.T) {
	ctx := context.Background()
	until := time.Unix(0, 0).UTC().Add(time.Hour)

	mail, snooze, _ := newSnoozeFixture(t)
	mail.setSnoozeErr = errBoom
	if err := snooze.Snooze(ctx, "m1", until); !errors.Is(err, errBoom) {
		t.Errorf("snooze error = %v, want wrapped boom", err)
	}

	mail, snooze, _ = newSnoozeFixture(t)
	mail.clearErr = errBoom
	if err := snooze.Unsnooze(ctx, "m1"); !errors.Is(err, errBoom) {
		t.Errorf("unsnooze error = %v, want wrapped boom", err)
	}

	mail, snooze, _ = newSnoozeFixture(t)
	mail.listSnoozeErr = errBoom
	if _, err := snooze.Snoozed(ctx); !errors.Is(err, errBoom) {
		t.Errorf("snoozed error = %v, want wrapped boom", err)
	}

	mail, snooze, _ = newSnoozeFixture(t)
	mail.popDueErr = errBoom
	if _, err := snooze.PopDue(ctx); !errors.Is(err, errBoom) {
		t.Errorf("pop-due error = %v, want wrapped boom", err)
	}

	mail, snooze, _ = newSnoozeFixture(t)
	mail.nextErr = errBoom
	if _, _, err := snooze.NextDue(ctx); !errors.Is(err, errBoom) {
		t.Errorf("next-due error = %v, want wrapped boom", err)
	}
}
