package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ContactEmailInput is a raw labelled email from the UI, validated into a domain value on save.
type ContactEmailInput struct {
	Label   string
	Address string
}

// ContactPhoneInput is a raw labelled phone number from the UI, validated on save.
type ContactPhoneInput struct {
	Label  string
	Number string
}

// ContactInput carries the fields to create or update a contact. An empty ID means a new contact; an
// empty UID means the store assigns one.
type ContactInput struct {
	ID            string
	UID           string
	FormattedName string
	GivenName     string
	FamilyName    string
	Organization  string
	Title         string
	Note          string
	Emails        []ContactEmailInput
	Phones        []ContactPhoneInput
}

// ContactGroupInput carries the fields to create or update a group. An empty ID means a new group.
type ContactGroupInput struct {
	ID      string
	Name    string
	Members []string
}

// ContactService is the use-case boundary for managing the address book: contacts and groups.
type ContactService struct {
	contacts ContactStore
	newID    IDGenerator
}

// NewContactService constructs the service with its injected store and id generator.
func NewContactService(contacts ContactStore, newID IDGenerator) *ContactService {
	return &ContactService{contacts: contacts, newID: newID}
}

// ListContacts returns all contacts.
func (s *ContactService) ListContacts(ctx context.Context) ([]domain.Contact, error) {
	contacts, err := s.contacts.ListContacts(ctx)
	if err != nil {
		return nil, fmt.Errorf("contacts: list: %w", err)
	}
	return contacts, nil
}

// GetContact returns a single contact by id.
func (s *ContactService) GetContact(ctx context.Context, id string) (domain.Contact, error) {
	contact, err := s.contacts.GetContact(ctx, id)
	if err != nil {
		return domain.Contact{}, fmt.Errorf("contacts: get %q: %w", id, err)
	}
	return contact, nil
}

// SaveContact validates and persists a contact, generating an id when one is not supplied (a new
// contact).
func (s *ContactService) SaveContact(ctx context.Context, in ContactInput) error {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = s.newID()
	}
	emails, err := buildContactEmails(in.Emails)
	if err != nil {
		return fmt.Errorf("contacts: build email: %w", err)
	}
	phones, err := buildContactPhones(in.Phones)
	if err != nil {
		return fmt.Errorf("contacts: build phone: %w", err)
	}
	contact, err := domain.NewContact(domain.ContactInput{
		ID:            id,
		UID:           in.UID,
		FormattedName: in.FormattedName,
		GivenName:     in.GivenName,
		FamilyName:    in.FamilyName,
		Organization:  in.Organization,
		Title:         in.Title,
		Note:          in.Note,
		Emails:        emails,
		Phones:        phones,
	})
	if err != nil {
		return fmt.Errorf("contacts: build contact: %w", err)
	}
	if err := s.contacts.SaveContact(ctx, contact); err != nil {
		return fmt.Errorf("contacts: save: %w", err)
	}
	return nil
}

// DeleteContact removes a contact by id.
func (s *ContactService) DeleteContact(ctx context.Context, id string) error {
	if err := s.contacts.DeleteContact(ctx, id); err != nil {
		return fmt.Errorf("contacts: delete %q: %w", id, err)
	}
	return nil
}

// ListGroups returns all contact groups.
func (s *ContactService) ListGroups(ctx context.Context) ([]domain.ContactGroup, error) {
	groups, err := s.contacts.ListContactGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("contacts: list groups: %w", err)
	}
	return groups, nil
}

// SaveGroup validates and persists a group, generating an id when one is not supplied.
func (s *ContactService) SaveGroup(ctx context.Context, in ContactGroupInput) error {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = s.newID()
	}
	group, err := domain.NewContactGroup(id, in.Name, in.Members)
	if err != nil {
		return fmt.Errorf("contacts: build group: %w", err)
	}
	if err := s.contacts.SaveContactGroup(ctx, group); err != nil {
		return fmt.Errorf("contacts: save group: %w", err)
	}
	return nil
}

// DeleteGroup removes a group by id.
func (s *ContactService) DeleteGroup(ctx context.Context, id string) error {
	if err := s.contacts.DeleteContactGroup(ctx, id); err != nil {
		return fmt.Errorf("contacts: delete group %q: %w", id, err)
	}
	return nil
}

// ImportContacts decodes contacts from the given bytes with the codec and saves each. A decoded
// contact keeps its own id (a vCard UID where present), so re-importing the same source updates the
// matching records rather than duplicating them. It returns the number imported.
func (s *ContactService) ImportContacts(ctx context.Context, codec ContactCodec, data []byte) (int, error) {
	contacts, err := codec.Decode(data)
	if err != nil {
		return 0, fmt.Errorf("contacts: decode import: %w", err)
	}
	for i, c := range contacts {
		if err := s.contacts.SaveContact(ctx, c); err != nil {
			return i, fmt.Errorf("contacts: import save %q: %w", c.ID(), err)
		}
	}
	return len(contacts), nil
}

// ExportContacts encodes every contact with the codec into its serialised form.
func (s *ContactService) ExportContacts(ctx context.Context, codec ContactCodec) ([]byte, error) {
	contacts, err := s.contacts.ListContacts(ctx)
	if err != nil {
		return nil, fmt.Errorf("contacts: list for export: %w", err)
	}
	data, err := codec.Encode(contacts)
	if err != nil {
		return nil, fmt.Errorf("contacts: encode export: %w", err)
	}
	return data, nil
}

// buildContactEmails validates each raw email input into a domain value, or returns nil when there are
// none.
func buildContactEmails(in []ContactEmailInput) ([]domain.ContactEmail, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]domain.ContactEmail, 0, len(in))
	for _, e := range in {
		email, err := domain.NewContactEmail(e.Label, e.Address)
		if err != nil {
			return nil, err
		}
		out = append(out, email)
	}
	return out, nil
}

// buildContactPhones validates each raw phone input into a domain value, or returns nil when there are
// none.
func buildContactPhones(in []ContactPhoneInput) ([]domain.ContactPhone, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]domain.ContactPhone, 0, len(in))
	for _, p := range in {
		phone, err := domain.NewContactPhone(p.Label, p.Number)
		if err != nil {
			return nil, err
		}
		out = append(out, phone)
	}
	return out, nil
}
