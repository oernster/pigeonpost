package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

func day(d, h int) time.Time { return time.Date(2026, time.July, d, h, 0, 0, 0, time.UTC) }

func winFrom() time.Time { return day(1, 0) }
func winTo() time.Time   { return day(31, 0) }

func recurringMaster(t *testing.T, id, uid string) domain.Event {
	t.Helper()
	e, err := domain.NewEvent(domain.EventInput{
		ID: id, UID: uid, CalendarID: "cal1", Summary: "Standup", Start: day(4, 9), End: day(4, 10),
		Recurrence: "FREQ=DAILY;COUNT=5",
	})
	if err != nil {
		t.Fatalf("recurringMaster: %v", err)
	}
	return e
}

func overrideAt(t *testing.T, id, uid string, occ, start time.Time) domain.Event {
	t.Helper()
	e, err := domain.NewEvent(domain.EventInput{
		ID: id, UID: uid, CalendarID: "cal1", Summary: "Standup moved", Start: start, End: start.Add(time.Hour),
		RecurrenceID: occ,
	})
	if err != nil {
		t.Fatalf("overrideAt: %v", err)
	}
	return e
}

func singleEvent(t *testing.T, id string, start, end time.Time) domain.Event {
	t.Helper()
	e, err := domain.NewEvent(domain.EventInput{ID: id, CalendarID: "cal1", Summary: "One off", Start: start, End: end})
	if err != nil {
		t.Fatalf("singleEvent: %v", err)
	}
	return e
}

func instanceStarts(insts []domain.EventInstance) []time.Time {
	out := make([]time.Time, len(insts))
	for i, x := range insts {
		out[i] = x.Start()
	}
	return out
}

func dailyExpander() *fakeRecurrence {
	return &fakeRecurrence{expandFunc: func(e domain.Event, from, to time.Time) ([]domain.EventInstance, error) {
		var out []domain.EventInstance
		for d := 4; d <= 8; d++ {
			start := day(d, 9)
			out = append(out, domain.NewEventInstance(e, start, start.Add(time.Hour), start))
		}
		return out, nil
	}}
}

