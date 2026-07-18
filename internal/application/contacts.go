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

// ContactAddressInput is a raw labelled postal address from the UI, validated into a domain value on
// save.
type ContactAddressInput struct {
	Label      string
	Street     string
	Locality   string
	Region     string
	PostalCode string
	Country    string
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
	Birthday      string
	Emails        []ContactEmailInput
	Phones        []ContactPhoneInput
	Addresses     []ContactAddressInput
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
	addresses, err := buildContactAddresses(in.Addresses)
	if err != nil {
		return fmt.Errorf("contacts: build address: %w", err)
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
		Birthday:      in.Birthday,
		Emails:        emails,
		Phones:        phones,
		Addresses:     addresses,
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

// ContactImportResult reports the outcome of an import: how many decoded records were stored as new
// contacts and how many were merged into ones already in the address book.
type ContactImportResult struct {
	Added   int
	Updated int
}

// ImportContacts decodes contacts from the given bytes with the codec and reconciles each against the
// existing address book, so re-importing a source updates its records rather than duplicating them.
// A decoded record matches an existing contact when it carries the same id (a vCard UID, where the
// format supplies one) or shares any email address with it; a record with no email at all falls back
// to matching an email-less contact by display name. CSV carries no id, which is why address matching
// is needed: without it every CSV import would insert a fresh copy of every row. A match is merged
// rather than overwritten, so an import only ever adds detail to a stored contact.
func (s *ContactService) ImportContacts(ctx context.Context, codec ContactCodec, data []byte) (ContactImportResult, error) {
	contacts, err := codec.Decode(data)
	if err != nil {
		return ContactImportResult{}, fmt.Errorf("contacts: decode import: %w", err)
	}
	existing, err := s.contacts.ListContacts(ctx)
	if err != nil {
		return ContactImportResult{}, fmt.Errorf("contacts: list for import: %w", err)
	}
	index := newContactIndex(existing)
	var result ContactImportResult
	for _, incoming := range contacts {
		record := incoming
		match, matched := index.find(incoming)
		if matched {
			record = match.MergedWith(incoming)
		}
		if err := s.contacts.SaveContact(ctx, record); err != nil {
			return result, fmt.Errorf("contacts: import save %q: %w", record.ID(), err)
		}
		// Index the stored record so later rows in the same source reconcile against it too, which is
		// what stops a file that repeats a contact from inserting it twice.
		index.put(record)
		if matched {
			result.Updated++
			continue
		}
		result.Added++
	}
	return result, nil
}

// contactIndex resolves a decoded contact to the stored contact it represents, by id, then by any
// shared email address, then by display name for records that carry no email.
type contactIndex struct {
	byID    map[string]domain.Contact
	byEmail map[string]string // lower-cased address to contact id
	byName  map[string]string // lower-cased display name to contact id, email-less contacts only
}

// newContactIndex builds the lookup over the contacts already in the address book.
func newContactIndex(contacts []domain.Contact) *contactIndex {
	index := &contactIndex{
		byID:    make(map[string]domain.Contact, len(contacts)),
		byEmail: make(map[string]string, len(contacts)),
		byName:  make(map[string]string, len(contacts)),
	}
	for _, c := range contacts {
		index.put(c)
	}
	return index
}

// put adds or refreshes a contact's entries. The first contact to claim an address or a name keeps it,
// so an import cannot silently repoint an existing key at a different record.
func (i *contactIndex) put(c domain.Contact) {
	i.byID[c.ID()] = c
	emails := c.Emails()
	for _, e := range emails {
		key := strings.ToLower(e.Address().Address())
		if _, taken := i.byEmail[key]; !taken {
			i.byEmail[key] = c.ID()
		}
	}
	if len(emails) > 0 {
		return
	}
	key := strings.ToLower(c.FormattedName())
	if _, taken := i.byName[key]; !taken {
		i.byName[key] = c.ID()
	}
}

// find returns the stored contact the incoming record represents, and whether there was one.
func (i *contactIndex) find(incoming domain.Contact) (domain.Contact, bool) {
	if c, ok := i.byID[incoming.ID()]; ok {
		return c, true
	}
	emails := incoming.Emails()
	for _, e := range emails {
		if id, ok := i.byEmail[strings.ToLower(e.Address().Address())]; ok {
			return i.byID[id], true
		}
	}
	if len(emails) > 0 {
		// A record that carries addresses but matched none of them is a different person, even when a
		// generic display name such as "Support" collides with one already stored.
		return domain.Contact{}, false
	}
	if id, ok := i.byName[strings.ToLower(incoming.FormattedName())]; ok {
		return i.byID[id], true
	}
	return domain.Contact{}, false
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

// buildContactAddresses validates each raw address input into a domain value, or returns nil when there
// are none.
func buildContactAddresses(in []ContactAddressInput) ([]domain.ContactAddress, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]domain.ContactAddress, 0, len(in))
	for _, a := range in {
		address, err := domain.NewContactAddress(a.Label, a.Street, a.Locality, a.Region, a.PostalCode, a.Country)
		if err != nil {
			return nil, err
		}
		out = append(out, address)
	}
	return out, nil
}
