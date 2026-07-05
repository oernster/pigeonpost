package ics

import (
	"fmt"
	"time"

	goical "github.com/emersion/go-ical"

	"github.com/oernster/pigeonpost/internal/domain"
)

// hoursPerYearScan bounds the hourly probe across a reference year when finding daylight-saving
// transitions (a leap year of hours, with the loop stopping once the year rolls over).
const hoursPerYearScan = 366 * 24

// tzDateTimeLayout is the floating local date-time form a VTIMEZONE DTSTART uses (no zone suffix).
const tzDateTimeLayout = "20060102T150405"

// weekdayCodes maps a Go weekday to the RFC 5545 two-letter BYDAY code.
var weekdayCodes = [...]string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}

// timezoneComponents builds a VTIMEZONE for every distinct IANA zone the events are kept in, so an
// exported calendar defines the zones its TZID parameters reference instead of relying on the reading
// application's own database. UTC, all-day and unloadable zones produce nothing.
func timezoneComponents(events []domain.Event) []*goical.Component {
	// Each zone is generated for the earliest event year using it, so the reference year is deterministic
	// and needs no wall-clock read.
	refYear := map[string]int{}
	var order []string
	for _, ev := range events {
		zone := ev.TimeZone()
		if zone == "" {
			continue
		}
		year := ev.Start().UTC().Year()
		if existing, ok := refYear[zone]; !ok {
			refYear[zone] = year
			order = append(order, zone)
		} else if year < existing {
			refYear[zone] = year
		}
	}
	var out []*goical.Component
	for _, zone := range order {
		if comp := timezoneComponent(zone, refYear[zone]); comp != nil {
			out = append(out, comp)
		}
	}
	return out
}

// timezoneComponent builds the VTIMEZONE for one IANA zone in the reference year. It probes the zone for
// its standard offset and, when daylight saving is observed that year, the two transitions, emitting a
// STANDARD sub-component and a DAYLIGHT one, each with an RRULE describing the recurring change. A zone
// that cannot be loaded, or that is plain UTC, yields nil.
func timezoneComponent(zone string, year int) *goical.Component {
	loc, err := time.LoadLocation(zone)
	if err != nil || loc == time.UTC {
		return nil
	}
	comp := goical.NewComponent(goical.CompTimezone)
	comp.Props.SetText(goical.PropTimezoneID, zone)
	transitions := findTransitions(loc, year)
	if len(transitions) < 2 {
		// No daylight saving this year: a single STANDARD carrying the constant offset.
		_, off := time.Date(year, 1, 1, 0, 0, 0, 0, loc).Zone()
		start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		comp.Children = append(comp.Children, subComponent(goical.CompTimezoneStandard, off, off, start, ""))
		return comp
	}
	for _, tr := range transitions {
		name := goical.CompTimezoneStandard
		if tr.toOff > tr.fromOff {
			name = goical.CompTimezoneDaylight // clocks moved forward: daylight time begins
		}
		// The DTSTART is the onset in local time using the from-offset; shifting the UTC instant by that
		// offset makes its UTC representation read as the wall clock.
		onset := tr.at.Add(time.Duration(tr.fromOff) * time.Second).UTC()
		comp.Children = append(comp.Children, subComponent(name, tr.fromOff, tr.toOff, onset, transitionRule(onset)))
	}
	return comp
}

// tzTransition is a single change in a zone's UTC offset within the reference year.
type tzTransition struct {
	at      time.Time // UTC instant at which the offset changes
	fromOff int       // seconds east of UTC before the change
	toOff   int       // seconds east of UTC after the change
}

// findTransitions probes a zone hour by hour across the year and records each offset change. A zone with
// no daylight saving yields none; a typical one yields two (the spring and autumn changes).
func findTransitions(loc *time.Location, year int) []tzTransition {
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	_, prev := start.In(loc).Zone()
	var out []tzTransition
	for h := 1; h <= hoursPerYearScan; h++ {
		t := start.Add(time.Duration(h) * time.Hour)
		if t.Year() > year {
			break
		}
		if _, off := t.In(loc).Zone(); off != prev {
			out = append(out, tzTransition{at: t, fromOff: prev, toOff: off})
			prev = off
		}
	}
	return out
}

// subComponent builds a STANDARD or DAYLIGHT sub-component with its onset, offsets and optional
// recurrence rule.
func subComponent(name string, fromOff, toOff int, onset time.Time, rule string) *goical.Component {
	sub := goical.NewComponent(name)
	dtstart := goical.NewProp(goical.PropDateTimeStart)
	dtstart.SetValueType(goical.ValueDateTime)
	dtstart.Value = onset.Format(tzDateTimeLayout)
	sub.Props.Set(dtstart)
	sub.Props.Set(rawProp(goical.PropTimezoneOffsetFrom, formatOffset(fromOff)))
	sub.Props.Set(rawProp(goical.PropTimezoneOffsetTo, formatOffset(toOff)))
	if rule != "" {
		sub.Props.Set(rawProp(goical.PropRecurrenceRule, rule))
	}
	return sub
}

// rawProp builds a property with a literal value and no forced value type, so an offset is written as
// the bare TZOFFSETFROM:+0000 rather than being tagged VALUE=TEXT.
func rawProp(name, value string) *goical.Prop {
	prop := goical.NewProp(name)
	prop.Value = value
	return prop
}

// formatOffset renders a UTC offset in seconds as the RFC 5545 +HHMM or -HHMM form.
func formatOffset(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	return fmt.Sprintf("%s%02d%02d", sign, seconds/3600, (seconds%3600)/60)
}

// transitionRule derives the yearly RRULE for a transition from its onset date: the same month and the
// same weekday-of-month, using -1 for a transition on the last such weekday of the month.
func transitionRule(onset time.Time) string {
	weekday := weekdayCodes[int(onset.Weekday())]
	ordinal := (onset.Day()-1)/7 + 1
	if onset.AddDate(0, 0, 7).Month() != onset.Month() {
		return fmt.Sprintf("FREQ=YEARLY;BYMONTH=%d;BYDAY=-1%s", int(onset.Month()), weekday)
	}
	return fmt.Sprintf("FREQ=YEARLY;BYMONTH=%d;BYDAY=%d%s", int(onset.Month()), ordinal, weekday)
}