func TestListEventInstancesListError(t *testing.T) {
	svc := NewCalendarService(&fakeCalendarStore{listEvtErr: errBoom}, fixedID("x"), &fakeRecurrence{})
	if _, err := svc.ListEventInstances(context.Background(), winFrom(), winTo()); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestListEventInstancesExpandsSuppressesAndFilters(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	override := overrideAt(t, "o1", "series-1", day(6, 9), day(6, 14))
	inWindow := singleEvent(t, "s1", day(9, 12), day(9, 13))
	past := singleEvent(t, "s2", day(9, 12).AddDate(0, 0, -60), day(9, 13).AddDate(0, 0, -60))
	store := &fakeCalendarStore{events: []domain.Event{master, override, inWindow, past}}
	svc := NewCalendarService(store, fixedID("x"), dailyExpander())

	insts, err := svc.ListEventInstances(context.Background(), winFrom(), winTo())
	if err != nil {
		t.Fatalf("ListEventInstances: %v", err)
	}
	// Expected: days 4,5,7,8 from the rule (day 6 suppressed), the override at day 6 14:00, and the single
	// on day 9. The past single is filtered out. Sorted by start.
	got := instanceStarts(insts)
	want := []time.Time{day(4, 9), day(5, 9), day(6, 14), day(7, 9), day(8, 9), day(9, 12)}
	if len(got) != len(want) {
		t.Fatalf("got %d instances %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Errorf("instance[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestListEventInstancesExpandErrorFallsBackWhenInWindow(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{events: []domain.Event{master}}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{expandErr: errBoom})
	insts, err := svc.ListEventInstances(context.Background(), winFrom(), winTo())
	if err != nil {
		t.Fatalf("ListEventInstances: %v", err)
	}
	if len(insts) != 1 || !insts[0].Start().Equal(day(4, 9)) {
		t.Errorf("fallback = %v, want a single instance at the master start", instanceStarts(insts))
	}
}

func TestListEventInstancesExpandErrorSkipsWhenOutOfWindow(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{events: []domain.Event{master}}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{expandErr: errBoom})
	// A window entirely before the master start: the fallback instance does not overlap, so nothing shows.
	insts, err := svc.ListEventInstances(context.Background(), day(1, 0), day(2, 0))
	if err != nil {
		t.Fatalf("ListEventInstances: %v", err)
	}
	if len(insts) != 0 {
		t.Errorf("expected no instances, got %v", instanceStarts(insts))
	}
}

func TestListEventInstancesPointEventWithoutEnd(t *testing.T) {
	reminder, _ := domain.NewEvent(domain.EventInput{ID: "r1", Summary: "Reminder", Start: day(9, 9)})
	store := &fakeCalendarStore{events: []domain.Event{reminder}}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	// In window: the zero-end event is treated as a point in time.
	if insts, _ := svc.ListEventInstances(context.Background(), winFrom(), winTo()); len(insts) != 1 {
		t.Errorf("point event in window = %v, want 1", instanceStarts(insts))
	}
	// Start after the window end: filtered out.
	if insts, _ := svc.ListEventInstances(context.Background(), day(1, 0), day(2, 0)); len(insts) != 0 {
		t.Errorf("point event after window = %v, want 0", instanceStarts(insts))
	}
}

func TestListEventInstancesGroupsSeriesWithoutUID(t *testing.T) {
	master, _ := domain.NewEvent(domain.EventInput{
		ID: "m2", CalendarID: "cal1", Summary: "Standup", Start: day(4, 9), End: day(4, 10),
		Recurrence: "FREQ=DAILY;COUNT=5",
	})
	// The override links to the UID-less master by carrying the master id as its UID (its series key).
	override := overrideAt(t, "o2", "m2", day(6, 9), day(6, 14))
	store := &fakeCalendarStore{events: []domain.Event{master, override}}
	svc := NewCalendarService(store, fixedID("x"), dailyExpander())
	insts, err := svc.ListEventInstances(context.Background(), winFrom(), winTo())
	if err != nil {
		t.Fatalf("ListEventInstances: %v", err)
	}
	// Day 6 from the rule is suppressed by the override, which appears at 14:00 instead.
	for _, x := range insts {
		if x.Start().Equal(day(6, 9)) {
			t.Errorf("occurrence at day 6 09:00 should be suppressed by the override")
		}
	}
	var sawOverride bool
	for _, x := range insts {
		if x.Start().Equal(day(6, 14)) {
			sawOverride = true
		}
	}
	if !sawOverride {
		t.Errorf("override at day 6 14:00 missing: %v", instanceStarts(insts))
	}
}

func TestUpdateEventScopeLoadError(t *testing.T) {
	svc := NewCalendarService(&fakeCalendarStore{getEvtErr: errBoom}, fixedID("x"), &fakeRecurrence{})
	if err := svc.UpdateEventScope(context.Background(), ScopeAll, EventInput{ID: "m1"}, day(4, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestUpdateEventScopeUnknown(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	svc := NewCalendarService(&fakeCalendarStore{gotEvent: master}, fixedID("x"), &fakeRecurrence{})
	if err := svc.UpdateEventScope(context.Background(), EventScope(99), editInput("m1", "series-1"), day(4, 9)); err == nil {
		t.Errorf("expected an error for an unknown scope")
	}
}

func editInput(id, uid string) EventInput {
	return EventInput{
		ID: id, UID: uid, CalendarID: "cal1", Summary: "Edited", Start: day(4, 9), End: day(4, 10),
		Recurrence: "FREQ=DAILY;COUNT=5",
	}
}

func TestUpdateEventScopeAll(t *testing.T) {
	master, _ := domain.NewEvent(domain.EventInput{
		ID: "m1", UID: "series-1", CalendarID: "cal1", Summary: "Standup", Start: day(4, 9), End: day(4, 10),
		Recurrence: "FREQ=DAILY;COUNT=5", ExDates: []time.Time{day(6, 9)}, Extra: "BEGIN:VEVENT\r\nEND:VEVENT\r\n",
	})
	store := &fakeCalendarStore{gotEvent: master}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.UpdateEventScope(context.Background(), ScopeAll, editInput("m1", "series-1"), day(4, 9)); err != nil {
		t.Fatalf("UpdateEventScope: %v", err)
	}
	saved := store.savedEvt[0]
	if saved.Summary() != "Edited" || saved.ID() != "m1" {
		t.Errorf("edit not applied: %+v", saved)
	}
	if len(saved.ExDates()) != 1 || saved.Extra() == "" {
		t.Errorf("all-scope edit dropped the master's recurrence set or extra: %+v", saved)
	}
}

func TestUpdateEventScopeAllBuildError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	bad := editInput("m1", "series-1")
	bad.Summary = "  "
	if err := svc.UpdateEventScope(context.Background(), ScopeAll, bad, day(4, 9)); !errors.Is(err, domain.ErrEmptyEventSummary) {
		t.Errorf("err = %v, want ErrEmptyEventSummary", err)
	}
}

func TestUpdateEventScopeAllSaveError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, saveEvtErr: errBoom}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.UpdateEventScope(context.Background(), ScopeAll, editInput("m1", "series-1"), day(4, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestUpdateEventScopeThisCreatesOverride(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}}
	svc := NewCalendarService(store, fixedID("gen"), &fakeRecurrence{})
	in := editInput("m1", "series-1")
	in.Start = day(6, 14)
	in.End = day(6, 15)
	if err := svc.UpdateEventScope(context.Background(), ScopeThis, in, day(6, 9)); err != nil {
		t.Fatalf("UpdateEventScope: %v", err)
	}
	saved := store.savedEvt[0]
	if saved.ID() != "gen" || saved.UID() != "series-1" || !saved.IsOverride() || !saved.RecurrenceID().Equal(day(6, 9)) {
		t.Errorf("override wrong: %+v", saved)
	}
	if saved.Recurrence() != "" {
		t.Errorf("override should carry no recurrence rule, got %q", saved.Recurrence())
	}
}

func TestUpdateEventScopeThisReusesExistingOverride(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	existing := overrideAt(t, "existing", "series-1", day(6, 9), day(6, 14))
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master, existing}}
	svc := NewCalendarService(store, fixedID("gen"), &fakeRecurrence{})
	in := editInput("m1", "series-1")
	in.Start = day(6, 15)
	in.End = day(6, 16)
	if err := svc.UpdateEventScope(context.Background(), ScopeThis, in, day(6, 9)); err != nil {
		t.Fatalf("UpdateEventScope: %v", err)
	}
	if store.savedEvt[0].ID() != "existing" {
		t.Errorf("expected the existing override id reused, got %q", store.savedEvt[0].ID())
	}
}

func TestUpdateEventScopeThisOverridesLookupError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, listEvtErr: errBoom}
	svc := NewCalendarService(store, fixedID("gen"), &fakeRecurrence{})
	if err := svc.UpdateEventScope(context.Background(), ScopeThis, editInput("m1", "series-1"), day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestUpdateEventScopeThisBuildError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}}
	svc := NewCalendarService(store, fixedID("gen"), &fakeRecurrence{})
	bad := editInput("m1", "series-1")
	bad.Summary = "  "
	if err := svc.UpdateEventScope(context.Background(), ScopeThis, bad, day(6, 9)); !errors.Is(err, domain.ErrEmptyEventSummary) {
		t.Errorf("err = %v, want ErrEmptyEventSummary", err)
	}
}

