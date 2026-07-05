package ics

import (
	"bytes"
	"strings"
	"time"

	goical "github.com/emersion/go-ical"

	"github.com/oernster/pigeonpost/internal/domain"
)

// passthroughFromComponent preserves a VTODO or VJOURNAL verbatim as a passthrough value. Any other
// component (VEVENT, VTIMEZONE, VFREEBUSY) returns false: events are handled separately and the rest are
// not preserved.
func passthroughFromComponent(comp *goical.Component) (domain.CalendarPassthrough, bool) {
	if comp.Name != goical.CompToDo && comp.Name != goical.CompJournal {
		return domain.CalendarPassthrough{}, false
	}
	uid := text(comp.Props, goical.PropUID)
	if uid == "" {
		uid = generatedID()
	}
	raw := rawComponent(comp, uid)
	if raw == "" {
		return domain.CalendarPassthrough{}, false
	}
	p, err := domain.NewCalendarPassthrough(uid, comp.Name, raw)
	if err != nil {
		return domain.CalendarPassthrough{}, false
	}
	return p, true
}

// rawComponent re-encodes a component as a standalone VCALENDAR string, filling in the UID and DTSTAMP
// the encoder requires when the source omits them. It returns an empty string on an encoding failure.
func rawComponent(comp *goical.Component, uid string) string {
	if comp.Props.Get(goical.PropUID) == nil {
		comp.Props.SetText(goical.PropUID, uid)
	}
	if comp.Props.Get(goical.PropDateTimeStamp) == nil {
		comp.Props.SetDateTime(goical.PropDateTimeStamp, time.Now().UTC())
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

// decodeComponent parses the first VTODO or VJOURNAL out of a preserved VCALENDAR string, or nil on any
// failure, so a corrupt preserved component is dropped rather than failing the whole export.
func decodeComponent(raw string) *goical.Component {
	cal, err := goical.NewDecoder(strings.NewReader(raw)).Decode()
	if err != nil {
		return nil
	}
	for _, child := range cal.Children {
		if child.Name == goical.CompToDo || child.Name == goical.CompJournal {
			return child
		}
	}
	return nil
}
