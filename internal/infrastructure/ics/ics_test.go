package ics

import (
	"strings"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// The codec must satisfy the application CalendarCodec port.
var _ application.CalendarCodec = Codec{}

func cal(lines ...string) []byte { return []byte(strings.Join(lines, "\r\n") + "\r\n") }

func TestICSRoundTrip(t *testing.T) {
	start := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	ev, err := domain.NewEvent(domain.EventInput{
		ID: "uid-1", UID: "uid-1", Summary: "Standup", Description: "daily sync", Location: "Room 1",
		Start: start, End: end, Recurrence: "FREQ=DAILY;COUNT=5",
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	data, err := New().Encode([]domain.Event{ev}, nil)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d events, want 1", len(got))
	}
	e := got[0]
	if e.ID() != "uid-1" || e.Summary() != "Standup" || e.Description() != "daily sync" || e.Location() != "Room 1" {
		t.Errorf("fields not preserved: %+v", e)
	}
	if !e.Start().Equal(start) || !e.End().Equal(end) {
		t.Errorf("times not preserved: start=%v end=%v", e.Start(), e.End())
	}
	if e.Recurrence() != "FREQ=DAILY;COUNT=5" {
		t.Errorf("recurrence = %q", e.Recurrence())
	}
}

func TestICSAllDayRoundTrip(t *testing.T) {
	day := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	ev, err := domain.NewEvent(domain.EventInput{ID: "d1", UID: "d1", Summary: "Holiday", Start: day, AllDay: true})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	data, err := New().Encode([]domain.Event{ev}, nil)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 || !got[0].AllDay() || got[0].HasEnd() {
		t.Fatalf("all-day not preserved: %+v", got)
	}
	if !got[0].Start().Equal(day) {
		t.Errorf("all-day start = %v, want %v", got[0].Start(), day)
	}
}

func TestICSPreservesUnmodelledProperties(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Mozilla.org//NONSGML Thunderbird//EN",
		"BEGIN:VEVENT", "UID:keep-1", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T100000Z",
		"DTEND:20260704T110000Z", "SUMMARY:Review",
		"CATEGORIES:WORK", "STATUS:CONFIRMED", "PRIORITY:1", "X-CUSTOM-FLAG:keepme",
		"BEGIN:VALARM", "ACTION:DISPLAY", "TRIGGER:-PT15M", "DESCRIPTION:Reminder", "END:VALARM",
		"END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("decoded %d, want 1", len(events))
	}
	out, err := New().Encode(events, nil)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	s := string(out)
	// VALARM is now a modelled property, re-emitted from the alarm model, so it is covered by the alarm
	// round-trip test rather than asserted verbatim here. CATEGORIES is likewise modelled now: its primary
	// value is re-emitted normalised (lowercased), so it is asserted in its normalised form here and
	// covered fully by the dedicated category tests.
	for _, want := range []string{"CATEGORIES:work", "STATUS:CONFIRMED", "PRIORITY:1", "X-CUSTOM-FLAG:keepme"} {
		if !strings.Contains(s, want) {
			t.Errorf("re-encoded ICS dropped %q:\n%s", want, s)
		}
	}
	if !strings.Contains(s, "SUMMARY:Review") || !strings.Contains(s, "UID:keep-1") {
		t.Errorf("modelled fields missing after round-trip:\n%s", s)
	}
}

func TestICSEditPreservesUnmodelledProperties(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:e9", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T100000Z",
		"SUMMARY:Old title", "CATEGORIES:PERSONAL", "X-KEEP:yes",
		"END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	orig := events[0]
	// Simulate an in-app edit: same event, a new summary, its modelled category carried through (the form
	// loads and re-saves it), and Extra carried through unchanged.
	edited, err := domain.NewEvent(domain.EventInput{
		ID: orig.ID(), UID: orig.UID(), Summary: "New title", Start: orig.Start(),
		Category: orig.Category(), Extra: orig.Extra(),
	})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	s := string(mustEncode(t, edited))
	if !strings.Contains(s, "SUMMARY:New title") {
		t.Errorf("edit not applied:\n%s", s)
	}
	if strings.Contains(s, "Old title") {
		t.Errorf("old summary should have been overwritten:\n%s", s)
	}
	// X-KEEP is unmodelled and survives verbatim; CATEGORIES is modelled now, re-emitted normalised.
	if !strings.Contains(s, "X-KEEP:yes") || !strings.Contains(s, "CATEGORIES:personal") {
		t.Errorf("props dropped on edit:\n%s", s)
	}
}

func TestICSRecurrenceDatesRoundTrip(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:r1", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T090000Z",
		"DTEND:20260704T100000Z", "SUMMARY:Standup", "RRULE:FREQ=DAILY;COUNT=5",
		"RDATE:20260710T090000Z,20260712T090000Z", "EXDATE:20260705T090000Z",
		"END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	e := events[0]
	if got := e.RDates(); len(got) != 2 ||
		!got[0].Equal(time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)) ||
		!got[1].Equal(time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("RDATE not parsed: %v", e.RDates())
	}
	if got := e.ExDates(); len(got) != 1 || !got[0].Equal(time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("EXDATE not parsed: %v", e.ExDates())
	}
	// Re-encoding reproduces the modelled recurrence dates.
	s := string(mustEncode(t, e))
	if !strings.Contains(s, "20260710T090000Z") || !strings.Contains(s, "20260712T090000Z") {
		t.Errorf("RDATE dropped on re-encode:\n%s", s)
	}
	if !strings.Contains(s, "EXDATE") || !strings.Contains(s, "20260705T090000Z") {
		t.Errorf("EXDATE dropped on re-encode:\n%s", s)
	}
}

