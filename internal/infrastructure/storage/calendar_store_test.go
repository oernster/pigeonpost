package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// The store must satisfy the application CalendarStore port.
var _ application.CalendarStore = (*Store)(nil)

func baseStart() time.Time { return time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC) }

func TestCalendarRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	cal, err := domain.NewCalendar("cal1", "Work", "#ff8800")
	if err != nil {
		t.Fatalf("calendar: %v", err)
	}
	if err := store.SaveCalendar(ctx, cal); err != nil {
		t.Fatalf("SaveCalendar: %v", err)
	}
	got, err := store.ListCalendars(ctx)
	if err != nil {
		t.Fatalf("ListCalendars: %v", err)
	}
	if len(got) != 1 || got[0].Name() != "Work" || got[0].Colour() != "#ff8800" {
		t.Errorf("calendar not persisted: %+v", got)
	}
}

func TestEventRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	start := baseStart()
	end := start.Add(time.Hour)
	ev, err := domain.NewEvent(domain.EventInput{
		ID: "e1", UID: "uid-1", CalendarID: "cal1", Summary: "Standup", Description: "daily",
		Location: "Room 1", Start: start, End: end, Recurrence: "FREQ=DAILY",
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if err := store.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	got, err := store.GetEvent(ctx, "e1")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.UID() != "uid-1" || got.CalendarID() != "cal1" || got.Summary() != "Standup" ||
		got.Description() != "daily" || got.Location() != "Room 1" || got.Recurrence() != "FREQ=DAILY" {
		t.Errorf("fields not persisted: %+v", got)
	}
	if !got.Start().Equal(start) || !got.End().Equal(end) || !got.HasEnd() {
		t.Errorf("times not persisted: start=%v end=%v", got.Start(), got.End())
	}
}

func TestEventNoEndRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	ev, _ := domain.NewEvent(domain.EventInput{ID: "d1", Summary: "Holiday", Start: baseStart(), AllDay: true})
	if err := store.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	got, err := store.GetEvent(ctx, "d1")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.HasEnd() || !got.AllDay() {
		t.Errorf("expected all-day with no end, got hasEnd=%v allDay=%v", got.HasEnd(), got.AllDay())
	}
}

func TestListEventsOrderedByStart(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	later, _ := domain.NewEvent(domain.EventInput{ID: "e2", Summary: "Later", Start: baseStart().Add(2 * time.Hour)})
	earlier, _ := domain.NewEvent(domain.EventInput{ID: "e1", Summary: "Earlier", Start: baseStart()})
	if err := store.SaveEvent(ctx, later); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.SaveEvent(ctx, earlier); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := store.ListEvents(ctx)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(got) != 2 || got[0].Summary() != "Earlier" || got[1].Summary() != "Later" {
		t.Errorf("events not ordered by start: %v", []string{got[0].Summary(), got[1].Summary()})
	}
}

func TestDeleteCalendarCascadesEvents(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	cal, _ := domain.NewCalendar("cal1", "Work", "")
	if err := store.SaveCalendar(ctx, cal); err != nil {
		t.Fatalf("save calendar: %v", err)
	}
	ev, _ := domain.NewEvent(domain.EventInput{ID: "e1", CalendarID: "cal1", Summary: "Standup", Start: baseStart()})
	if err := store.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("save event: %v", err)
	}
	if err := store.DeleteCalendar(ctx, "cal1"); err != nil {
		t.Fatalf("DeleteCalendar: %v", err)
	}
	cals, err := store.ListCalendars(ctx)
	if err != nil {
		t.Fatalf("list calendars: %v", err)
	}
	events, err := store.ListEvents(ctx)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(cals) != 0 || len(events) != 0 {
		t.Errorf("expected calendar and its events removed, got %d cals, %d events", len(cals), len(events))
	}
}

func TestEventRecurrenceSetRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	start := baseStart()
	rd := start.Add(48 * time.Hour)
	ed := start.Add(24 * time.Hour)
	ev, err := domain.NewEvent(domain.EventInput{
		ID: "e1", UID: "uid-1", Summary: "Standup", Start: start, End: start.Add(time.Hour),
		Recurrence: "FREQ=DAILY;COUNT=5", RDates: []time.Time{rd}, ExDates: []time.Time{ed},
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if err := store.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	got, err := store.GetEvent(ctx, "e1")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if gotRD := got.RDates(); len(gotRD) != 1 || !gotRD[0].Equal(rd) {
		t.Errorf("RDates not persisted: %v", got.RDates())
	}
	if ex := got.ExDates(); len(ex) != 1 || !ex[0].Equal(ed) {
		t.Errorf("ExDates not persisted: %v", got.ExDates())
	}
	if got.IsOverride() {
		t.Errorf("non-override event came back as an override")
	}
}

func TestEventTimeZoneRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	ev, err := domain.NewEvent(domain.EventInput{
		ID: "e1", Summary: "Standup", Start: baseStart(), End: baseStart().Add(time.Hour),
		Recurrence: "FREQ=DAILY", TimeZone: "Europe/London",
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if err := store.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	got, err := store.GetEvent(ctx, "e1")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.TimeZone() != "Europe/London" {
		t.Errorf("time zone not persisted: %q", got.TimeZone())
	}
}

func TestEventOverrideRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	start := baseStart()
	ev, err := domain.NewEvent(domain.EventInput{
		ID: "e1-override", UID: "uid-1", Summary: "Standup moved", Start: start.Add(2 * time.Hour),
		End: start.Add(3 * time.Hour), RecurrenceID: start,
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if err := store.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	got, err := store.GetEvent(ctx, "e1-override")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if !got.IsOverride() || !got.RecurrenceID().Equal(start) {
		t.Errorf("override not persisted: isOverride=%v id=%v", got.IsOverride(), got.RecurrenceID())
	}
}

func TestEventAlarmsRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	ev, err := domain.NewEvent(domain.EventInput{
		ID: "e1", Summary: "Standup", Start: baseStart(),
		Alarms: []domain.Alarm{domain.NewAlarm(-15 * time.Minute), domain.NewAlarm(-24 * time.Hour)},
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if err := store.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	got, err := store.GetEvent(ctx, "e1")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	alarms := got.Alarms()
	if len(alarms) != 2 || alarms[0].Offset() != -15*time.Minute || alarms[1].Offset() != -24*time.Hour {
		t.Errorf("alarms not persisted: %v", alarms)
	}
}

func TestEncodeDecodeAlarms(t *testing.T) {
	if s := encodeAlarms(nil); s != "" {
		t.Errorf("empty encode = %q, want empty", s)
	}
	got, err := decodeAlarms(encodeAlarms([]domain.Alarm{domain.NewAlarm(-15 * time.Minute)}))
	if err != nil || len(got) != 1 || got[0].Offset() != -15*time.Minute {
		t.Errorf("round trip = %v, %v", got, err)
	}
	if _, err := decodeAlarms("not-a-number"); err == nil {
		t.Errorf("expected an error decoding a non-numeric alarm")
	}
}

func TestEncodeDecodeTimesRoundTrip(t *testing.T) {
	if s := encodeTimes(nil); s != "" {
		t.Errorf("empty encode = %q, want empty", s)
	}
	if got, err := decodeTimes(""); err != nil || got != nil {
		t.Errorf("empty decode = %v, %v, want nil, nil", got, err)
	}
	times := []time.Time{baseStart(), baseStart().Add(time.Hour)}
	got, err := decodeTimes(encodeTimes(times))
	if err != nil {
		t.Fatalf("decodeTimes: %v", err)
	}
	if len(got) != 2 || !got[0].Equal(times[0]) || !got[1].Equal(times[1]) {
		t.Errorf("round trip mismatch: %v", got)
	}
	if _, err := decodeTimes("not-a-number"); err == nil {
		t.Errorf("expected an error decoding a non-numeric value")
	}
}

func TestDeleteEvent(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	ev, _ := domain.NewEvent(domain.EventInput{ID: "e1", Summary: "Standup", Start: baseStart()})
	if err := store.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.DeleteEvent(ctx, "e1"); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if _, err := store.GetEvent(ctx, "e1"); err == nil {
		t.Errorf("expected an error getting a deleted event")
	}
}

func TestPassthroughRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	p, err := domain.NewCalendarPassthrough("todo-1", domain.PassthroughToDo,
		"BEGIN:VCALENDAR\r\nBEGIN:VTODO\r\nUID:todo-1\r\nEND:VTODO\r\nEND:VCALENDAR\r\n")
	if err != nil {
		t.Fatalf("passthrough: %v", err)
	}
	if err := store.SavePassthrough(ctx, p); err != nil {
		t.Fatalf("SavePassthrough: %v", err)
	}
	// A re-save under the same UID replaces rather than duplicates.
	replaced, _ := domain.NewCalendarPassthrough("todo-1", domain.PassthroughToDo,
		"BEGIN:VCALENDAR\r\nBEGIN:VTODO\r\nUID:todo-1\r\nSUMMARY:Updated\r\nEND:VTODO\r\nEND:VCALENDAR\r\n")
	if err := store.SavePassthrough(ctx, replaced); err != nil {
		t.Fatalf("SavePassthrough replace: %v", err)
	}
	got, err := store.ListPassthrough(ctx)
	if err != nil {
		t.Fatalf("ListPassthrough: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d passthrough, want 1 (replaced by UID)", len(got))
	}
	if got[0].UID() != "todo-1" || got[0].Kind() != domain.PassthroughToDo {
		t.Errorf("passthrough = %+v", got[0])
	}
	if !strings.Contains(got[0].Raw(), "SUMMARY:Updated") {
		t.Errorf("raw not replaced: %q", got[0].Raw())
	}
}