func TestUpdateEventScopeThisSaveError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}, saveEvtErr: errBoom}
	svc := NewCalendarService(store, fixedID("gen"), &fakeRecurrence{})
	if err := svc.UpdateEventScope(context.Background(), ScopeThis, editInput("m1", "series-1"), day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestUpdateEventScopeFutureAtStartEditsAll(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	// Occurrence equal to the master start: nothing precedes it, so this behaves as a whole-series edit.
	if err := svc.UpdateEventScope(context.Background(), ScopeFuture, editInput("m1", "series-1"), day(4, 9)); err != nil {
		t.Fatalf("UpdateEventScope: %v", err)
	}
	if len(store.savedEvt) != 1 || store.savedEvt[0].ID() != "m1" {
		t.Errorf("expected a single whole-series save, got %v saves", len(store.savedEvt))
	}
}

func TestUpdateEventScopeFutureSplits(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	futureOverride := overrideAt(t, "ov-future", "series-1", day(7, 9), day(7, 14))
	pastOverride := overrideAt(t, "ov-past", "series-1", day(5, 9), day(5, 14))
	store := &fakeCalendarStore{
		gotEvent: master,
		events:   []domain.Event{master, futureOverride, pastOverride},
	}
	rec := &fakeRecurrence{truncated: "FREQ=DAILY;UNTIL=20260606T085959Z"}
	svc := NewCalendarService(store, fixedID("new"), rec)
	if err := svc.UpdateEventScope(context.Background(), ScopeFuture, editInput("m1", "series-1"), day(6, 9)); err != nil {
		t.Fatalf("UpdateEventScope: %v", err)
	}
	// Saved: truncated master, new series, and the migrated future override (past override untouched).
	var truncatedMaster, newSeries, migrated bool
	for _, e := range store.savedEvt {
		switch {
		case e.ID() == "m1" && e.Recurrence() == "FREQ=DAILY;UNTIL=20260606T085959Z":
			truncatedMaster = true
		case e.ID() == "new" && e.UID() == "new":
			newSeries = true
		case e.ID() == "ov-future" && e.UID() == "new":
			migrated = true
		}
	}
	if !truncatedMaster || !newSeries || !migrated {
		t.Errorf("split incomplete: truncated=%v newSeries=%v migrated=%v", truncatedMaster, newSeries, migrated)
	}
	if rec.gotTruncate != "FREQ=DAILY;COUNT=5" {
		t.Errorf("truncated the wrong rule: %q", rec.gotTruncate)
	}
}

func TestUpdateEventScopeFutureTruncateError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}}
	svc := NewCalendarService(store, fixedID("new"), &fakeRecurrence{truncateErr: errBoom})
	if err := svc.UpdateEventScope(context.Background(), ScopeFuture, editInput("m1", "series-1"), day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestUpdateEventScopeFutureTruncatedSaveError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}, failSaveID: "m1"}
	svc := NewCalendarService(store, fixedID("new"), &fakeRecurrence{truncated: "FREQ=DAILY"})
	if err := svc.UpdateEventScope(context.Background(), ScopeFuture, editInput("m1", "series-1"), day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestUpdateEventScopeFutureNewSeriesBuildError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}}
	svc := NewCalendarService(store, fixedID("new"), &fakeRecurrence{truncated: "FREQ=DAILY"})
	bad := editInput("m1", "series-1")
	bad.Summary = "  "
	if err := svc.UpdateEventScope(context.Background(), ScopeFuture, bad, day(6, 9)); !errors.Is(err, domain.ErrEmptyEventSummary) {
		t.Errorf("err = %v, want ErrEmptyEventSummary", err)
	}
}