func TestICSRecurrenceIDRoundTrip(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:series-1", "DTSTAMP:20260704T090000Z", "DTSTART:20260706T110000Z",
		"DTEND:20260706T120000Z", "SUMMARY:Standup moved", "RECURRENCE-ID:20260706T090000Z",
		"END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	e := events[0]
	if !e.IsOverride() || !e.RecurrenceID().Equal(time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("RECURRENCE-ID not parsed: isOverride=%v id=%v", e.IsOverride(), e.RecurrenceID())
	}
	s := string(mustEncode(t, e))
	if !strings.Contains(s, "RECURRENCE-ID") || !strings.Contains(s, "20260706T090000Z") {
		t.Errorf("RECURRENCE-ID dropped on re-encode:\n%s", s)
	}
}

func TestICSAllDayExceptionDateRoundTrip(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:h1", "DTSTAMP:20260704T090000Z", "DTSTART;VALUE=DATE:20260704",
		"SUMMARY:Daily standup", "RRULE:FREQ=DAILY;COUNT=3", "EXDATE;VALUE=DATE:20260705",
		"END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	e := events[0]
	if !e.AllDay() {
		t.Fatalf("expected all-day event")
	}
	if got := e.ExDates(); len(got) != 1 || !got[0].Equal(time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("all-day EXDATE not parsed: %v", e.ExDates())
	}
	s := string(mustEncode(t, e))
	if !strings.Contains(s, "EXDATE;VALUE=DATE:20260705") {
		t.Errorf("all-day EXDATE not re-encoded as a DATE:\n%s", s)
	}
}

func TestICSTimeZoneRoundTrip(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:z1", "DTSTAMP:20260704T090000Z",
		"DTSTART;TZID=Europe/London:20260705T090000", "DTEND;TZID=Europe/London:20260705T100000",
		"SUMMARY:Standup", "RRULE:FREQ=DAILY", "END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	e := events[0]
	if e.TimeZone() != "Europe/London" {
		t.Errorf("zone not parsed: %q", e.TimeZone())
	}
	// 09:00 London on 5 July 2026 is 08:00 UTC (BST).
	if !e.Start().Equal(time.Date(2026, 7, 5, 8, 0, 0, 0, time.UTC)) {
		t.Errorf("zoned start instant = %v, want 08:00 UTC", e.Start().UTC())
	}
	// Re-encoding keeps the TZID and the local wall time (not a UTC Z value).
	s := string(mustEncode(t, e))
	if !strings.Contains(s, "DTSTART;TZID=Europe/London:20260705T090000") {
		t.Errorf("TZID start not re-encoded:\n%s", s)
	}
	if !strings.Contains(s, "DTEND;TZID=Europe/London:20260705T100000") {
		t.Errorf("TZID end not re-encoded:\n%s", s)
	}
}

