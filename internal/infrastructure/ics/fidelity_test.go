package ics

import (
	"strings"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// mustDecode decodes a calendar and fails the test on a decode error.
func mustDecode(t *testing.T, data []byte) []domain.Event {
	t.Helper()
	events, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return events
}

// mustDecodeOne decodes a calendar that must hold exactly one usable event.
func mustDecodeOne(t *testing.T, data []byte) domain.Event {
	t.Helper()
	events := mustDecode(t, data)
	if len(events) != 1 {
		t.Fatalf("decoded %d events, want 1", len(events))
	}
	return events[0]
}

// vevent wraps VEVENT body lines in a calendar with the required UID and DTSTAMP.
func vevent(body ...string) []byte {
	lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//EN",
		"BEGIN:VEVENT", "UID:f1", "DTSTAMP:20261101T000000Z", "SUMMARY:Fidelity"}
	lines = append(lines, body...)
	lines = append(lines, "END:VEVENT", "END:VCALENDAR")
	return cal(lines...)
}

// A series master and its RECURRENCE-ID override share a UID; each must decode to its own id so the
// store keeps both rows; both must also be written back out on export (P-3).
func TestDecodeOverrideGetsDistinctID(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//EN",
		"BEGIN:VEVENT", "UID:weekly-1", "DTSTAMP:20261101T000000Z", "DTSTART:20261102T090000Z",
		"DTEND:20261102T100000Z", "SUMMARY:Weekly", "RRULE:FREQ=WEEKLY;COUNT=5", "END:VEVENT",
		"BEGIN:VEVENT", "UID:weekly-1", "DTSTAMP:20261101T000000Z", "DTSTART:20261109T140000Z",
		"DTEND:20261109T150000Z", "SUMMARY:Weekly moved", "RECURRENCE-ID:20261109T090000Z", "END:VEVENT",
		"END:VCALENDAR",
	)
	events := mustDecode(t, data)
	if len(events) != 2 {
		t.Fatalf("decoded %d events, want 2", len(events))
	}
	master, override := events[0], events[1]
	if master.ID() == override.ID() {
		t.Fatalf("master and override share id %q", master.ID())
	}
	rid := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
	if master.ID() != "weekly-1" || override.ID() != domain.EventIDFor("weekly-1", rid) {
		t.Errorf("ids = %q, %q", master.ID(), override.ID())
	}
	if master.UID() != "weekly-1" || override.UID() != "weekly-1" {
		t.Errorf("uids = %q, %q, want both weekly-1", master.UID(), override.UID())
	}
	out := string(mustEncode(t, events...))
	if n := strings.Count(out, "BEGIN:VEVENT"); n != 2 {
		t.Errorf("encoded %d VEVENTs, want 2:\n%s", n, out)
	}
	if strings.Count(out, "UID:weekly-1") != 2 || !strings.Contains(out, "RECURRENCE-ID:20261109T090000Z") {
		t.Errorf("override not written back with the shared UID:\n%s", out)
	}
}

// SEQUENCE is read on import (absent, malformed or negative reads as 0) and written on export when
// non-zero, so a newer invitation can be told from an older one.
func TestICSSequenceRoundTrip(t *testing.T) {
	cases := []struct {
		line string
		want int
	}{
		{"SEQUENCE:3", 3},
		{"SEQUENCE:abc", 0},
		{"SEQUENCE:-2", 0},
		{"", 0},
	}
	for _, c := range cases {
		body := []string{"DTSTART:20261110T090000Z"}
		if c.line != "" {
			body = append(body, c.line)
		}
		ev := mustDecodeOne(t, vevent(body...))
		if ev.Sequence() != c.want {
			t.Errorf("%q: Sequence() = %d, want %d", c.line, ev.Sequence(), c.want)
		}
		out := string(mustEncode(t, ev))
		hasSeq := strings.Contains(out, "SEQUENCE:")
		if c.want == 0 && hasSeq {
			t.Errorf("%q: zero sequence written:\n%s", c.line, out)
		}
		if c.want != 0 && !strings.Contains(out, "SEQUENCE:3") {
			t.Errorf("%q: sequence not written:\n%s", c.line, out)
		}
	}
}

