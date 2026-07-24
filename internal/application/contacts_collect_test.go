package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// contactWithEmail builds a stored contact carrying one address, for seeding the fake store.
func contactWithEmail(t *testing.T, id, name, address string) domain.Contact {
	t.Helper()
	email, err := domain.NewContactEmail("", address)
	if err != nil {
		t.Fatalf("email %q: %v", address, err)
	}
	c, err := domain.NewContact(domain.ContactInput{ID: id, FormattedName: name, Emails: []domain.ContactEmail{email}})
	if err != nil {
		t.Fatalf("contact %q: %v", id, err)
	}
	return c
}

func TestCollectAddressesAddsOnlyUnknown(t *testing.T) {
	store := &fakeContactStore{contacts: []domain.Contact{contactWithEmail(t, "c1", "Jane", "jane@example.com")}}
	svc := NewContactService(store, fixedID("new"))

	added, err := svc.CollectAddresses(context.Background(), []string{
		"JANE@example.com",     // already in the book, case-insensitively
		"bob@new.example",      // new
		"Bob@New.example",      // duplicate of the previous within this call
		"not-an-email",         // malformed, skipped silently
		"",                     // empty, skipped silently
		"second@new.example",   // new
	})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if added != 2 || len(store.savedC) != 2 {
		t.Fatalf("added = %d (saved %d), want 2", added, len(store.savedC))
	}
	first := store.savedC[0]
	if first.FormattedName() != "bob@new.example" {
		t.Errorf("collected display name = %q, want the address", first.FormattedName())
	}
	emails := first.Emails()
	if len(emails) != 1 || emails[0].Address().Address() != "bob@new.example" {
		t.Errorf("collected emails = %+v", emails)
	}
}

func TestCollectAddressesNothingNew(t *testing.T) {
	store := &fakeContactStore{contacts: []domain.Contact{contactWithEmail(t, "c1", "Jane", "jane@example.com")}}
	svc := NewContactService(store, fixedID("new"))
	added, err := svc.CollectAddresses(context.Background(), []string{"jane@example.com"})
	if err != nil || added != 0 || len(store.savedC) != 0 {
		t.Fatalf("collect = %d, %v (saved %d); want 0 added, no error, no saves", added, err, len(store.savedC))
	}
}

func TestCollectAddressesListError(t *testing.T) {
	svc := NewContactService(&fakeContactStore{listErr: errBoom}, fixedID("new"))
	if _, err := svc.CollectAddresses(context.Background(), []string{"a@b.example"}); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestCollectAddressesSaveError(t *testing.T) {
	svc := NewContactService(&fakeContactStore{saveErr: errBoom}, fixedID("new"))
	added, err := svc.CollectAddresses(context.Background(), []string{"a@b.example"})
	if added != 0 || !errors.Is(err, errBoom) {
		t.Errorf("collect = %d, %v; want 0 and wrapped errBoom", added, err)
	}
}

func TestCollectAddressesRejectsAnEmptyGeneratedID(t *testing.T) {
	svc := NewContactService(&fakeContactStore{}, fixedID(""))
	_, err := svc.CollectAddresses(context.Background(), []string{"a@b.example"})
	if err == nil || !strings.Contains(err.Error(), "build collected contact") {
		t.Errorf("err = %v, want a build error", err)
	}
}
