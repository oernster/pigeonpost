package csv

import "testing"

func TestParseBirthday(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{"iso", "1980-01-15", "1980-01-15"},
		{"year first with slashes", "1980/01/15", "1980-01-15"},
		{"day disambiguated by being over twelve", "15/01/1980", "1980-01-15"},
		{"month disambiguated by the day being over twelve", "01/15/1980", "1980-01-15"},
		{"ambiguous is read day first", "01/02/1980", "1980-02-01"},
		{"dotted", "15.01.1980", "1980-01-15"},
		{"spelled out day first", "15 January 1980", "1980-01-15"},
		{"spelled out month first", "January 15, 1980", "1980-01-15"},
		{"abbreviated month", "15 Jan 1980", "1980-01-15"},
		{"two digit year is rejected rather than guessed", "15/01/80", ""},
		{"impossible date", "31/02/1980", ""},
		{"month out of range", "1980-13-01", ""},
		{"not a date", "sometime in 1980", ""},
		{"wrong number of parts", "15/01", ""},
		{"non numeric part", "15/Jan/1980", ""},
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseBirthday(tc.in); got != tc.want {
				t.Errorf("parseBirthday(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBirthdayFromParts(t *testing.T) {
	for _, tc := range []struct {
		name, year, month, day, want string
	}{
		{"complete", "1980", "1", "15", "1980-01-15"},
		{"zero padded", "1980", "01", "05", "1980-01-05"},
		{"missing day", "1980", "1", "", ""},
		{"missing year", "", "1", "15", ""},
		{"non numeric", "nineteen eighty", "1", "15", ""},
		{"impossible", "1980", "2", "31", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := birthdayFromParts(tc.year, tc.month, tc.day); got != tc.want {
				t.Errorf("birthdayFromParts(%q,%q,%q) = %q, want %q", tc.year, tc.month, tc.day, got, tc.want)
			}
		})
	}
}