func TestICSUTCEventHasNoZone(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:u1", "DTSTAMP:20260704T090000Z", "DTSTART:20260705T090000Z",
		"SUMMARY:UTC event", "END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	if events[0].TimeZone() != "" {
		t.Errorf("UTC event should carry no zone, got %q", events[0].TimeZone())
	}
	if s := string(mustEncode(t, events[0])); !strings.Contains(s, "DTSTART:20260705T090000Z") {
		t.Errorf("UTC event should re-encode as a Z value:\n%s", s)
	}
}

func TestICSAlarmRoundTrip(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:a1", "DTSTAMP:20260704T090000Z", "DTSTART:20260705T090000Z", "SUMMARY:Standup",
		"BEGIN:VALARM", "ACTION:DISPLAY", "TRIGGER:-PT15M", "DESCRIPTION:Reminder", "END:VALARM",
		"END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	alarms := events[0].Alarms()
	if len(alarms) != 1 || alarms[0].Offset() != -15*time.Minute {
		t.Errorf("alarm not parsed: %v", alarms)
	}
	// Re-encoding writes exactly one VALARM with the same trigger, not a duplicate.
	s := string(mustEncode(t, events[0]))
	if strings.Count(s, "BEGIN:VALARM") != 1 {
		t.Errorf("expected one VALARM, got %d:\n%s", strings.Count(s, "BEGIN:VALARM"), s)
	}
	if !strings.Contains(s, "TRIGGER:-PT15M") {
		t.Errorf("alarm trigger not re-encoded:\n%s", s)
	}
}

func TestICSPreservesExoticAlarms(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:a2", "DTSTAMP:20260704T090000Z", "DTSTART:20260705T090000Z", "SUMMARY:Standup",
		"BEGIN:VALARM", "ACTION:DISPLAY", "TRIGGER:-PT15M", "DESCRIPTION:Reminder", "END:VALARM",
		"BEGIN:VALARM", "ACTION:EMAIL", "TRIGGER:-PT30M", "DESCRIPTION:Mail me", "SUMMARY:Subj",
		"ATTENDEE:mailto:a@b.example", "END:VALARM",
		"BEGIN:VALARM", "ACTION:AUDIO", "TRIGGER;VALUE=DATE-TIME:20260705T080000Z", "END:VALARM",
		"END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	// Only the DISPLAY relative alarm surfaces as an editable reminder.
	if got := events[0].Alarms(); len(got) != 1 || got[0].Offset() != -15*time.Minute {
		t.Fatalf("modelled alarms = %v, want one -PT15M", got)
	}
	s := string(mustEncode(t, events[0]))
	if !strings.Contains(s, "TRIGGER:-PT15M") {
		t.Errorf("DISPLAY reminder not re-encoded:\n%s", s)
	}
	if !strings.Contains(s, "ACTION:EMAIL") {
		t.Errorf("EMAIL alarm not preserved:\n%s", s)
	}
	if !strings.Contains(s, "ACTION:AUDIO") {
		t.Errorf("AUDIO alarm not preserved:\n%s", s)
	}
	if n := strings.Count(s, "BEGIN:VALARM"); n != 3 {
		t.Errorf("expected three VALARMs, got %d:\n%s", n, s)
	}
}

