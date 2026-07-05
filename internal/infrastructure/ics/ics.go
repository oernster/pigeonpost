// Package ics converts calendar events to and from iCalendar (RFC 5545, .ics), the format Thunderbird
// and Outlook import and export. It implements the application CalendarCodec port and depends only on
// the domain and the go-ical library.
package ics

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	goical "github.com/emersion/go-ical"

	"github.com/oernster/pigeonpost/internal/domain"
)

// generatedIDBytes is the length of a random id assigned to an event that carries no UID.
const generatedIDBytes = 16

// productID identifies PigeonPost as the writer of an exported calendar (the required PRODID property).
const productID = "-//PigeonPost//Calendar//EN"

// emptyCalendar is a minimal valid VCALENDAR, returned when there are no events to encode (the encoder
// rejects a childless calendar).
var emptyCalendar = []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:" + productID + "\r\nEND:VCALENDAR\r\n")

// Codec is the iCalendar implementation of the application CalendarCodec port.
type Codec struct{}

// New constructs an ICS codec.
func New() Codec { return Codec{} }

// Decode parses the VEVENTs from one or more VCALENDARs into events. An event's UID becomes its id so a
// re-import updates the same record; a UID-less event is given a generated id. An event that cannot form
// a valid domain value (no start, or an end before its start) is skipped rather than failing the import.
func (Codec) Decode(data []byte) ([]domain.Event, error) {
	dec := goical.NewDecoder(bytes.NewReader(data))
	var events []domain.Event
	for {
		cal, err := dec.Decode()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("ics: decode: %w", err)
		}
		for _, e := range cal.Events() {
			event, ok := eventFromICS(e)
			if ok {
				events = append(events, event)
			}
		}
	}
	return events, nil
}

// eventFromICS maps a parsed VEVENT into a domain event. The bool is false for an event that cannot be
// represented (no usable start, or a validation failure), which the caller skips.
func eventFromICS(e goical.Event) (domain.Event, bool) {
	start, err := e.DateTimeStart(time.UTC)
	if err != nil || start.IsZero() {
		return domain.Event{}, false
	}
	var end time.Time
	if e.Props.Get(goical.PropDateTimeEnd) != nil {
		if parsed, endErr := e.DateTimeEnd(time.UTC); endErr == nil {
			end = parsed
		}
	}
	uid := text(e.Props, goical.PropUID)
	id := uid
	if id == "" {
		id = generatedID()
		uid = id
	}
	summary := text(e.Props, goical.PropSummary)
	if summary == "" {
		summary = "(no title)"
	}
	allDay := false
	zone := ""
	if startProp := e.Props.Get(goical.PropDateTimeStart); startProp != nil {
		allDay = startProp.ValueType() == goical.ValueDate
		// The TZID parameter names the IANA zone the wall-clock time is in; a UTC or all-day start has none.
		zone = startProp.Params.Get(goical.PropTimezoneID)
	}
	recurrence := ""
	if rrule := e.Props.Get(goical.PropRecurrenceRule); rrule != nil {
		recurrence = rrule.Value
	}
	event, err := domain.NewEvent(domain.EventInput{
		ID:           id,
		UID:          uid,
		Summary:      summary,
		Description:  text(e.Props, goical.PropDescription),
		Location:     text(e.Props, goical.PropLocation),
		Start:        start,
		End:          end,
		AllDay:       allDay,
		Recurrence:   recurrence,
		RDates:       parseDateList(e.Props, goical.PropRecurrenceDates),
		ExDates:      parseDateList(e.Props, goical.PropExceptionDates),
		RecurrenceID: parseRecurrenceID(e.Props),
		TimeZone:     zone,
		Alarms:       parseAlarms(e.Component),
		Extra:        rawICS(e),
	})
	if err != nil {
		return domain.Event{}, false
	}
	return event, true
}

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

// text returns a property's text value, or an empty string when it is absent or unreadable.
func text(props goical.Props, name string) string {
	v, err := props.Text(name)
	if err != nil {
		return ""
	}
	return v
}

// Encode writes the events as a single VCALENDAR. An empty event set yields a minimal valid calendar.
func (Codec) Encode(events []domain.Event) ([]byte, error) {
	if len(events) == 0 {
		return emptyCalendar, nil
	}
	cal := goical.NewCalendar()
	cal.Props.SetText(goical.PropVersion, "2.0")
	cal.Props.SetText(goical.PropProductID, productID)
	for _, ev := range events {
		cal.Children = append(cal.Children, eventToComponent(ev))
	}
	var buf bytes.Buffer
	if err := goical.NewEncoder(&buf).Encode(cal); err != nil {
		return nil, fmt.Errorf("ics: encode: %w", err)
	}
	return buf.Bytes(), nil
}

