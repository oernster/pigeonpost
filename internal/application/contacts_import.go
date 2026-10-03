package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// ContactImportResult reports the outcome of an import: how many decoded records were stored as new
// contacts, how many were merged into ones already in the address book and how many records in the
// file were skipped because they could not form a contact.
type ContactImportResult struct {
	Added   int
	Updated int
	Skipped int
}

// ImportContacts decodes contacts from the given bytes with the codec and reconciles each against the
// existing address book, so re-importing a source updates its records rather than duplicating them.
// A decoded record matches an existing contact when it carries the same id (a vCard UID, where the
// format supplies one) or shares any email address with it; a record with no email at all falls back
// to matching an email-less contact by display name. CSV carries no id, which is why address matching
// is needed: without it every CSV import would insert a fresh copy of every row. A match is merged
// rather than overwritten, so an import only ever adds detail to a stored contact.
func (s *ContactService) ImportContacts(ctx context.Context, codec ContactCodec, data []byte) (ContactImportResult, error) {
	contacts, skipped, err := codec.DecodeImport(data)
	if err != nil {
		return ContactImportResult{}, fmt.Errorf("contacts: decode import: %w", err)
	}
	existing, err := s.contacts.ListContacts(ctx)
	if err != nil {
		return ContactImportResult{}, fmt.Errorf("contacts: list for import: %w", err)
	}
	index := newContactIndex(existing)
	result := ContactImportResult{Skipped: skipped}
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

// find returns the stored contact the incoming record represents with whether there was one.
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