func TestICSEditKeepsExoticAlarm(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:a3", "DTSTAMP:20260704T090000Z", "DTSTART:20260705T090000Z", "SUMMARY:Standup",
		"BEGIN:VALARM", "ACTION:DISPLAY", "TRIGGER:-PT15M", "DESCRIPTION:Reminder", "END:VALARM",
		"BEGIN:VALARM", "ACTION:AUDIO", "TRIGGER;VALUE=DATE-TIME:20260705T080000Z", "END:VALARM",
		"END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	// The user changes the on-screen reminder from 15 to 5 minutes.
	edited := events[0].WithAlarms([]domain.Alarm{domain.NewAlarm(-5 * time.Minute)})
	s := string(mustEncode(t, edited))
	if !strings.Contains(s, "TRIGGER:-PT5M") {
		t.Errorf("edited reminder not written:\n%s", s)
	}
	if strings.Contains(s, "TRIGGER:-PT15M") {
		t.Errorf("old reminder should have been replaced:\n%s", s)
	}
	if !strings.Contains(s, "ACTION:AUDIO") {
		t.Errorf("exotic AUDIO alarm dropped after edit:\n%s", s)
	}
}

func TestICSImportPrimaryCategoryLowercased(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:c1", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T100000Z",
		"SUMMARY:Review", "CATEGORIES:WORK", "END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	if events[0].Category() != "work" {
		t.Errorf("category = %q, want %q", events[0].Category(), "work")
	}
	// Re-encoding writes the modelled primary category back.
	if s := string(mustEncode(t, events[0])); !strings.Contains(s, "CATEGORIES:work") {
		t.Errorf("re-encoded ICS lost the category:\n%s", s)
	}
}

func TestICSEditKeepsExtraCategories(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:c2", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T100000Z",
		"SUMMARY:Review", "CATEGORIES:WORK,IMPORTANT", "END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	orig := events[0]
	if orig.Category() != "work" {
		t.Errorf("primary category = %q, want %q", orig.Category(), "work")
	}
	// Simulate an in-app edit that sets the primary category to meeting, the same way the application
	// rebuilds an event through EventInput while carrying its preserved ICS unchanged.
	edited, err := domain.NewEvent(domain.EventInput{
		ID: orig.ID(), UID: orig.UID(), Summary: orig.Summary(), Start: orig.Start(),
		Category: "meeting", Extra: orig.Extra(),
	})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	s := string(mustEncode(t, edited))
	// The primary slot becomes meeting; the extra IMPORTANT value is preserved (lossless multi-value).
	if !strings.Contains(s, "CATEGORIES:meeting,IMPORTANT") {
		t.Errorf("multi-value CATEGORIES not preserved on edit:\n%s", s)
	}
}

func TestICSFreshEventWritesCategory(t *testing.T) {
	ev, err := domain.NewEvent(domain.EventInput{
		ID: "c3", UID: "c3", Summary: "Lunch", Start: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
		Category: "personal",
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if s := string(mustEncode(t, ev)); !strings.Contains(s, "CATEGORIES:personal") {
		t.Errorf("fresh event did not write CATEGORIES:personal:\n%s", s)
	}
}

func TestICSEditToEmptyCategoryDropsProperty(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:c4", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T100000Z",
		"SUMMARY:Review", "CATEGORIES:WORK", "END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	orig := events[0]
	// The user clears the category: the single-value CATEGORIES is dropped from the export.
	cleared, err := domain.NewEvent(domain.EventInput{
		ID: orig.ID(), UID: orig.UID(), Summary: orig.Summary(), Start: orig.Start(), Extra: orig.Extra(),
	})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if s := string(mustEncode(t, cleared)); strings.Contains(s, "CATEGORIES") {
		t.Errorf("cleared category should drop CATEGORIES:\n%s", s)
	}
}

func mustEncode(t *testing.T, events ...domain.Event) []byte {
	t.Helper()
	out, err := New().Encode(events, nil)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return out
}

func TestDecodeThunderbirdStyleCalendar(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Mozilla.org//NONSGML Thunderbird//EN",
		"BEGIN:VEVENT", "UID:abc-123", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T100000Z",
		"DTEND:20260704T110000Z", "SUMMARY:Team meeting", "LOCATION:HQ", "DESCRIPTION:Quarterly review",
		"END:VEVENT", "END:VCALENDAR",
	)
	got, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d, want 1", len(got))
	}
	e := got[0]
	if e.ID() != "abc-123" || e.Summary() != "Team meeting" || e.Location() != "HQ" || e.Description() != "Quarterly review" {
		t.Errorf("mapping = %+v", e)
	}
	if !e.Start().Equal(time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("start = %v", e.Start())
	}
}

