package ics

import (
	"testing"
	"time"
)

// A Windows zone name (as Outlook and Exchange emit) must resolve to its IANA equivalent on import and
// interpret the wall-clock time in that zone, rather than dropping the event because LoadLocation rejects
// the Windows name.
func TestICSWindowsTimeZoneResolvesToIANA(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//test//EN",
		"BEGIN:VEVENT", "UID:tz-win-1", "SUMMARY:London meeting",
		"DTSTART;TZID=GMT Standard Time:20260115T100000",
		"DTEND;TZID=GMT Standard Time:20260115T110000",
		"END:VEVENT", "END:VCALENDAR",
	)
	got, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d events, want 1 (event dropped instead of resolving the Windows zone)", len(got))
	}
	e := got[0]
	if e.TimeZone() != "Europe/London" {
		t.Errorf("time zone = %q, want Europe/London", e.TimeZone())
	}
	// January London is GMT (UTC+0), so 10:00 local is 10:00 UTC.
	if want := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC); !e.Start().Equal(want) {
		t.Errorf("start = %v, want %v", e.Start(), want)
	}
}

// A Windows zone must be interpreted DST-correctly, not with a fixed offset.
func TestICSWindowsTimeZoneHonoursDST(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//test//EN",
		"BEGIN:VEVENT", "UID:tz-win-2", "SUMMARY:Berlin meeting",
		"DTSTART;TZID=W. Europe Standard Time:20260715T120000",
		"END:VEVENT", "END:VCALENDAR",
	)
	got, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d events, want 1", len(got))
	}
	e := got[0]
	if e.TimeZone() != "Europe/Berlin" {
		t.Errorf("time zone = %q, want Europe/Berlin", e.TimeZone())
	}
	// July Berlin is CEST (UTC+2), so 12:00 local is 10:00 UTC.
	if want := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC); !e.Start().Equal(want) {
		t.Errorf("start = %v, want %v", e.Start(), want)
	}
}

// An unresolvable zone must degrade to floating time (read in UTC, no zone) rather than dropping the event.
func TestICSUnresolvableTimeZoneDegradesToFloating(t *testing.T) {
	data := cal(
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//test//EN",
		"BEGIN:VEVENT", "UID:tz-bad-1", "SUMMARY:Mystery zone",
		"DTSTART;TZID=Totally Bogus Zone:20260115T100000",
		"END:VEVENT", "END:VCALENDAR",
	)
	got, _, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d events, want 1 (unresolvable zone dropped the event)", len(got))
	}
	e := got[0]
	if e.TimeZone() != "" {
		t.Errorf("time zone = %q, want empty (floating)", e.TimeZone())
	}
	if want := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC); !e.Start().Equal(want) {
		t.Errorf("start = %v, want %v (floating read in UTC)", e.Start(), want)
	}
}

func TestResolveZone(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"GMT Standard Time", "Europe/London", true},
		{"W. Europe Standard Time", "Europe/Berlin", true},
		{"Europe/Paris", "Europe/Paris", true},
		{"Totally Bogus Zone", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := resolveZone(c.in)
		if got != c.want || ok != c.wantOK {
			t.Errorf("resolveZone(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.wantOK)
		}
	}
}
