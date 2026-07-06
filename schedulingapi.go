package main

import (
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// OrganizerDTO is the JSON-serialisable view of a meeting organizer.
type OrganizerDTO struct {
	Address    string `json:"address"`
	CommonName string `json:"commonName"`
}

// AttendeeDTO is the JSON-serialisable view of a meeting attendee, including their role, reply status
// and whether the organizer requested a reply.
type AttendeeDTO struct {
	Address    string `json:"address"`
	CommonName string `json:"commonName"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	Rsvp       bool   `json:"rsvp"`
}

// InvitationDTO is the JSON-serialisable view of an incoming meeting invitation. Method is the iTIP
// method (REQUEST, REPLY, CANCEL or PUBLISH); Me is the recipient account address; MyStatus is the
// recipient's own current response, so the reader can highlight the chosen action.
type InvitationDTO struct {
	Method    string        `json:"method"`
	Event     EventDTO      `json:"event"`
	Me        string        `json:"me"`
	MyStatus  string        `json:"myStatus"`
	Organizer OrganizerDTO  `json:"organizer"`
	Attendees []AttendeeDTO `json:"attendees"`
}

// GetInvitation resolves the meeting invitation carried by a message for display, including the
// recipient's own current response.
func (a *App) GetInvitation(messageID string) (InvitationDTO, error) {
	inv, err := a.scheduling.Invitation(a.ctx, messageID)
	if err != nil {
		return InvitationDTO{}, err
	}
	return toInvitationDTO(inv), nil
}

// RespondToInvitation records the recipient's answer to a meeting request: it saves the meeting with the
// chosen status and sends a REPLY to the organizer. The status is an ICS PARTSTAT value (ACCEPTED,
// DECLINED or TENTATIVE).
func (a *App) RespondToInvitation(messageID, status string) error {
	partStat, err := domain.ParseParticipationStatus(status)
	if err != nil {
		return err
	}
	return a.scheduling.Respond(a.ctx, messageID, partStat)
}

// RemoveCancelledMeeting removes the meeting a CANCEL message withdraws from the calendar.
func (a *App) RemoveCancelledMeeting(messageID string) error {
	return a.scheduling.ApplyCancellation(a.ctx, messageID)
}

// ApplyMeetingReply folds an incoming REPLY into the organizer's stored meeting, updating the
// responding attendee's status.
func (a *App) ApplyMeetingReply(messageID string) error {
	return a.scheduling.ApplyReply(a.ctx, messageID)
}

// SendMeetingRequest emails a meeting REQUEST to the attendees of the event identified by id, inviting
// them. The event must already carry its organizer and attendee list. It logs each step so a send that
// does not reach the recipients can be diagnosed from the app log rather than guessed at.
func (a *App) SendMeetingRequest(accountID, eventID string) error {
	runtime.LogInfof(a.ctx, "meeting invite: send REQUEST account=%q event=%q", accountID, eventID)
	event, err := a.calendar.GetEvent(a.ctx, eventID)
	if err != nil {
		runtime.LogErrorf(a.ctx, "meeting invite: load event %q failed: %v", eventID, err)
		return err
	}
	if len(event.Attendees()) == 0 {
		runtime.LogErrorf(a.ctx, "meeting invite: event %q has no attendees to invite", eventID)
		return fmt.Errorf("this meeting has no attendees to invite")
	}
	runtime.LogInfof(a.ctx, "meeting invite: event %q has %d attendee(s), sending", eventID, len(event.Attendees()))
	if err := a.scheduling.SendRequest(a.ctx, accountID, []domain.Event{event}); err != nil {
		runtime.LogErrorf(a.ctx, "meeting invite: send REQUEST failed: %v", err)
		return err
	}
	runtime.LogInfof(a.ctx, "meeting invite: REQUEST for event %q sent", eventID)
	return nil
}

// SendMeetingCancel emails a meeting CANCEL to the attendees of the event identified by id, withdrawing
// the meeting.
func (a *App) SendMeetingCancel(accountID, eventID string) error {
	runtime.LogInfof(a.ctx, "meeting invite: send CANCEL account=%q event=%q", accountID, eventID)
	event, err := a.calendar.GetEvent(a.ctx, eventID)
	if err != nil {
		runtime.LogErrorf(a.ctx, "meeting invite: load event %q failed: %v", eventID, err)
		return err
	}
	if err := a.scheduling.SendCancel(a.ctx, accountID, []domain.Event{event}); err != nil {
		runtime.LogErrorf(a.ctx, "meeting invite: send CANCEL failed: %v", err)
		return err
	}
	runtime.LogInfof(a.ctx, "meeting invite: CANCEL for event %q sent", eventID)
	return nil
}

// toInvitationDTO maps an application Invitation to its DTO.
func toInvitationDTO(inv application.Invitation) InvitationDTO {
	organizer := inv.Event.Organizer()
	return InvitationDTO{
		Method:    string(inv.Method),
		Event:     toEventDTO(inv.Event),
		Me:        inv.Me.Address(),
		MyStatus:  string(inv.MyStatus),
		Organizer: OrganizerDTO{Address: organizer.Address().Address(), CommonName: organizer.CommonName()},
		Attendees: toAttendeeDTOs(inv.Event.Attendees()),
	}
}

// toAttendeeDTOs maps domain attendees to their DTOs.
func toAttendeeDTOs(attendees []domain.Attendee) []AttendeeDTO {
	out := make([]AttendeeDTO, 0, len(attendees))
	for _, at := range attendees {
		out = append(out, AttendeeDTO{
			Address:    at.Address().Address(),
			CommonName: at.CommonName(),
			Role:       string(at.Role()),
			Status:     string(at.Status()),
			Rsvp:       at.RSVP(),
		})
	}
	return out
}
