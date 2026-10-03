package ics

import (
	"bytes"
	"testing"
	"time"

	goical "github.com/emersion/go-ical"
)

// zonesOf parses a calendar made of the given VTIMEZONE lines and returns its custom zones.
func zonesOf(t *testing.T, zone ...string) map[string]customZone {
	t.Helper()
	c, err := goical.NewDecoder(bytes.NewReader(zonedCalendar(zone, "DTSTART:20260101T000000Z"))).Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return customZones(c)
}

func TestParseOffset(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"+0530", 5*3600 + 30*60, true},
		{"-0430", -(4*3600 + 30*60), true},
		{"+013015", 3600 + 30*60 + 15, true},
		{"0530", 0, false},
		{"+05", 0, false},
		{"+05x0", 0, false},
		{"+0530xx", 0, false},
	}
	for _, c := range cases {
		got, ok := parseOffset(&goical.Prop{Value: c.in})
		if got != c.want || ok != c.ok {
			t.Errorf("parseOffset(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
	if _, ok := parseOffset(nil); ok {
		t.Errorf("parseOffset(nil) succeeded")
	}
}

// A definition with any unreadable block keeps only its IANA hint; with no hint it cannot resolve.
func TestCustomZoneUnreadableBlocks(t *testing.T) {
	for _, block := range [][]string{
		{"TZOFFSETFROM:+0100", "TZOFFSETTO:+0100"},
		{"DTSTART:bad", "TZOFFSETFROM:+0100", "TZOFFSETTO:+0100"},
		{"DTSTART:19700101T000000", "TZOFFSETFROM:+0100", "TZOFFSETTO:+0100", "RRULE:FREQ=NOPE"},
		{"DTSTART:19700101T000000", "TZOFFSETFROM:+0100", "TZOFFSETTO:+0100", "RRULE:FREQ=YEARLY;BYMONTH=13"},
		{"DTSTART:19700101T000000", "TZOFFSETFROM:+0100", "TZOFFSETTO:+0100", "RDATE:garbage"},
	} {
		lines := append([]string{"BEGIN:VTIMEZONE", "TZID:Broken", "BEGIN:STANDARD"}, block...)
		lines = append(lines, "END:STANDARD", "END:VTIMEZONE")
		zone := zonesOf(t, lines...)["Broken"]
		if len(zone.observances) != 0 {
			t.Errorf("%v: observances kept from an unreadable block", block)
		}
		prop := goical.Prop{Name: goical.PropDateTimeStart, Value: "20261110T090000", Params: goical.Params{}}
		if zone.resolve(&prop) {
			t.Errorf("%v: resolved without a usable definition", block)
		}
	}
	zone := zonesOf(t, "BEGIN:VTIMEZONE", "TZID:Hinted", "X-LIC-LOCATION:Europe/Paris", "BEGIN:STANDARD",
		"TZOFFSETFROM:+0100", "END:STANDARD", "END:VTIMEZONE")["Hinted"]
	if zone.hint != "Europe/Paris" {
		t.Errorf("hint = %q, want Europe/Paris", zone.hint)
	}
}

// Neither a hint that does not load nor a TZID with no IANA suffix gives a hint.
func TestZoneHintRejectsUnknownNames(t *testing.T) {
	zones := zonesOf(t, "BEGIN:VTIMEZONE", "TZID:Not/A/Zone", "X-LIC-LOCATION:Nowhere/Special",
		"BEGIN:STANDARD", "DTSTART:19700101T000000", "TZOFFSETFROM:+0100", "TZOFFSETTO:+0100",
		"END:STANDARD", "END:VTIMEZONE")
	if hint := zones["Not/A/Zone"].hint; hint != "" {
		t.Errorf("hint = %q, want none", hint)
	}
}

// A zone whose changes are listed by RDATE rather than RRULE is read at the right offset either side of
// each change; a value before the first change uses the offset the earliest block changes from.
func TestCustomZoneRDateTransitions(t *testing.T) {
	zones := zonesOf(t, "BEGIN:VTIMEZONE", "TZID:Listed",
		"BEGIN:DAYLIGHT", "DTSTART:20260301T020000", "RDATE:20270301T020000",
		"TZOFFSETFROM:+0317", "TZOFFSETTO:+0417", "END:DAYLIGHT",
		"BEGIN:STANDARD", "DTSTART:20261001T030000", "TZOFFSETFROM:+0417", "TZOFFSETTO:+0317",
		"END:STANDARD", "END:VTIMEZONE")
	zone := zones["Listed"]
	const std, dst = 3*3600 + 17*60, 4*3600 + 17*60
	cases := []struct {
		wall time.Time
		off  int
	}{
		{time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC), std},
		{time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC), dst},
		{time.Date(2026, 12, 15, 9, 0, 0, 0, time.UTC), std},
		{time.Date(2027, 6, 15, 9, 0, 0, 0, time.UTC), dst},
	}
	for _, c := range cases {
		want := c.wall.Add(-time.Duration(c.off) * time.Second)
		if got := zone.wallToUTC(c.wall); !got.Equal(want) {
			t.Errorf("wallToUTC(%v) = %v, want %v", c.wall, got, want)
		}
	}
	if got := zone.transitions(2026); len(got) != 2 || got[0].toOff != dst || got[1].toOff != std {
		t.Errorf("transitions = %+v", got)
	}
	// No IANA zone uses +0317, so a value in it is rewritten to UTC rather than matched.
	prop := goical.Prop{Name: goical.PropDateTimeStart, Value: "20260615T090000,20261215T090000",
		Params: goical.Params{goical.PropTimezoneID: []string{"Listed"}}}
	if !zone.resolve(&prop) || prop.Value != "20260615T044300Z,20261215T054300Z" ||
		prop.Params.Get(goical.PropTimezoneID) != "" {
		t.Errorf("resolve gave %q tzid=%q", prop.Value, prop.Params.Get(goical.PropTimezoneID))
	}
}

func TestSameTransitions(t *testing.T) {
	at := time.Date(2026, 3, 29, 1, 0, 0, 0, time.UTC)
	a := []tzTransition{{at: at, fromOff: 3600, toOff: 7200}}
	if !sameTransitions(a, a) {
		t.Errorf("identical lists differ")
	}
	if sameTransitions(a, nil) {
		t.Errorf("lists of different length match")
	}
	if sameTransitions(a, []tzTransition{{at: at, fromOff: 3600, toOff: 3600}}) {
		t.Errorf("different offsets match")
	}
}

// An all-day value carrying an unknown TZID that the file defines is read as a plain date.
func TestCustomZoneOnDateValueDegrades(t *testing.T) {
	ev := mustDecodeOne(t, zonedCalendar(outlookBerlin,
		"DTSTART;VALUE=DATE;TZID=\"(UTC+01:00) Amsterdam, Berlin, Rome\":20261110"))
	if !ev.AllDay() || !ev.Start().Equal(time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("all-day = %v start = %v", ev.AllDay(), ev.Start())
	}
}