func TestDecodeNoUIDGeneratesID(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T100000Z", "SUMMARY:No Uid",
		"END:VEVENT", "END:VCALENDAR",
	)
	got, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 || got[0].ID() == "" || got[0].UID() != got[0].ID() {
		t.Errorf("expected a generated id used as uid, got %+v", got)
	}
}

func TestDecodeMissingSummaryGetsDefault(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:x1", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T100000Z",
		"END:VEVENT", "END:VCALENDAR",
	)
	got, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 || got[0].Summary() != "(no title)" {
		t.Errorf("expected default summary, got %+v", got)
	}
}

func TestDecodeSkipsEventWithoutStart(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:x1", "DTSTAMP:20260704T090000Z", "SUMMARY:No Start",
		"END:VEVENT", "END:VCALENDAR",
	)
	got, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected the start-less event skipped, got %d", len(got))
	}
}

func TestDecodeSkipsEndBeforeStart(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:x1", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T110000Z",
		"DTEND:20260704T100000Z", "SUMMARY:Backwards", "END:VEVENT", "END:VCALENDAR",
	)
	got, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected the end-before-start event skipped, got %d", len(got))
	}
}

func TestEncodeEmptyIsMinimalCalendar(t *testing.T) {
	data, err := New().Encode(nil, nil)
	if err != nil {
		t.Fatalf("Encode(nil): %v", err)
	}
	got, _, err := New().Decode(data)
	if err != nil || len(got) != 0 {
		t.Errorf("empty round-trip = %v, %v", got, err)
	}
}

func TestDecodeMalformedReturnsError(t *testing.T) {
	if _, _, err := New().Decode([]byte("this is not an ics file")); err == nil {
		t.Errorf("expected a decode error for malformed input")
	}
}

func TestICSPreservesTodoAndJournalPassthrough(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN",
		"BEGIN:VEVENT", "UID:e1", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T100000Z",
		"SUMMARY:Meeting", "END:VEVENT",
		"BEGIN:VTODO", "UID:todo-1", "DTSTAMP:20260704T090000Z", "SUMMARY:Buy milk",
		"STATUS:NEEDS-ACTION", "END:VTODO",
		"BEGIN:VJOURNAL", "UID:journal-1", "DTSTAMP:20260704T090000Z", "SUMMARY:Notes",
		"DESCRIPTION:Today went well", "END:VJOURNAL",
		"END:VCALENDAR",
	)
	events, passthrough, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("decoded %d events, want 1", len(events))
	}
	if len(passthrough) != 2 {
		t.Fatalf("decoded %d passthrough, want 2 (VTODO + VJOURNAL)", len(passthrough))
	}
	kinds := map[string]bool{}
	for _, p := range passthrough {
		kinds[p.Kind()] = true
	}
	if !kinds[domain.PassthroughToDo] || !kinds[domain.PassthroughJournal] {
		t.Errorf("passthrough kinds = %v, want both VTODO and VJOURNAL", kinds)
	}
	out, err := New().Encode(events, passthrough)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"BEGIN:VTODO", "UID:todo-1", "SUMMARY:Buy milk", "STATUS:NEEDS-ACTION",
		"BEGIN:VJOURNAL", "UID:journal-1", "DESCRIPTION:Today went well",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("round-trip dropped %q:\n%s", want, s)
		}
	}
}

