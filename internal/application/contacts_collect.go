package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// CollectAddresses adds a minimal contact (the address as its display name, nothing else) for each
// given address not already present anywhere in the address book, and returns how many were added.
// It backs the automatic collection of outgoing recipients: sending to someone quietly makes them a
// contact, which the user can then flesh out or delete. Matching is case-insensitive and an address
// appearing twice in one call is added once. A malformed or empty address is skipped rather than an
// error, because collection is a best-effort side effect of sending, never a reason to complain
// about a send that has already succeeded.
func (s *ContactService) CollectAddresses(ctx context.Context, addresses []string) (int, error) {
	existing, err := s.contacts.ListContacts(ctx)
	if err != nil {
		return 0, fmt.Errorf("contacts: list for collect: %w", err)
	}
	known := make(map[string]bool)
	for _, contact := range existing {
		for _, email := range contact.Emails() {
			known[strings.ToLower(email.Address().Address())] = true
		}
	}
	added := 0
	for _, raw := range addresses {
		email, err := domain.NewContactEmail("", raw)
		if err != nil {
			continue
		}
		key := strings.ToLower(email.Address().Address())
		if known[key] {
			continue
		}
		known[key] = true
		contact, err := domain.NewContact(domain.ContactInput{
			ID:            s.newID(),
			FormattedName: email.Address().Address(),
			Emails:        []domain.ContactEmail{email},
		})
		if err != nil {
			return added, fmt.Errorf("contacts: build collected contact %q: %w", key, err)
		}
		if err := s.contacts.SaveContact(ctx, contact); err != nil {
			return added, fmt.Errorf("contacts: save collected contact %q: %w", key, err)
		}
		added++
	}
	return added, nil
}
