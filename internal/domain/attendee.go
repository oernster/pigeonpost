package domain

import "strings"

// Role is an attendee's part in a meeting, matching the RFC 5545 ROLE parameter. An absent role means
// REQ-PARTICIPANT, the value a required attendee carries.
type Role string

// The recognised attendee roles. REQ-PARTICIPANT is the default when none is stated.
const (
	RoleChair          Role = "CHAIR"
	RoleRequired       Role = "REQ-PARTICIPANT"
	RoleOptional       Role = "OPT-PARTICIPANT"
	RoleNonParticipant Role = "NON-PARTICIPANT"
)

// ParseRole normalises and validates a ROLE parameter value. An empty value yields the REQ-PARTICIPANT
// default; an unrecognised value is rejected.
func ParseRole(s string) (Role, error) {
	v := Role(strings.ToUpper(strings.TrimSpace(s)))
	switch v {
	case "":
		return RoleRequired, nil
	case RoleChair, RoleRequired, RoleOptional, RoleNonParticipant:
		return v, nil
	}
	return "", ErrInvalidRole
}

// ParticipationStatus is an attendee's reply state, matching the RFC 5545 PARTSTAT parameter for a
// VEVENT. An absent status means NEEDS-ACTION, the state a freshly invited attendee holds.
type ParticipationStatus string

// The recognised participation statuses for a meeting attendee. NEEDS-ACTION is the default before the
// attendee has responded.
const (
	PartStatNeedsAction ParticipationStatus = "NEEDS-ACTION"
	PartStatAccepted    ParticipationStatus = "ACCEPTED"
	PartStatDeclined    ParticipationStatus = "DECLINED"
	PartStatTentative   ParticipationStatus = "TENTATIVE"
	PartStatDelegated   ParticipationStatus = "DELEGATED"
)

// ParseParticipationStatus normalises and validates a PARTSTAT parameter value. An empty value yields
// the NEEDS-ACTION default; an unrecognised value is rejected so the codec can decide how to be lenient.
func ParseParticipationStatus(s string) (ParticipationStatus, error) {
	v := ParticipationStatus(strings.ToUpper(strings.TrimSpace(s)))
	switch v {
	case "":
		return PartStatNeedsAction, nil
	case PartStatNeedsAction, PartStatAccepted, PartStatDeclined, PartStatTentative, PartStatDelegated:
		return v, nil
	}
	return "", ErrInvalidParticipationStatus
}

// Organizer is the party that owns a meeting and receives the replies to it, matching the RFC 5545
// ORGANIZER property: a validated address with an optional common name. It is immutable once created.
type Organizer struct {
	address    EmailAddress
	commonName string
}

// NewOrganizer builds an organizer from a validated address and an optional common name. A zero address
// is rejected: an organizer without an address cannot be replied to.
func NewOrganizer(address EmailAddress, commonName string) (Organizer, error) {
	if address.IsZero() {
		return Organizer{}, ErrEmptyOrganizerAddress
	}
	return Organizer{address: address, commonName: strings.TrimSpace(commonName)}, nil
}

// Address returns the organizer's validated address.
func (o Organizer) Address() EmailAddress { return o.address }

// CommonName returns the optional display name, which may be empty.
func (o Organizer) CommonName() string { return o.commonName }

// IsZero reports whether this is the empty organizer, the value an event carries when it has no
// organizer.
func (o Organizer) IsZero() bool { return o.address.IsZero() }

// AttendeeInput carries the fields for constructing an Attendee. Address is required; Role and Status
// default to REQ-PARTICIPANT and NEEDS-ACTION when left as their zero values.
type AttendeeInput struct {
	Address    EmailAddress
	CommonName string
	Role       Role
	Status     ParticipationStatus
	RSVP       bool
}

// Attendee is one invited party on a meeting, matching an RFC 5545 ATTENDEE property with its ROLE,
// PARTSTAT and RSVP parameters. It is immutable once created.
type Attendee struct {
	address    EmailAddress
	commonName string
	role       Role
	status     ParticipationStatus
	rsvp       bool
}

// NewAttendee builds an attendee, rejecting a zero address and filling the Role and Status defaults when
// they are left empty.
func NewAttendee(in AttendeeInput) (Attendee, error) {
	if in.Address.IsZero() {
		return Attendee{}, ErrEmptyAttendeeAddress
	}
	role := in.Role
	if role == "" {
		role = RoleRequired
	}
	status := in.Status
	if status == "" {
		status = PartStatNeedsAction
	}
	return Attendee{
		address:    in.Address,
		commonName: strings.TrimSpace(in.CommonName),
		role:       role,
		status:     status,
		rsvp:       in.RSVP,
	}, nil
}

// Address returns the attendee's validated address.
func (a Attendee) Address() EmailAddress { return a.address }

// CommonName returns the optional display name, which may be empty.
func (a Attendee) CommonName() string { return a.commonName }

// Role returns the attendee's part in the meeting.
func (a Attendee) Role() Role { return a.role }

// Status returns the attendee's reply state.
func (a Attendee) Status() ParticipationStatus { return a.status }

// RSVP reports whether the organizer has requested a reply from this attendee.
func (a Attendee) RSVP() bool { return a.rsvp }

// WithStatus returns a copy of the attendee with its reply state replaced, used when an incoming REPLY
// updates one attendee's PARTSTAT. The attendee stays immutable: the receiver is unchanged.
func (a Attendee) WithStatus(status ParticipationStatus) Attendee {
	a.status = status
	return a
}