// An iTIP payload carries its SEQUENCE through to the scheduling message's events.
func TestDecodeSchedulingReadsSequence(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//EN", "METHOD:REQUEST",
		"BEGIN:VEVENT", "UID:m1", "DTSTAMP:20260704T090000Z", "DTSTART:20260704T090000Z",
		"SEQUENCE:4", "SUMMARY:Sync", "ORGANIZER:mailto:chair@example.com", "END:VEVENT", "END:VCALENDAR",
	)
	msg, err := New().DecodeScheduling(data)
	if err != nil {
		t.Fatalf("DecodeScheduling: %v", err)
	}
	if got := msg.PrimaryEvent().Sequence(); got != 4 {
		t.Errorf("Sequence() = %d, want 4", got)
	}
}

// Plain text holding an angle-bracketed address is not HTML and must survive untouched (P-10).
func TestDescriptionKeepsAngleBracketAddress(t *testing.T) {
	ev := mustDecodeOne(t, vevent("DTSTART:20261110T090000Z",
		"DESCRIPTION:Call John <john@x.com> first", "LOCATION:Desk of Ann <ann@x.com>"))
	if ev.Description() != "Call John <john@x.com> first" {
		t.Errorf("description = %q", ev.Description())
	}
	if ev.Location() != "Desk of Ann <ann@x.com>" {
		t.Errorf("location = %q", ev.Location())
	}
	if got := descriptionText("a < b and c > d"); got != "a < b and c > d" {
		t.Errorf("comparison text changed: %q", got)
	}
}

// A trigger anchored to the END fires that long before the end; it is written back anchored to the end.
func TestICSAlarmRelatedToEnd(t *testing.T) {
	ev := mustDecodeOne(t, vevent("DTSTART:20261110T090000Z", "DTEND:20261110T100000Z",
		"BEGIN:VALARM", "ACTION:DISPLAY", "TRIGGER;RELATED=END:-PT15M", "DESCRIPTION:x", "END:VALARM"))
	alarms := ev.Alarms()
	if len(alarms) != 1 {
		t.Fatalf("alarms = %d, want 1", len(alarms))
	}
	if got, want := alarms[0].TriggerAt(ev.Start()), time.Date(2026, 11, 10, 9, 45, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("fires at %v, want %v (15 min before the end)", got, want)
	}
	out := string(mustEncode(t, ev))
	if !strings.Contains(out, "TRIGGER;RELATED=END:-PT15M") {
		t.Errorf("end-anchored trigger not round-tripped:\n%s", out)
	}
	if strings.Count(out, "BEGIN:VALARM") != 1 {
		t.Errorf("alarm duplicated on export:\n%s", out)
	}
}

// A DURATION without DTEND gives the event its real end rather than none.
func TestDecodeDurationWithoutEnd(t *testing.T) {
	ev := mustDecodeOne(t, vevent("DTSTART:20261110T090000Z", "DURATION:PT1H30M"))
	if want := time.Date(2026, 11, 10, 10, 30, 0, 0, time.UTC); !ev.End().Equal(want) {
		t.Errorf("end = %v, want %v", ev.End(), want)
	}
}

// A floating time (no Z, no TZID) stays floating on export rather than becoming UTC.
func TestICSFloatingTimeStaysFloating(t *testing.T) {
	ev := mustDecodeOne(t, vevent("DTSTART:20261110T090000", "DTEND:20261110T100000",
		"RRULE:FREQ=DAILY;COUNT=3", "EXDATE:20261111T090000"))
	out := string(mustEncode(t, ev))
	for _, want := range []string{"DTSTART:20261110T090000\r\n", "DTEND:20261110T100000\r\n", "EXDATE:20261111T090000\r\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing floating %q:\n%s", strings.TrimSpace(want), out)
		}
	}
}

// outlookBerlin is the VTIMEZONE Outlook writes for a display-name TZID: a Central European zone whose
// summer time starts on the last Sunday in March and ends on the last Sunday in October.
var outlookBerlin = []string{
	"BEGIN:VTIMEZONE", "TZID:(UTC+01:00) Amsterdam\\, Berlin\\, Rome",
	"BEGIN:STANDARD", "DTSTART:16010101T030000", "TZOFFSETFROM:+0200", "TZOFFSETTO:+0100",
	"RRULE:FREQ=YEARLY;BYDAY=-1SU;BYMONTH=10", "END:STANDARD",
	"BEGIN:DAYLIGHT", "DTSTART:16010101T020000", "TZOFFSETFROM:+0100", "TZOFFSETTO:+0200",
	"RRULE:FREQ=YEARLY;BYDAY=-1SU;BYMONTH=3", "END:DAYLIGHT", "END:VTIMEZONE",
}