func mustZonedEvent(t *testing.T, zone string) domain.Event {
	t.Helper()
	ev, err := domain.NewEvent(domain.EventInput{
		ID: "e1", UID: "e1", Summary: "Standup",
		Start: time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC), TimeZone: zone,
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	return ev
}

func TestICSExportGeneratesVTimezoneForDSTZone(t *testing.T) {
	out, err := New().Encode([]domain.Event{mustZonedEvent(t, "Europe/London")}, nil)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"BEGIN:VTIMEZONE", "TZID:Europe/London",
		"BEGIN:DAYLIGHT", "TZOFFSETFROM:+0000", "TZOFFSETTO:+0100", "BYMONTH=3;BYDAY=-1SU",
		"BEGIN:STANDARD", "TZOFFSETFROM:+0100", "TZOFFSETTO:+0000", "BYMONTH=10;BYDAY=-1SU",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("export missing %q:\n%s", want, s)
		}
	}
}

func TestICSExportVTimezoneForZoneWithoutDST(t *testing.T) {
	s := string(mustEncode(t, mustZonedEvent(t, "Asia/Tokyo")))
	if !strings.Contains(s, "TZID:Asia/Tokyo") || !strings.Contains(s, "BEGIN:STANDARD") ||
		!strings.Contains(s, "TZOFFSETTO:+0900") {
		t.Errorf("no-DST VTIMEZONE wrong:\n%s", s)
	}
	if strings.Contains(s, "BEGIN:DAYLIGHT") {
		t.Errorf("a zone without daylight saving should have no DAYLIGHT:\n%s", s)
	}
}

func TestICSExportNoVTimezoneForUTCEvent(t *testing.T) {
	if s := string(mustEncode(t, mustZonedEvent(t, ""))); strings.Contains(s, "BEGIN:VTIMEZONE") {
		t.Errorf("a UTC event should produce no VTIMEZONE:\n%s", s)
	}
}

func TestDecodeTeamsMeetingFallsBackToAltDesc(t *testing.T) {
	// A Teams invite with an empty plain DESCRIPTION: the join link lives in the HTML X-ALT-DESC and the
	// X-MICROSOFT-SKYPETEAMSMEETINGURL property. The imported description must surface the join URL.
	const joinURL = "https://teams.microsoft.com/l/meetup-join/abc"
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Microsoft//Outlook//EN",
		"BEGIN:VEVENT", "UID:teams-1", "DTSTAMP:20260706T090000Z", "DTSTART:20260706T100000Z",
		"SUMMARY:Sync",
		`X-ALT-DESC;FMTTYPE=text/html:<html><body><a href="`+joinURL+`">Join the meeting</a></body></html>`,
		"X-MICROSOFT-SKYPETEAMSMEETINGURL:"+joinURL,
		"END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	desc := events[0].Description()
	if !strings.Contains(desc, joinURL) {
		t.Errorf("description did not surface the Teams join URL: %q", desc)
	}
	if !strings.Contains(desc, "Join the meeting") {
		t.Errorf("description lost the alt-desc label: %q", desc)
	}
}

func TestDecodeTeamsMeetingAppendsJoinURLToDescription(t *testing.T) {
	// A plain DESCRIPTION that does not mention the join link: the Teams URL is appended so it is not lost.
	const joinURL = "https://teams.microsoft.com/l/meetup-join/xyz"
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Microsoft//Outlook//EN",
		"BEGIN:VEVENT", "UID:teams-2", "DTSTAMP:20260706T090000Z", "DTSTART:20260706T100000Z",
		"SUMMARY:Sync", "DESCRIPTION:Agenda: planning",
		"X-MICROSOFT-SKYPETEAMSMEETINGURL:"+joinURL,
		"END:VEVENT", "END:VCALENDAR",
	)
	events, _, err := New().Decode(data)
	if err != nil || len(events) != 1 {
		t.Fatalf("Decode: %v (n=%d)", err, len(events))
	}
	desc := events[0].Description()
	if !strings.Contains(desc, "Agenda: planning") {
		t.Errorf("description lost the plain text: %q", desc)
	}
	if !strings.Contains(desc, joinURL) {
		t.Errorf("description did not gain the Teams join URL: %q", desc)
	}
}
