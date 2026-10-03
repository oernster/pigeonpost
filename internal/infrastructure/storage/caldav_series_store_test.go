package storage

import (
	"context"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// A recurring series and its override share a UID by definition. Through the CalDAV services and the real
// store they must keep a row each, so the override never overwrites the master and the object pushed back
// to the server is the whole series.

// seriesSource is a CalDAVSource holding one collection with one object.
type seriesSource struct{}

func (seriesSource) ListCalendars(context.Context) ([]application.RemoteCalendar, error) {
	return []application.RemoteCalendar{{Path: "/cal/", DisplayName: "Work"}}, nil
}

func (seriesSource) ListObjects(context.Context, application.RemoteCalendar) ([]application.RemoteObject, error) {
	return []application.RemoteObject{{Href: "/cal/series.ics", ETag: "e1", Data: []byte("series")}}, nil
}

func (seriesSource) CollectionCTag(context.Context, string) (string, error) { return "", nil }

// seriesCodec decodes every body to a weekly master and one moved occurrence, both keyed by the bare UID
// as the ICS codec keyed them before overrides had an id of their own.
type seriesCodec struct{ events []domain.Event }

func (c seriesCodec) Decode([]byte) ([]domain.Event, []domain.CalendarPassthrough, error) {
	return c.events, nil, nil
}

func (seriesCodec) Encode([]domain.Event, []domain.CalendarPassthrough) ([]byte, error) {
	return nil, nil
}

func newSeriesCodec(t *testing.T) seriesCodec {
	t.Helper()
	start := baseStart()
	occurrence := start.Add(7 * 24 * time.Hour)
	master, err := domain.NewEvent(domain.EventInput{
		ID: "uid-s", UID: "uid-s", Summary: "Weekly", Start: start, Recurrence: "FREQ=WEEKLY",
	})
	if err != nil {
		t.Fatalf("master: %v", err)
	}
	override, err := domain.NewEvent(domain.EventInput{
		ID: "uid-s", UID: "uid-s", Summary: "Weekly (moved)", Start: occurrence.Add(time.Hour), RecurrenceID: occurrence,
	})
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	return seriesCodec{events: []domain.Event{master, override}}
}

func assertSeriesKept(t *testing.T, store *Store) {
	t.Helper()
	events, err := store.EventsByHref(context.Background(), "/cal/series.ics")
	if err != nil {
		t.Fatalf("EventsByHref: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("stored %d events for the object, want the master and its override", len(events))
	}
	if events[0].IsOverride() || events[0].ID() != "uid-s" || events[0].Recurrence() == "" {
		t.Errorf("first row = %q (override %v), want the recurring master under the bare UID", events[0].ID(), events[0].IsOverride())
	}
	if !events[1].IsOverride() || events[1].ID() != domain.EventIDFor("uid-s", events[1].RecurrenceID()) {
		t.Errorf("second row = %q, want the override under its own id", events[1].ID())
	}
}

func TestCalDAVPullKeepsASeriesAndItsOverrideInTheStore(t *testing.T) {
	store := openTestStore(t)
	saved, err := application.NewCalDAVSyncService(seriesSource{}, newSeriesCodec(t), store, "acc1").Pull(context.Background())
	if err != nil || saved != 2 {
		t.Fatalf("Pull = (%d, %v), want (2, nil)", saved, err)
	}
	assertSeriesKept(t, store)
}

func TestCalDAVReconcileKeepsASeriesAndItsOverrideInTheStore(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	codec := newSeriesCodec(t)
	records, err := application.NewCalDAVSyncService(seriesSource{}, codec, store, "acc1").Discover(ctx)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if err := application.NewCalDAVReconcileService(store, codec, func() string { return "copy" }).Reconcile(ctx, seriesSource{}, records); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertSeriesKept(t, store)
}
