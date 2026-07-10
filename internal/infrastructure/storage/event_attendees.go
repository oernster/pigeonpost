package storage

import (
	"encoding/json"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// organizerRow and attendeeRow are the JSON shapes an event's organizer and attendees are stored as, in
// the event table's organizer and attendees columns. JSON keeps the structured attendee fields (role,
// status, rsvp) unambiguous where a delimited string would not.
type organizerRow struct {
	Address    string `json:"address"`
	CommonName string `json:"commonName"`
}

type attendeeRow struct {
	Address    string `json:"address"`
	CommonName string `json:"commonName"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	RSVP       bool   `json:"rsvp"`
}

// encodeOrganizer serialises an event's organizer as JSON, or the empty string when it has none.
func encodeOrganizer(o domain.Organizer) (string, error) {
	if o.IsZero() {
		return "", nil
	}
	b, err := json.Marshal(organizerRow{Address: o.Address().Address(), CommonName: o.CommonName()})
	if err != nil {
		return "", fmt.Errorf("encode organizer: %w", err)
	}
	return string(b), nil
}

// decodeOrganizer parses a stored organizer JSON value back into a domain organizer. The empty string
// decodes to the zero organizer.
func decodeOrganizer(s string) (domain.Organizer, error) {
	if s == "" {
		return domain.Organizer{}, nil
	}
	var row organizerRow
	if err := json.Unmarshal([]byte(s), &row); err != nil {
		return domain.Organizer{}, fmt.Errorf("decode organizer: %w", err)
	}
	addr, err := domain.NewEmailAddress("", row.Address)
	if err != nil {
		return domain.Organizer{}, fmt.Errorf("decode organizer address: %w", err)
	}
	return domain.NewOrganizer(addr, row.CommonName)
}

// encodeAttendees serialises an event's attendees as a JSON array, or the empty string when it has none.
func encodeAttendees(attendees []domain.Attendee) (string, error) {
	if len(attendees) == 0 {
		return "", nil
	}
	rows := make([]attendeeRow, len(attendees))
	for i, a := range attendees {
		rows[i] = attendeeRow{
			Address:    a.Address().Address(),
			CommonName: a.CommonName(),
			Role:       string(a.Role()),
			Status:     string(a.Status()),
			RSVP:       a.RSVP(),
		}
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return "", fmt.Errorf("encode attendees: %w", err)
	}
	return string(b), nil
}

// decodeAttendees parses a stored attendees JSON array back into domain attendees. The empty string
// decodes to no attendees.
func decodeAttendees(s string) ([]domain.Attendee, error) {
	if s == "" {
		return nil, nil
	}
	var rows []attendeeRow
	if err := json.Unmarshal([]byte(s), &rows); err != nil {
		return nil, fmt.Errorf("decode attendees: %w", err)
	}
	out := make([]domain.Attendee, 0, len(rows))
	for _, row := range rows {
		addr, err := domain.NewEmailAddress("", row.Address)
		if err != nil {
			return nil, fmt.Errorf("decode attendee address: %w", err)
		}
		role, err := domain.ParseRole(row.Role)
		if err != nil {
			return nil, fmt.Errorf("decode attendee role: %w", err)
		}
		status, err := domain.ParseParticipationStatus(row.Status)
		if err != nil {
			return nil, fmt.Errorf("decode attendee status: %w", err)
		}
		att, err := domain.NewAttendee(domain.AttendeeInput{
			Address: addr, CommonName: row.CommonName, Role: role, Status: status, RSVP: row.RSVP,
		})
		if err != nil {
			return nil, fmt.Errorf("decode attendee: %w", err)
		}
		out = append(out, att)
	}
	return out, nil
}