func TestUpdateEventScopeFutureNewSeriesSaveError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}, failSaveID: "new"}
	svc := NewCalendarService(store, fixedID("new"), &fakeRecurrence{truncated: "FREQ=DAILY"})
	if err := svc.UpdateEventScope(context.Background(), ScopeFuture, editInput("m1", "series-1"), day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestUpdateEventScopeFutureMigrateOverridesError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	futureOverride := overrideAt(t, "ov-future", "series-1", day(7, 9), day(7, 14))
	store := &fakeCalendarStore{
		gotEvent:   master,
		events:     []domain.Event{master, futureOverride},
		failSaveID: "ov-future",
	}
	svc := NewCalendarService(store, fixedID("new"), &fakeRecurrence{truncated: "FREQ=DAILY"})
	if err := svc.UpdateEventScope(context.Background(), ScopeFuture, editInput("m1", "series-1"), day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestUpdateEventScopeFutureMigrateLookupError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	// The two split saves succeed; the override lookup inside migration then fails.
	store := &fakeCalendarStore{gotEvent: master, listEvtErr: errBoom}
	svc := NewCalendarService(store, fixedID("new"), &fakeRecurrence{truncated: "FREQ=DAILY"})
	if err := svc.UpdateEventScope(context.Background(), ScopeFuture, editInput("m1", "series-1"), day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestScopeResolvesMasterFromOverrideID(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	override := overrideAt(t, "o1", "series-1", day(6, 9), day(6, 14))
	// The caller passes the override id; the master must still be resolved and the whole series removed.
	store := &fakeCalendarStore{gotEvent: override, events: []domain.Event{master, override}}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), ScopeAll, "o1", day(6, 9)); err != nil {
		t.Fatalf("DeleteEventScope: %v", err)
	}
	if len(store.deletedEvt) != 2 || store.deletedEvt[1] != "m1" {
		t.Errorf("expected the resolved master deleted, got %v", store.deletedEvt)
	}
}

func TestScopeResolveMasterListError(t *testing.T) {
	override := overrideAt(t, "o1", "series-1", day(6, 9), day(6, 14))
	store := &fakeCalendarStore{gotEvent: override, listEvtErr: errBoom}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), ScopeAll, "o1", day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestScopeResolveMasterFallsBackToEvent(t *testing.T) {
	// A non-recurring event with no master in its series resolves to itself.
	single := singleEvent(t, "s1", day(9, 9), day(9, 10))
	store := &fakeCalendarStore{gotEvent: single, events: []domain.Event{single}}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), ScopeAll, "s1", day(9, 9)); err != nil {
		t.Fatalf("DeleteEventScope: %v", err)
	}
	if len(store.deletedEvt) != 1 || store.deletedEvt[0] != "s1" {
		t.Errorf("expected the event itself deleted, got %v", store.deletedEvt)
	}
}

