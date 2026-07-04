package main

import (
	"fmt"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/csv"
	"github.com/oernster/pigeonpost/internal/infrastructure/vcard"
)

// ContactEmailDTO is a labelled email address on a contact.
type ContactEmailDTO struct {
	Label   string `json:"label"`
	Address string `json:"address"`
}

// ContactPhoneDTO is a labelled phone number on a contact.
type ContactPhoneDTO struct {
	Label  string `json:"label"`
	Number string `json:"number"`
}

// ContactDTO is the JSON-serialisable view of an address-book contact.
type ContactDTO struct {
	ID            string            `json:"id"`
	UID           string            `json:"uid"`
	FormattedName string            `json:"formattedName"`
	GivenName     string            `json:"givenName"`
	FamilyName    string            `json:"familyName"`
	Organization  string            `json:"organization"`
	Title         string            `json:"title"`
	Note          string            `json:"note"`
	Emails        []ContactEmailDTO `json:"emails"`
	Phones        []ContactPhoneDTO `json:"phones"`
}

// ContactRequest is the front-end payload for creating or updating a contact. An empty id means a new
// contact.
type ContactRequest struct {
	ID            string            `json:"id"`
	UID           string            `json:"uid"`
	FormattedName string            `json:"formattedName"`
	GivenName     string            `json:"givenName"`
	FamilyName    string            `json:"familyName"`
	Organization  string            `json:"organization"`
	Title         string            `json:"title"`
	Note          string            `json:"note"`
	Emails        []ContactEmailDTO `json:"emails"`
	Phones        []ContactPhoneDTO `json:"phones"`
}

// ContactGroupDTO is the JSON-serialisable view of a contact group (mailing list).
type ContactGroupDTO struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

// ContactGroupRequest is the front-end payload for creating or updating a group. An empty id means a
// new group.
type ContactGroupRequest struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

// ListContacts returns every contact.
func (a *App) ListContacts() ([]ContactDTO, error) {
	contacts, err := a.contacts.ListContacts(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ContactDTO, 0, len(contacts))
	for _, c := range contacts {
		out = append(out, toContactDTO(c))
	}
	return out, nil
}

// GetContact returns a single contact by id.
func (a *App) GetContact(id string) (ContactDTO, error) {
	c, err := a.contacts.GetContact(a.ctx, id)
	if err != nil {
		return ContactDTO{}, err
	}
	return toContactDTO(c), nil
}

// SaveContact creates or updates a contact.
func (a *App) SaveContact(req ContactRequest) error {
	return a.contacts.SaveContact(a.ctx, application.ContactInput{
		ID:            req.ID,
		UID:           req.UID,
		FormattedName: req.FormattedName,
		GivenName:     req.GivenName,
		FamilyName:    req.FamilyName,
		Organization:  req.Organization,
		Title:         req.Title,
		Note:          req.Note,
		Emails:        toEmailInputs(req.Emails),
		Phones:        toPhoneInputs(req.Phones),
	})
}

// DeleteContact removes a contact by id.
func (a *App) DeleteContact(id string) error {
	return a.contacts.DeleteContact(a.ctx, id)
}

// ListContactGroups returns every contact group.
func (a *App) ListContactGroups() ([]ContactGroupDTO, error) {
	groups, err := a.contacts.ListGroups(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ContactGroupDTO, 0, len(groups))
	for _, g := range groups {
		out = append(out, ContactGroupDTO{ID: g.ID(), Name: g.Name(), Members: g.Members()})
	}
	return out, nil
}

// SaveContactGroup creates or updates a contact group.
func (a *App) SaveContactGroup(req ContactGroupRequest) error {
	return a.contacts.SaveGroup(a.ctx, application.ContactGroupInput{
		ID: req.ID, Name: req.Name, Members: req.Members,
	})
}

// DeleteContactGroup removes a contact group by id.
func (a *App) DeleteContactGroup(id string) error {
	return a.contacts.DeleteGroup(a.ctx, id)
}

// ImportContacts decodes the given file content in the named format ("vcard" or "csv") and saves the
// contacts, returning the number imported.
func (a *App) ImportContacts(format, content string) (int, error) {
	codec, err := contactCodec(format)
	if err != nil {
		return 0, err
	}
	return a.contacts.ImportContacts(a.ctx, codec, []byte(content))
}

// ExportContacts encodes every contact in the named format ("vcard" or "csv") and returns the file
// content.
func (a *App) ExportContacts(format string) (string, error) {
	codec, err := contactCodec(format)
	if err != nil {
		return "", err
	}
	data, err := a.contacts.ExportContacts(a.ctx, codec)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// contactCodec selects the import/export codec for a format token.
func contactCodec(format string) (application.ContactCodec, error) {
	switch format {
	case "vcard":
		return vcard.New(), nil
	case "csv":
		return csv.New(), nil
	default:
		return nil, fmt.Errorf("unknown contact format %q", format)
	}
}

// toContactDTO maps a domain contact to its DTO.
func toContactDTO(c domain.Contact) ContactDTO {
	emails := make([]ContactEmailDTO, 0, len(c.Emails()))
	for _, e := range c.Emails() {
		emails = append(emails, ContactEmailDTO{Label: e.Label(), Address: e.Address().Address()})
	}
	phones := make([]ContactPhoneDTO, 0, len(c.Phones()))
	for _, p := range c.Phones() {
		phones = append(phones, ContactPhoneDTO{Label: p.Label(), Number: p.Number()})
	}
	return ContactDTO{
		ID:            c.ID(),
		UID:           c.UID(),
		FormattedName: c.FormattedName(),
		GivenName:     c.GivenName(),
		FamilyName:    c.FamilyName(),
		Organization:  c.Organization(),
		Title:         c.Title(),
		Note:          c.Note(),
		Emails:        emails,
		Phones:        phones,
	}
}

// toEmailInputs maps email DTOs to the application input type.
func toEmailInputs(in []ContactEmailDTO) []application.ContactEmailInput {
	out := make([]application.ContactEmailInput, 0, len(in))
	for _, e := range in {
		out = append(out, application.ContactEmailInput{Label: e.Label, Address: e.Address})
	}
	return out
}

// toPhoneInputs maps phone DTOs to the application input type.
func toPhoneInputs(in []ContactPhoneDTO) []application.ContactPhoneInput {
	out := make([]application.ContactPhoneInput, 0, len(in))
	for _, p := range in {
		out = append(out, application.ContactPhoneInput{Label: p.Label, Number: p.Number})
	}
	return out
}
