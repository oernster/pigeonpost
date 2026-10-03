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
	"strconv"
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
// rejects a childless calendar). Its VERSION and PRODID match newICSCalendar's (see encode.go).
var emptyCalendar = []byte("BEGIN:VCALENDAR\r\nVERSION:" + icalVersion + "\r\nPRODID:" + productID + "\r\nEND:VCALENDAR\r\n")

// Codec is the iCalendar implementation of the application CalendarCodec port.
type Codec struct{}

// New constructs an ICS codec.
func New() Codec { return Codec{} }

// Decode parses the VEVENTs from one or more VCALENDARs into events; it also preserves any VTODO or VJOURNAL
// components as passthrough so they survive a round-trip. An event's id is derived from its UID and
// RECURRENCE-ID (domain.EventIDFor), so a re-import updates the same record while a series master and its
// overrides stay distinct; a component's UID becomes its id; one without a UID is given a generated
// id. An event that cannot form a valid domain value (no start or an end before its start) is skipped
// rather than failing the import.
func (Codec) Decode(data []byte) ([]domain.Event, []domain.CalendarPassthrough, error) {
	dec := goical.NewDecoder(bytes.NewReader(data))
	var events []domain.Event
	var passthrough []domain.CalendarPassthrough
	for {
		cal, err := dec.Decode()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("ics: decode: %w", err)
		}
		events = append(events, calendarEvents(cal)...)
		for _, child := range cal.Children {
			if p, ok := passthroughFromComponent(child); ok {
				passthrough = append(passthrough, p)
			}
		}
	}
	return events, passthrough, nil
}

// calendarEvents decodes the usable VEVENTs of one VCALENDAR, resolving their TZIDs against the
// VTIMEZONE definitions that calendar itself carries.
func calendarEvents(cal *goical.Calendar) []domain.Event {
	zones := customZones(cal)
	var events []domain.Event
	for _, e := range cal.Events() {
		if event, ok := eventFromICS(e, zones); ok {
			events = append(events, event)
		}
	}
	return events
}

// eventFromICS maps a parsed VEVENT into a domain event. The bool is false for an event that cannot be
// represented (no usable start or a validation failure), which the caller skips.
func eventFromICS(e goical.Event, zones map[string]customZone) (domain.Event, bool) {
	// Rewrite non-IANA zone names (Outlook display names, Windows names, custom zones defined in the file)
	// before any time is read, so a non-IANA TZID no longer fails the whole event; a zone that cannot be
	// resolved at all degrades to floating rather than dropping the event.
	normalizeZones(e, zones)
	start, err := e.DateTimeStart(time.UTC)
	if err != nil || start.IsZero() {
		return domain.Event{}, false
	}
	end := eventEnd(e)
	recurrenceID := parseRecurrenceID(e.Props)
	uid := text(e.Props, goical.PropUID)
	if uid == "" {
		uid = generatedID()
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
		ID:           domain.EventIDFor(uid, recurrenceID),
		UID:          uid,
		Summary:      summary,
		Description:  eventDescription(e.Props),
		Location:     descriptionText(text(e.Props, goical.PropLocation)),
		Category:     primaryCategory(e.Props),
		Start:        start,
		End:          end,
		AllDay:       allDay,
		Recurrence:   recurrence,
		RDates:       parseDateList(e.Props, goical.PropRecurrenceDates),
		ExDates:      parseDateList(e.Props, goical.PropExceptionDates),
		RecurrenceID: recurrenceID,
		Sequence:     parseSequence(e.Props),
		TimeZone:     zone,
		Alarms:       parseAlarms(e.Component, eventLength(start, end)),
		Organizer:    parseOrganizer(e.Props),
		Attendees:    parseAttendees(e.Props),
		Extra:        rawICS(e),
	})
	if err != nil {
		return domain.Event{}, false
	}
	return event, true
}

// parseSequence reads the organiser's revision number (SEQUENCE). An absent or malformed value reads as
// zero (the RFC 5545 default); a negative one is clamped to zero, so a sloppy revision number never
// costs the user the whole event.
func parseSequence(props goical.Props) int {
	prop := props.Get(goical.PropSequence)
	if prop == nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(prop.Value))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// eventLength is the span from start to end, zero for an event without an end (an end-anchored alarm on