func TestDeleteEventScopeLoadError(t *testing.T) {
	svc := NewCalendarService(&fakeCalendarStore{getEvtErr: errBoom}, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), ScopeAll, "m1", day(4, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestDeleteEventScopeUnknown(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), EventScope(99), "m1", day(4, 9)); err == nil {
		t.Errorf("expected an error for an unknown scope")
	}
}

func TestDeleteEventScopeAll(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	override := overrideAt(t, "o1", "series-1", day(6, 9), day(6, 14))
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master, override}}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), ScopeAll, "m1", day(4, 9)); err != nil {
		t.Fatalf("DeleteEventScope: %v", err)
	}
	if len(store.deletedEvt) != 2 || store.deletedEvt[0] != "o1" || store.deletedEvt[1] != "m1" {
		t.Errorf("expected the override then the master deleted, got %v", store.deletedEvt)
	}
}

func TestDeleteEventScopeAllOverridesLookupError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, listEvtErr: errBoom}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), ScopeAll, "m1", day(4, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestDeleteEventScopeAllOverrideDeleteError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	override := overrideAt(t, "o1", "series-1", day(6, 9), day(6, 14))
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master, override}, failDelID: "o1"}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), ScopeAll, "m1", day(4, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestDeleteEventScopeAllMasterDeleteError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}, failDelID: "m1"}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), ScopeAll, "m1", day(4, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestDeleteEventScopeThisExcludesOccurrence(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	override := overrideAt(t, "o1", "series-1", day(6, 9), day(6, 14))
	other := overrideAt(t, "o2", "series-1", day(7, 9), day(7, 14))
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master, override, other}}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), ScopeThis, "m1", day(6, 9)); err != nil {
		t.Fatalf("DeleteEventScope: %v", err)
	}
	if len(store.deletedEvt) != 1 || store.deletedEvt[0] != "o1" {
		t.Errorf("expected only the matching override deleted, got %v", store.deletedEvt)
	}
	saved := store.savedEvt[0]
	if saved.ID() != "m1" {
		t.Fatalf("expected the master re-saved, got %q", saved.ID())
	}
	var excluded bool
	for _, ex := range saved.ExDates() {
		if ex.Equal(day(6, 9)) {
			excluded = true
		}
	}
	if !excluded {
		t.Errorf("occurrence not added to EXDATE: %v", saved.ExDates())
	}
}