// zonedCalendar builds a calendar holding the VTIMEZONE lines and one event starting at dtstart.
func zonedCalendar(zone []string, dtstart string) []byte {
	lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//EN"}
	lines = append(lines, zone...)
	lines = append(lines, "BEGIN:VEVENT", "UID:z1", "DTSTAMP:20261101T000000Z", "SUMMARY:Zoned",
		dtstart, "END:VEVENT", "END:VCALENDAR")
	return cal(lines...)
}

// An Outlook display-name TZID resolves through the file's own VTIMEZONE, honouring its daylight saving,
// to the IANA zone with the same rules.
func TestICSOutlookDisplayNameZoneUsesVTimezone(t *testing.T) {
	tzid := "DTSTART;TZID=\"(UTC+01:00) Amsterdam, Berlin, Rome\":"
	cases := []struct {
		wall string
		want time.Time
	}{
		{"20260715T090000", time.Date(2026, 7, 15, 7, 0, 0, 0, time.UTC)},
		{"20260115T090000", time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		ev := mustDecodeOne(t, zonedCalendar(outlookBerlin, tzid+c.wall))
		if !ev.Start().Equal(c.want) {
			t.Errorf("%s: start = %v, want %v", c.wall, ev.Start().UTC(), c.want)
		}
		if ev.TimeZone() != "Europe/Berlin" {
			t.Errorf("%s: zone = %q, want Europe/Berlin", c.wall, ev.TimeZone())
		}
	}
}

// A custom zone with no daylight saving and an offset no known zone has is still read at its own offset.
func TestICSCustomFixedZoneUsesVTimezone(t *testing.T) {
	zone := []string{"BEGIN:VTIMEZONE", "TZID:Odd Zone", "BEGIN:STANDARD", "DTSTART:19700101T000000",
		"TZOFFSETFROM:+0517", "TZOFFSETTO:+0517", "END:STANDARD", "END:VTIMEZONE"}
	ev := mustDecodeOne(t, zonedCalendar(zone, "DTSTART;TZID=Odd Zone:20261110T090000"))
	if want := time.Date(2026, 11, 10, 3, 43, 0, 0, time.UTC); !ev.Start().Equal(want) {
		t.Errorf("start = %v, want %v", ev.Start().UTC(), want)
	}
	if ev.TimeZone() != "" {
		t.Errorf("zone = %q, want empty (a fixed offset has no IANA name)", ev.TimeZone())
	}
}

// A custom TZID matching a constant-offset zone (or holding an IANA name) resolves to that zone.
func TestICSCustomZoneMatchesIANA(t *testing.T) {
	india := []string{"BEGIN:VTIMEZONE", "TZID:India Custom", "BEGIN:STANDARD", "DTSTART:16010101T000000",
		"TZOFFSETFROM:+0530", "TZOFFSETTO:+0530", "END:STANDARD", "END:VTIMEZONE"}
	ev := mustDecodeOne(t, zonedCalendar(india, "DTSTART;TZID=India Custom:20261110T090000"))
	if want := time.Date(2026, 11, 10, 3, 30, 0, 0, time.UTC); !ev.Start().Equal(want) {
		t.Errorf("india start = %v, want %v", ev.Start().UTC(), want)
	}
	// Asia/Colombo and Asia/Kolkata share every rule; either is a faithful match, so the offset is checked.
	const indiaOffset = 5*3600 + 30*60
	loc, err := time.LoadLocation(ev.TimeZone())
	if _, off := ev.Start().In(loc).Zone(); err != nil || ev.TimeZone() == "" || off != indiaOffset {
		t.Errorf("india zone = %q (offset %d), want a +0530 IANA zone", ev.TimeZone(), off)
	}
	mozilla := []string{"BEGIN:VTIMEZONE", "TZID:/mozilla.org/20050126_1/America/New_York",
		"BEGIN:STANDARD", "DTSTART:19701101T020000", "TZOFFSETFROM:-0400", "TZOFFSETTO:-0500",
		"END:STANDARD", "END:VTIMEZONE"}
	ev = mustDecodeOne(t, zonedCalendar(mozilla, "DTSTART;TZID=/mozilla.org/20050126_1/America/New_York:20260715T090000"))
	if ev.TimeZone() != "America/New_York" || !ev.Start().Equal(time.Date(2026, 7, 15, 13, 0, 0, 0, time.UTC)) {
		t.Errorf("mozilla zone = %q start = %v", ev.TimeZone(), ev.Start().UTC())
	}
}
