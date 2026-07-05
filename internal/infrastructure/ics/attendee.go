package ics

import (
	"strings"

	goical "github.com/emersion/go-ical"

	"github.com/oernster/pigeonpost/internal/domain"
)

// mailtoPrefix is the scheme an ICS CAL-ADDRESS value carries (ORGANIZER and ATTENDEE hold a
// "mailto:user@host" URI). It is stripped on decode and prepended on encode.
const mailtoPrefix = "mailto:"

// rsvpTrue is the ICS RSVP parameter value that requests a reply from an attendee.
const rsvpTrue = "TRUE"

// parseOrganizer reads the ORGANIZER property into a domain organizer, returning the zero organizer when
// the property is absent or its address cannot be represented, so a malformed organizer cannot fail the
// whole import.
func parseOrganizer(props goical.Props) domain.Organizer {
	prop := props.Get(goical.PropOrganizer)
	if prop == nil {
		return domain.Organizer{}
	}
	addr, ok := calAddress(prop.Value)
	if !ok {
		return domain.Organizer{}
	}
	organizer, err := domain.NewOrganizer(addr, prop.Params.Get(goical.ParamCommonName))
	if err != nil {
		return domain.Organizer{}
	}
	return organizer
}

// parseAttendees reads every ATTENDEE property into domain attendees. An unrecognised ROLE or PARTSTAT
// falls back to its default rather than failing, and an attendee whose address cannot be represented is
// skipped, so a malformed line cannot fail the whole import.
func parseAttendees(props goical.Props) []domain.Attendee {
	var out []domain.Attendee
	for _, prop := range props.Values(goical.PropAttendee) {
		addr, ok := calAddress(prop.Value)
		if !ok {
			continue
		}
		role, err := domain.ParseRole(prop.Params.Get(goical.ParamRole))
		if err != nil {
			role = domain.RoleRequired
		}
		status, err := domain.ParseParticipationStatus(prop.Params.Get(goical.ParamParticipationStatus))
		if err != nil {
			status = domain.PartStatNeedsAction
		}
		attendee, err := domain.NewAttendee(domain.AttendeeInput{
			Address:    addr,
			CommonName: prop.Params.Get(goical.ParamCommonName),
			Role:       role,
			Status:     status,
			RSVP:       parseRSVP(prop.Params.Get(goical.ParamRSVP)),
		})
		if err != nil {
			continue
		}
		out = append(out, attendee)
	}
	return out
}

// calAddress strips the optional mailto: scheme from a CAL-ADDRESS value and validates the remainder as
// an email address. The bool is false when the value cannot form a valid address.
func calAddress(value string) (domain.EmailAddress, bool) {
	raw := strings.TrimSpace(value)
	if len(raw) >= len(mailtoPrefix) && strings.EqualFold(raw[:len(mailtoPrefix)], mailtoPrefix) {
		raw = raw[len(mailtoPrefix):]
	}
	addr, err := domain.NewEmailAddress("", raw)
	if err != nil {
		return domain.EmailAddress{}, false
	}
	return addr, true
}

// parseRSVP reads the RSVP parameter, which is TRUE when the organizer wants a reply and absent or FALSE
// otherwise.
func parseRSVP(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), rsvpTrue)
}

// setOrganizer overwrites the ORGANIZER property from the event's organizer, or removes it when the event
// has none, so clearing the organizer in the app clears it in the exported component too.
func setOrganizer(comp *goical.Component, organizer domain.Organizer) {
	comp.Props.Del(goical.PropOrganizer)
	if organizer.IsZero() {
		return
	}
	prop := goical.NewProp(goical.PropOrganizer)
	prop.Value = mailtoPrefix + organizer.Address().Address()
	if cn := organizer.CommonName(); cn != "" {
		prop.Params.Set(goical.ParamCommonName, cn)
	}
	comp.Props.Set(prop)
}

// setAttendees replaces every ATTENDEE property with one line per domain attendee, writing the CN, ROLE
// and PARTSTAT parameters and RSVP only when requested. Removing then re-adding means an in-app edit
// replaces rather than duplicates any preserved attendee list.
func setAttendees(comp *goical.Component, attendees []domain.Attendee) {
	comp.Props.Del(goical.PropAttendee)
	for _, a := range attendees {
		prop := goical.NewProp(goical.PropAttendee)
		prop.Value = mailtoPrefix + a.Address().Address()
		if cn := a.CommonName(); cn != "" {
			prop.Params.Set(goical.ParamCommonName, cn)
		}
		prop.Params.Set(goical.ParamRole, string(a.Role()))
		prop.Params.Set(goical.ParamParticipationStatus, string(a.Status()))
		if a.RSVP() {
			prop.Params.Set(goical.ParamRSVP, rsvpTrue)
		}
		comp.Props.Add(prop)
	}
}