func TestDeleteEventScopeThisOverrideLookupError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, listEvtErr: errBoom}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), ScopeThis, "m1", day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestDeleteEventScopeThisOverrideDeleteError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	override := overrideAt(t, "o1", "series-1", day(6, 9), day(6, 14))
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master, override}, failDelID: "o1"}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), ScopeThis, "m1", day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestDeleteEventScopeThisSaveError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}, failSaveID: "m1"}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), ScopeThis, "m1", day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestDeleteEventScopeFutureAtStartDeletesWholeSeries(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{})
	if err := svc.DeleteEventScope(context.Background(), ScopeFuture, "m1", day(4, 9)); err != nil {
		t.Fatalf("DeleteEventScope: %v", err)
	}
	if len(store.deletedEvt) != 1 || store.deletedEvt[0] != "m1" {
		t.Errorf("expected the whole series deleted, got %v", store.deletedEvt)
	}
}

func TestDeleteEventScopeFutureTruncatesAndDropsLaterOverrides(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	future := overrideAt(t, "ov-future", "series-1", day(7, 9), day(7, 14))
	past := overrideAt(t, "ov-past", "series-1", day(5, 9), day(5, 14))
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master, future, past}}
	rec := &fakeRecurrence{truncated: "FREQ=DAILY;UNTIL=20260606T085959Z"}
	svc := NewCalendarService(store, fixedID("x"), rec)
	if err := svc.DeleteEventScope(context.Background(), ScopeFuture, "m1", day(6, 9)); err != nil {
		t.Fatalf("DeleteEventScope: %v", err)
	}
	if len(store.deletedEvt) != 1 || store.deletedEvt[0] != "ov-future" {
		t.Errorf("expected only the future override deleted, got %v", store.deletedEvt)
	}
	if len(store.savedEvt) != 1 || store.savedEvt[0].Recurrence() != "FREQ=DAILY;UNTIL=20260606T085959Z" {
		t.Errorf("expected the master truncated, got %+v", store.savedEvt)
	}
}

func TestDeleteEventScopeFutureTruncateError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{truncateErr: errBoom})
	if err := svc.DeleteEventScope(context.Background(), ScopeFuture, "m1", day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestDeleteEventScopeFutureOverrideLookupError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, listEvtErr: errBoom}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{truncated: "FREQ=DAILY"})
	if err := svc.DeleteEventScope(context.Background(), ScopeFuture, "m1", day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestDeleteEventScopeFutureOverrideDeleteError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	future := overrideAt(t, "ov-future", "series-1", day(7, 9), day(7, 14))
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master, future}, failDelID: "ov-future"}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{truncated: "FREQ=DAILY"})
	if err := svc.DeleteEventScope(context.Background(), ScopeFuture, "m1", day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestDeleteEventScopeFutureTruncatedSaveError(t *testing.T) {
	master := recurringMaster(t, "m1", "series-1")
	store := &fakeCalendarStore{gotEvent: master, events: []domain.Event{master}, failSaveID: "m1"}
	svc := NewCalendarService(store, fixedID("x"), &fakeRecurrence{truncated: "FREQ=DAILY"})
	if err := svc.DeleteEventScope(context.Background(), ScopeFuture, "m1", day(6, 9)); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}
