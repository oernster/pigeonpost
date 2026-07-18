package csv

import (
	"strconv"
	"strings"
	"time"
)

const (
	// isoDateLayout is the form the contact editor's date input accepts. A birthday is normalised to it
	// or dropped, since a value the input cannot parse shows as an empty field and would read as data
	// loss the next time the contact is saved.
	isoDateLayout = "2006-01-02"
	// datePartCount is the number of components in a numeric date.
	datePartCount = 3
	// monthsInYear bounds the month component. A day-or-month component larger than this can only be
	// the day, which is what resolves most numeric dates without having to know the source locale.
	monthsInYear = 12
	// isoYearDigits is the width of a four-digit year, used to spot which component is the year.
	isoYearDigits = 4
)

// textDateLayouts are the spelled-out forms Outlook writes on some locales. They name the month, so
// they carry no day-or-month ambiguity and are tried first.
var textDateLayouts = []string{
	"2 January 2006", "02 January 2006", "2 Jan 2006", "02 Jan 2006",
	"January 2, 2006", "Jan 2, 2006", "January 2 2006",
}

// dateSeparators are the characters either exporter uses between numeric date components.
func dateSeparators(r rune) bool { return r == '/' || r == '-' || r == '.' }

// parseBirthday normalises an exported birthday to ISO form, returning an empty string when the value
// is absent or cannot be read confidently. Outlook writes a single Birthday column in the locale of
// the machine that exported it; Thunderbird writes the parts separately (see birthdayFromParts).
func parseBirthday(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range textDateLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.Format(isoDateLayout)
		}
	}
	return parseNumericDate(value)
}

// parseNumericDate reads a three-part numeric date, working out which component is which from the
// values themselves rather than assuming a locale.
func parseNumericDate(value string) string {
	parts := strings.FieldsFunc(value, dateSeparators)
	if len(parts) != datePartCount {
		return ""
	}
	nums := make([]int, datePartCount)
	for i, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return ""
		}
		nums[i] = n
	}
	year, month, day, ok := orderDateParts(parts, nums)
	if !ok {
		return ""
	}
	return isoDate(year, month, day)
}

// orderDateParts identifies the year, month and day of a numeric date.
//
// A four-digit component fixes the year, at either end. Once the year is known, a leading or middle
// component above twelve can only be the day, which settles most real dates. What is genuinely
// undecidable is a date such as 01/02/1980, where day-first and month-first are both valid readings
// and nothing in the file says which the exporting machine used. Those are read day-first: it is the
// reading Thunderbird produces on this project's own locale and the one Outlook produces everywhere
// outside the United States. A date with no four-digit year is rejected rather than guessed, since a
// two-digit year adds a third unknown to a choice that is already a coin toss.
func orderDateParts(parts []string, nums []int) (year, month, day int, ok bool) {
	const first, middle, last = 0, 1, 2
	switch {
	case len(parts[first]) == isoYearDigits:
		return nums[first], nums[middle], nums[last], true
	case len(parts[last]) != isoYearDigits:
		return 0, 0, 0, false
	case nums[first] > monthsInYear:
		return nums[last], nums[middle], nums[first], true
	case nums[middle] > monthsInYear:
		return nums[last], nums[first], nums[middle], true
	default:
		return nums[last], nums[middle], nums[first], true
	}
}

// birthdayFromParts builds an ISO date from Thunderbird's separate year, month and day columns, which
// are unambiguous because each is named. Any part missing or unreadable yields an empty string.
func birthdayFromParts(year, month, day string) string {
	y, errY := strconv.Atoi(strings.TrimSpace(year))
	m, errM := strconv.Atoi(strings.TrimSpace(month))
	d, errD := strconv.Atoi(strings.TrimSpace(day))
	if errY != nil || errM != nil || errD != nil {
		return ""
	}
	return isoDate(y, m, d)
}

// isoDate formats a validated calendar date, returning an empty string when the components do not
// describe a real one. time.Date normalises out-of-range values (month 13 becomes January of the next
// year), so the result is compared back against the input to reject them.
func isoDate(year, month, day int) string {
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if t.Year() != year || int(t.Month()) != month || t.Day() != day {
		return ""
	}
	return t.Format(isoDateLayout)
}