// eventToComponent builds a VEVENT for an event. An imported event starts from its preserved original
// VEVENT (Extra) so properties PigeonPost does not model survive the round-trip; a fresh event starts
// empty. The fields the app owns are then overlaid. DTSTAMP is required: an imported event keeps its
// original stamp, a fresh one uses the start time. All-day events use DATE values, timed use DATE-TIME.
func eventToComponent(ev domain.Event) *goical.Component {
	comp := baseComponent(ev)
	uid := ev.UID()
	if uid == "" {
		uid = ev.ID()
	}
	comp.Props.SetText(goical.PropUID, uid)
	if comp.Props.Get(goical.PropDateTimeStamp) == nil {
		comp.Props.SetDateTime(goical.PropDateTimeStamp, ev.Start().UTC())
	}
	comp.Props.SetText(goical.PropSummary, ev.Summary())
	loc := icsLocation(ev.TimeZone())
	setWhen(comp, goical.PropDateTimeStart, ev.Start(), ev.AllDay(), loc)
	if ev.HasEnd() {
		setWhen(comp, goical.PropDateTimeEnd, ev.End(), ev.AllDay(), loc)
		comp.Props.Del(goical.PropDuration)
	} else {
		comp.Props.Del(goical.PropDateTimeEnd)
	}
	setOrDel(comp, goical.PropDescription, ev.Description())
	setOrDel(comp, goical.PropLocation, ev.Location())
	if ev.Recurrence() != "" {
		rrule := goical.NewProp(goical.PropRecurrenceRule)
		rrule.Value = ev.Recurrence()
		comp.Props.Set(rrule)
	} else {
		comp.Props.Del(goical.PropRecurrenceRule)
	}
	setDateList(comp, goical.PropRecurrenceDates, ev.RDates(), ev.AllDay())
	setDateList(comp, goical.PropExceptionDates, ev.ExDates(), ev.AllDay())
	if ev.IsOverride() {
		setWhen(comp, goical.PropRecurrenceID, ev.RecurrenceID(), ev.AllDay(), loc)
	} else {
		comp.Props.Del(goical.PropRecurrenceID)
	}
	setAlarms(comp, ev.Alarms())
	return comp
}

// setDateList overwrites a date-list property (RDATE or EXDATE) with the given occurrence starts as a
// single comma-separated value, or removes it when the list is empty, so an in-app edit replaces rather
// than duplicates any preserved list. Values are DATE for an all-day event and UTC DATE-TIME otherwise.
func setDateList(comp *goical.Component, name string, times []time.Time, allDay bool) {
	comp.Props.Del(name)
	if len(times) == 0 {
		return
	}
	parts := make([]string, len(times))
	for i, t := range times {
		parts[i] = formatWhen(t, allDay)
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

// formatWhen renders a single time in the ICS DATE or UTC DATE-TIME wire form, reusing the library's own
// property setters so the format strings are not duplicated here.
func formatWhen(t time.Time, allDay bool) string {
	prop := goical.NewProp(goical.PropDateTimeStart)
	if allDay {
		prop.SetDate(t)
	} else {
		prop.SetDateTime(t.UTC())
	}
	return prop.Value
}

// baseComponent returns the VEVENT to build on: the preserved original when the event was imported, or
// a new empty VEVENT otherwise. A preserved component that cannot be decoded falls back to empty.
func baseComponent(ev domain.Event) *goical.Component {
	if ev.Extra() != "" {
		if comp := decodeExtra(ev.Extra()); comp != nil {
			return comp
		}
	}
	return goical.NewComponent(goical.CompEvent)
}

// decodeExtra parses the first VEVENT out of a preserved VCALENDAR string, or nil on any failure.
func decodeExtra(raw string) *goical.Component {
	cal, err := goical.NewDecoder(strings.NewReader(raw)).Decode()
	if err != nil {
		return nil
	}
	events := cal.Events()
	if len(events) == 0 {
		return nil
	}
	return events[0].Component
}

// setOrDel writes a text property when the value is non-empty, otherwise removes it, so clearing a
// field in the app clears it in the exported (possibly preserved) component too.
func setOrDel(comp *goical.Component, name, value string) {
	if value != "" {
		comp.Props.SetText(name, value)
		return
	}
	comp.Props.Del(name)
}

// rawICS re-encodes a parsed VEVENT into a standalone VCALENDAR string for preservation. DTSTAMP and
// UID are required by the encoder, so a source missing either is given a synthetic value (the UID is
// overwritten from the domain on export, so a synthetic one here is harmless). An encoding failure
// yields an empty string, degrading to the earlier lossy behaviour rather than failing the import.
func rawICS(e goical.Event) string {
	comp := e.Component
	if comp.Props.Get(goical.PropDateTimeStamp) == nil {
		if start, err := e.DateTimeStart(time.UTC); err == nil {
			comp.Props.SetDateTime(goical.PropDateTimeStamp, start)
		}
	}
	if comp.Props.Get(goical.PropUID) == nil {
		comp.Props.SetText(goical.PropUID, generatedID())
	}
	cal := goical.NewCalendar()
	cal.Props.SetText(goical.PropVersion, "2.0")
	cal.Props.SetText(goical.PropProductID, productID)
	cal.Children = append(cal.Children, comp)
	var buf bytes.Buffer
	if err := goical.NewEncoder(&buf).Encode(cal); err != nil {
		return ""
	}
	return buf.String()
}

// setWhen writes a date or date-time property. A timed value is written in loc: a real zone makes go-ical
// add the TZID parameter and a floating value, while UTC yields a Z value. An all-day value is a DATE.
func setWhen(comp *goical.Component, name string, when time.Time, allDay bool, loc *time.Location) {
	if allDay {
		comp.Props.SetDate(name, when)
		return
	}
	comp.Props.SetDateTime(name, when.In(loc))
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

// generatedID returns a random hex id for an event that carries no UID.
func generatedID() string {
	var b [generatedIDBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "event"
	}
	return hex.EncodeToString(b[:])
}
