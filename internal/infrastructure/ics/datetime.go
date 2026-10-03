package ics

import (
	"strings"
	"time"

	goical "github.com/emersion/go-ical"
)

// This file reads and writes the date and date-time values of a VEVENT. On import every timed value is
// read as a UTC instant; a floating value (no Z and no TZID) is read with its wall clock taken as UTC. On
// export a value is spelled by a timeForm: a DATE for an all-day event, a TZID-qualified local time for a
// zoned event, a floating wall clock for an event that was floating when imported, otherwise UTC.

// parseDateList reads every occurrence start from the named property (RDATE or EXDATE), which may repeat
// and may carry a comma-separated list of DATE or DATE-TIME values. Unparseable or zero values are
// skipped so a malformed entry cannot fail the whole import.
func parseDateList(props goical.Props, name string) []time.Time {
	var out []time.Time
	for _, prop := range props[name] {
		for _, raw := range strings.Split(prop.Value, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			part := prop
			part.Value = raw
			when, err := part.DateTime(time.UTC)
			if err != nil || when.IsZero() {
				continue
			}
			out = append(out, when.UTC())
		}
	}
	return out
}

// parseRecurrenceID reads the RECURRENCE-ID that marks an event as an override of a single occurrence,
// returning the zero time when the property is absent or unparseable.
func parseRecurrenceID(props goical.Props) time.Time {
	prop := props.Get(goical.PropRecurrenceID)
	if prop == nil {
		return time.Time{}
	}
	when, err := prop.DateTime(time.UTC)
	if err != nil {
		return time.Time{}
	}
	return when.UTC()
}

// eventEnd reads the event's end from DTEND; failing that, from DTSTART plus DURATION, so an event
// written with a duration keeps its length. An event with neither (or with an unreadable value) has no end
// (the zero time); the all-day default of one day that go-ical applies is deliberately not used here.
func eventEnd(e goical.Event) time.Time {
	if e.Props.Get(goical.PropDateTimeEnd) == nil && e.Props.Get(goical.PropDuration) == nil {
		return time.Time{}
	}
	end, err := e.DateTimeEnd(time.UTC)
	if err != nil {
		return time.Time{}
	}
	return end
}

// timeForm decides how a timed value is spelled on export: in loc (a real zone makes go-ical add the TZID
// parameter, UTC yields a Z value); with floating set it is a floating wall clock instead.
type timeForm struct {
	loc      *time.Location
	floating bool
}

// exportForm picks the form for an event kept in zone. A zoned event is written in its zone; a zone-less
// event is written floating when its preserved original DTSTART was floating, so a floating import stays
// floating; otherwise it is written in UTC.
func exportForm(zone string, base *goical.Component) timeForm {
	if zone != "" {
		return timeForm{loc: icsLocation(zone)}
	}
	return timeForm{loc: time.UTC, floating: floatingStart(base)}
}

// floatingStart reports whether the component's DTSTART is a floating date-time: a DATE-TIME value with
// neither a TZID parameter nor the UTC Z suffix.
func floatingStart(comp *goical.Component) bool {
	prop := comp.Props.Get(goical.PropDateTimeStart)
	if prop == nil || prop.Params.Get(goical.PropTimezoneID) != "" {
		return false
	}
	return len(prop.Value) == len(tzDateTimeLayout) && prop.ValueType() != goical.ValueDate
}

// setWhen writes a date or date-time property in the form. An all-day value is a DATE.
func (f timeForm) setWhen(comp *goical.Component, name string, when time.Time, allDay bool) {
	if allDay {
		comp.Props.SetDate(name, when)
		return
	}
	if f.floating {
		prop := goical.NewProp(name)
		prop.SetValueType(goical.ValueDateTime)
		prop.Value = f.format(when, false)
		comp.Props.Set(prop)
		return
	}
	comp.Props.SetDateTime(name, when.In(f.loc))
}

// setDateList overwrites a date-list property (RDATE or EXDATE) with the given occurrence starts as a
// single comma-separated value; it removes the property when the list is empty, so an in-app edit replaces rather
// than duplicates any preserved list. Values are DATE for an all-day event, floating for a floating event
// and UTC DATE-TIME otherwise.
func (f timeForm) setDateList(comp *goical.Component, name string, times []time.Time, allDay bool) {
	comp.Props.Del(name)
	if len(times) == 0 {
		return
	}
	parts := make([]string, len(times))
	for i, t := range times {
		parts[i] = f.format(t, allDay)
	}
	prop := goical.NewProp(name)
	if allDay {
		prop.SetValueType(goical.ValueDate)
	} else {
		prop.SetValueType(goical.ValueDateTime)
	}
	prop.Value = strings.Join(parts, ",")
	comp.Props.Set(prop)
}

// format renders a single time in the ICS DATE, floating DATE-TIME or UTC DATE-TIME wire form, reusing the
// library's own property setters so the date and UTC format strings are not duplicated here. A floating
// value was read with its wall clock taken as UTC, so its UTC fields are that wall clock.
func (f timeForm) format(t time.Time, allDay bool) string {
	prop := goical.NewProp(goical.PropDateTimeStart)
	switch {
	case allDay:
		prop.SetDate(t)
	case f.floating:
		return t.UTC().Format(tzDateTimeLayout)
	default:
		prop.SetDateTime(t.UTC())
	}
	return prop.Value
}

// icsLocation loads the IANA zone, falling back to UTC for an empty name or an unknown zone so export
// never fails on a bad zone; a UTC location makes setWhen write plain Z values.
func icsLocation(zone string) *time.Location {
	if zone == "" {
		return time.UTC
	}
	if loc, err := time.LoadLocation(zone); err == nil {
		return loc
	}
	return time.UTC
}
