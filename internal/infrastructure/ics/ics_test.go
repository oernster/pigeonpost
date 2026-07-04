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
	data, err := New().Encode([]domain.Event{ev})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := New().Decode(data)
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
	data, err := New().Encode([]domain.Event{ev})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := New().Decode(data)
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

func TestDecodeThunderbirdStyleCalendar(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Mozilla.org//NONSGML Thunderbird//EN",
		"BEGIN:VEVENT", "UID:abc-123", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T100000Z",
		"DTEND:20260704T110000Z", "SUMMARY:Team meeting", "LOCATION:HQ", "DESCRIPTION:Quarterly review",
		"END:VEVENT", "END:VCALENDAR",
	)
	got, err := New().Decode(data)
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
	got, err := New().Decode(data)
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
	got, err := New().Decode(data)
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
	got, err := New().Decode(data)
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
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected the end-before-start event skipped, got %d", len(got))
	}
}

func TestEncodeEmptyIsMinimalCalendar(t *testing.T) {
	data, err := New().Encode(nil)
	if err != nil {
		t.Fatalf("Encode(nil): %v", err)
	}
	got, err := New().Decode(data)
	if err != nil || len(got) != 0 {
		t.Errorf("empty round-trip = %v, %v", got, err)
	}
}

func TestDecodeMalformedReturnsError(t *testing.T) {
	if _, err := New().Decode([]byte("this is not an ics file")); err == nil {
		t.Errorf("expected a decode error for malformed input")
	}
}