// such an event is then anchored to its start, as RFC 5545 gives it no end to anchor to).
func eventLength(start, end time.Time) time.Duration {
	if end.IsZero() {
		return 0
	}
	return end.Sub(start)
}

// text returns a property's text value (empty when it is absent or unreadable).
func text(props goical.Props, name string) string {
	v, err := props.Text(name)
	if err != nil {
		return ""
	}
	return v
}

// Encode writes the events and preserved passthrough components as a single VCALENDAR. An empty set
// yields a minimal valid calendar.
func (Codec) Encode(events []domain.Event, passthrough []domain.CalendarPassthrough) ([]byte, error) {
	if len(events) == 0 && len(passthrough) == 0 {
		return emptyCalendar, nil
	}
	cal := newICSCalendar()
	// VTIMEZONE definitions come first so the events' TZID references resolve within the file itself.
	cal.Children = append(cal.Children, timezoneComponents(events)...)
	for _, ev := range events {
		cal.Children = append(cal.Children, eventToComponent(ev))
	}
	for _, p := range passthrough {
		if comp := decodeComponent(p.Raw()); comp != nil {
			cal.Children = append(cal.Children, comp)
		}
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
	// The form is decided from the preserved DTSTART before it is overwritten below.
	form := exportForm(ev.TimeZone(), comp)
	uid := ev.UID()
	if uid == "" {
		uid = ev.ID()
	}
	comp.Props.SetText(goical.PropUID, uid)
	if comp.Props.Get(goical.PropDateTimeStamp) == nil {
		comp.Props.SetDateTime(goical.PropDateTimeStamp, ev.Start().UTC())
	}
	comp.Props.SetText(goical.PropSummary, ev.Summary())
	form.setWhen(comp, goical.PropDateTimeStart, ev.Start(), ev.AllDay())
	if ev.HasEnd() {
		form.setWhen(comp, goical.PropDateTimeEnd, ev.End(), ev.AllDay())
		comp.Props.Del(goical.PropDuration)
	} else {
		comp.Props.Del(goical.PropDateTimeEnd)
	}
	setOrDel(comp, goical.PropDescription, ev.Description())
	setOrDel(comp, goical.PropLocation, ev.Location())
	setCategory(comp, ev.Category())
	setSequence(comp, ev.Sequence())
	if ev.Recurrence() != "" {
		rrule := goical.NewProp(goical.PropRecurrenceRule)
		rrule.Value = ev.Recurrence()
		comp.Props.Set(rrule)
	} else {
		comp.Props.Del(goical.PropRecurrenceRule)
	}
	form.setDateList(comp, goical.PropRecurrenceDates, ev.RDates(), ev.AllDay())
	form.setDateList(comp, goical.PropExceptionDates, ev.ExDates(), ev.AllDay())
	if ev.IsOverride() {
		form.setWhen(comp, goical.PropRecurrenceID, ev.RecurrenceID(), ev.AllDay())
	} else {
		comp.Props.Del(goical.PropRecurrenceID)
	}
	setOrganizer(comp, ev.Organizer())
	setAttendees(comp, ev.Attendees())
	setAlarms(comp, ev.Alarms(), eventLength(ev.Start(), ev.End()))
	return comp
}

// setSequence writes SEQUENCE when the event has been revised; otherwise it removes the property, so the model
// stays authoritative over a preserved (possibly malformed) original value; zero is the RFC 5545 default.
func setSequence(comp *goical.Component, sequence int) {
	if sequence == 0 {
		comp.Props.Del(goical.PropSequence)
		return
	}
	prop := goical.NewProp(goical.PropSequence)
	prop.SetValueType(goical.ValueInt)
	prop.Value = strconv.Itoa(sequence)
	comp.Props.Set(prop)
}

// baseComponent returns the VEVENT to build on: the preserved original when the event was imported;
// otherwise a new empty VEVENT. A preserved component that cannot be decoded falls back to empty.
func baseComponent(ev domain.Event) *goical.Component {
	if ev.Extra() != "" {
		if comp := decodeExtra(ev.Extra()); comp != nil {
			return comp
		}
	}
	return goical.NewComponent(goical.CompEvent)
}

// decodeExtra parses the first VEVENT out of a preserved VCALENDAR string (nil on any failure).
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
	return encodeStandalone(comp)
}

// generatedID returns a random hex id for an event that carries no UID.
func generatedID() string {
	var b [generatedIDBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "event"
	}
	return hex.EncodeToString(b[:])
}
