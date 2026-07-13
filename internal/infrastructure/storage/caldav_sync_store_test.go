package storage

import (
	"context"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// The store must satisfy the application CalendarSyncStore port used by two-way CalDAV sync.
var _ application.CalendarSyncStore = (*Store)(nil)

func syncedEvent(t *testing.T, id, uid string, start time.Time, recurrenceID time.Time) domain.Event {
	t.Helper()
	ev, err := domain.NewEvent(domain.EventInput{
		ID: id, UID: uid, CalendarID: "cal1", Summary: "S", Start: start, RecurrenceID: recurrenceID,
	})
	if err != nil {
		t.Fatalf("event %q: %v", id, err)
	}
	return ev
}

func TestSaveSyncedEventCarriesHrefAndEtag(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	const href, etag = "https://dav.example.com/cal1/obj1.ics", "etag-1"
	start := baseStart()
	master := syncedEvent(t, "obj1", "uid-1", start, time.Time{})
	override := syncedEvent(t, "obj1#1", "uid-1", start.Add(24*time.Hour), start.Add(24*time.Hour))
	if err := store.SaveSyncedEvent(ctx, master, href, etag); err != nil {
		t.Fatalf("SaveSyncedEvent master: %v", err)
	}
	if err := store.SaveSyncedEvent(ctx, override, href, etag); err != nil {
		t.Fatalf("SaveSyncedEvent override: %v", err)
	}
	// A local-only event (no href) in the same calendar must not appear as a synced object.
	local := syncedEvent(t, "local1", "uid-local", start, time.Time{})
	if err := store.SaveEvent(ctx, local); err != nil {
		t.Fatalf("SaveEvent local: %v", err)
	}

	objs, err := store.ListSyncedObjects(ctx, "cal1")
	if err != nil {
		t.Fatalf("ListSyncedObjects: %v", err)
	}
	if len(objs) != 1 || objs[0].Href != href || objs[0].ETag != etag {
		t.Fatalf("synced objects = %+v, want one {%s,%s}", objs, href, etag)
	}

	events, err := store.EventsByHref(ctx, href)
	if err != nil {
		t.Fatalf("EventsByHref: %v", err)
	}
	if len(events) != 2 || events[0].ID() != "obj1" || events[1].ID() != "obj1#1" {
		t.Fatalf("events by href = %+v, want master then override", events)
	}

	if err := store.DeleteEventsByHref(ctx, href); err != nil {
		t.Fatalf("DeleteEventsByHref: %v", err)
	}
	events, err = store.EventsByHref(ctx, href)
	if err != nil {
		t.Fatalf("EventsByHref after delete: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events remain after delete: %+v", events)
	}
	// The local-only event survives an object delete: DeleteEventsByHref only removes rows of that object.
	if _, err := store.GetEvent(ctx, "local1"); err != nil {
		t.Fatalf("local event removed by DeleteEventsByHref: %v", err)
	}
}

func TestRemoteCalendarAndCTag(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	remote, err := domain.NewCalendar("cal1", "Work", "#3b82f6")
	if err != nil {
		t.Fatalf("calendar: %v", err)
	}
	if err := store.SaveRemoteCalendar(ctx, remote, "acc1", "https://dav.example.com/cal1/", "ctag-1"); err != nil {
		t.Fatalf("SaveRemoteCalendar: %v", err)
	}
	// A purely local calendar (no account) must not be listed for the account.
	local, _ := domain.NewCalendar("local", "Local", "#888888")
	if err := store.SaveCalendar(ctx, local); err != nil {
		t.Fatalf("SaveCalendar: %v", err)
	}

	records, err := store.ListRemoteCalendars(ctx, "acc1")
	if err != nil {
		t.Fatalf("ListRemoteCalendars: %v", err)
	}
	want := application.RemoteCalendarRecord{
		CalendarID: "cal1", AccountID: "acc1", Href: "https://dav.example.com/cal1/", CTag: "ctag-1", Name: "Work",
	}
	if len(records) != 1 || records[0] != want {
		t.Fatalf("remote calendars = %+v, want [%+v]", records, want)
	}

	ctag, err := store.CalendarCTag(ctx, "cal1")
	if err != nil || ctag != "ctag-1" {
		t.Fatalf("CalendarCTag(cal1) = %q, %v; want ctag-1", ctag, err)
	}
	// An unknown calendar has no CTag rather than an error, so a first sync treats it as changed.
	empty, err := store.CalendarCTag(ctx, "missing")
	if err != nil || empty != "" {
		t.Fatalf("CalendarCTag(missing) = %q, %v; want empty", empty, err)
	}
	// An updated CTag overwrites the stored one.
	if err := store.SaveRemoteCalendar(ctx, remote, "acc1", "https://dav.example.com/cal1/", "ctag-2"); err != nil {
		t.Fatalf("SaveRemoteCalendar update: %v", err)
	}
	if ctag, _ := store.CalendarCTag(ctx, "cal1"); ctag != "ctag-2" {
		t.Fatalf("CTag not updated: %q", ctag)
	}
}

func TestPendingCalendarOps(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	const href1, href2 = "https://dav.example.com/cal1/a.ics", "https://dav.example.com/cal1/b.ics"
	create := application.PendingCalendarObject{CalendarID: "cal1", Href: href1, Op: application.CalendarOpCreate}
	update := application.PendingCalendarObject{CalendarID: "cal1", Href: href2, Op: application.CalendarOpUpdate, BaseETag: "e2"}
	if err := store.SetPendingCalendarOp(ctx, create); err != nil {
		t.Fatalf("SetPendingCalendarOp create: %v", err)
	}
	if err := store.SetPendingCalendarOp(ctx, update); err != nil {
		t.Fatalf("SetPendingCalendarOp update: %v", err)
	}

	ops, err := store.ListPendingCalendarOps(ctx)
	if err != nil {
		t.Fatalf("ListPendingCalendarOps: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("pending ops = %+v, want 2", ops)
	}

	// Setting the same object again replaces the intent rather than adding a row: a queued create that is
	// then locally deleted becomes a delete for the same (calendar, href), still one row.
	del := application.PendingCalendarObject{CalendarID: "cal1", Href: href1, Op: application.CalendarOpDelete, BaseETag: "e1"}
	if err := store.SetPendingCalendarOp(ctx, del); err != nil {
		t.Fatalf("SetPendingCalendarOp replace: %v", err)
	}
	ops, _ = store.ListPendingCalendarOps(ctx)
	if len(ops) != 2 {
		t.Fatalf("replace added a row: %+v", ops)
	}
	var got application.PendingCalendarObject
	for _, o := range ops {
		if o.Href == href1 {
			got = o
		}
	}
	if got != del {
		t.Fatalf("replaced op = %+v, want %+v", got, del)
	}

	if err := store.ClearPendingCalendarOp(ctx, "cal1", href1); err != nil {
		t.Fatalf("ClearPendingCalendarOp: %v", err)
	}
	ops, _ = store.ListPendingCalendarOps(ctx)
	if len(ops) != 1 || ops[0].Href != href2 {
		t.Fatalf("after clear = %+v, want only %s", ops, href2)
	}
}
