package ics

import (
	"bytes"

	goical "github.com/emersion/go-ical"
)

// icalVersion is the iCalendar version every VCALENDAR PigeonPost writes declares (the required VERSION
// property); RFC 5545 defines "2.0".
const icalVersion = "2.0"

// newICSCalendar returns an empty VCALENDAR carrying the required VERSION and PRODID properties, the
// header every calendar PigeonPost encodes shares. Callers append their children (and a METHOD for a
// scheduling payload) before encoding.
func newICSCalendar() *goical.Calendar {
	cal := goical.NewCalendar()
	cal.Props.SetText(goical.PropVersion, icalVersion)
	cal.Props.SetText(goical.PropProductID, productID)
	return cal
}

// encodeStandalone wraps a single component in a fresh VCALENDAR and encodes it to a string, the shared
// tail of the passthrough and preserved-event round-trips. It returns an empty string on an encoding
// failure, so a component that cannot be re-encoded is dropped rather than failing the whole operation.
func encodeStandalone(comp *goical.Component) string {
	cal := newICSCalendar()
	cal.Children = append(cal.Children, comp)
	var buf bytes.Buffer
	if err := goical.NewEncoder(&buf).Encode(cal); err != nil {
		return ""
	}
	return buf.String()
}
