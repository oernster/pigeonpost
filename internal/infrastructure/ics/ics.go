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
	if startProp := e.Props.Get(goical.PropDateTimeStart); startProp != nil {
		allDay = startProp.ValueType() == goical.ValueDate
	}
	recurrence := ""
	if rrule := e.Props.Get(goical.PropRecurrenceRule); rrule != nil {
		recurrence = rrule.Value
	}
	event, err := domain.NewEvent(domain.EventInput{
		ID:          id,
		UID:         uid,
		Summary:     summary,
		Description: text(e.Props, goical.PropDescription),
		Location:    text(e.Props, goical.PropLocation),
		Start:       start,
		End:         end,
		AllDay:      allDay,
		Recurrence:  recurrence,
	})
	if err != nil {
		return domain.Event{}, false
	}
	return event, true
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

// eventToComponent builds a VEVENT from an event. DTSTAMP is required, so the start time doubles as a
// stable stamp; all-day events use DATE values, timed events use DATE-TIME.
func eventToComponent(ev domain.Event) *goical.Component {
	comp := goical.NewComponent(goical.CompEvent)
	uid := ev.UID()
	if uid == "" {
		uid = ev.ID()
	}
	comp.Props.SetText(goical.PropUID, uid)
	comp.Props.SetDateTime(goical.PropDateTimeStamp, ev.Start().UTC())
	comp.Props.SetText(goical.PropSummary, ev.Summary())
	setWhen(comp, goical.PropDateTimeStart, ev.Start(), ev.AllDay())
	if ev.HasEnd() {
		setWhen(comp, goical.PropDateTimeEnd, ev.End(), ev.AllDay())
	}
	setIfPresent(comp, goical.PropDescription, ev.Description())
	setIfPresent(comp, goical.PropLocation, ev.Location())
	if ev.Recurrence() != "" {
		rrule := goical.NewProp(goical.PropRecurrenceRule)
		rrule.Value = ev.Recurrence()
		comp.Props.Set(rrule)
	}
	return comp
}

// setWhen writes a date or date-time property depending on whether the event is all-day.
func setWhen(comp *goical.Component, name string, when time.Time, allDay bool) {
	if allDay {
		comp.Props.SetDate(name, when)
		return
	}
	comp.Props.SetDateTime(name, when)
}

// setIfPresent writes a text property only when the value is non-empty.
func setIfPresent(comp *goical.Component, name, value string) {
	if value != "" {
		comp.Props.SetText(name, value)
	}
}

// generatedID returns a random hex id for an event that carries no UID.
func generatedID() string {
	var b [generatedIDBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "event"
	}
	return hex.EncodeToString(b[:])
}
