package storage

import (
	"context"
	"testing"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// The store must satisfy the application ContactStore port.
var _ application.ContactStore = (*Store)(nil)

func buildTestContact(t *testing.T, id, name string, emails, phones int) domain.Contact {
	t.Helper()
	in := domain.ContactInput{ID: id, UID: "uid-" + id, FormattedName: name, GivenName: "Given", FamilyName: "Family"}
	c, err := domain.NewContact(withChildren(t, in, emails, phones))
	if err != nil {
		t.Fatalf("contact: %v", err)
	}
	return c
}

// withChildren fills the given input with the requested number of emails and phones.
func withChildren(t *testing.T, in domain.ContactInput, emails, phones int) domain.ContactInput {
	t.Helper()
	for i := 0; i < emails; i++ {
		e, err := domain.NewContactEmail("work", in.ID+"@example.com")
		if err != nil {
			t.Fatalf("email: %v", err)
		}
		in.Emails = append(in.Emails, e)
	}
	for i := 0; i < phones; i++ {
		p, err := domain.NewContactPhone("mobile", "12345")
		if err != nil {
			t.Fatalf("phone: %v", err)
		}
		in.Phones = append(in.Phones, p)
	}
	return in
}

func TestContactRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	in := withChildren(t, domain.ContactInput{
		ID: "c1", UID: "uid-c1", FormattedName: "Jo Bloggs", GivenName: "Given", FamilyName: "Family",
		Birthday: "1990-05-17",
	}, 1, 1)
	addr1, _ := domain.NewContactAddress("home", "1 High St", "London", "Greater London", "E1 1AA", "UK")
	addr2, _ := domain.NewContactAddress("work", "2 Low St", "Leeds", "", "LS1 1AA", "UK")
	in.Addresses = []domain.ContactAddress{addr1, addr2}
	c, err := domain.NewContact(in)
	if err != nil {
		t.Fatalf("contact: %v", err)
	}
	if err := store.SaveContact(ctx, c); err != nil {
		t.Fatalf("SaveContact: %v", err)
	}
	got, err := store.GetContact(ctx, "c1")
	if err != nil {
		t.Fatalf("GetContact: %v", err)
	}
	if got.FormattedName() != "Jo Bloggs" || got.UID() != "uid-c1" || got.GivenName() != "Given" {
		t.Errorf("fields not persisted: %+v", got)
	}
	if got.Birthday() != "1990-05-17" {
		t.Errorf("birthday = %q, want 1990-05-17", got.Birthday())
	}
	if len(got.Emails()) != 1 || got.PrimaryEmail().Address() != "c1@example.com" {
		t.Errorf("emails = %+v", got.Emails())
	}
	if len(got.Phones()) != 1 || got.Phones()[0].Number() != "12345" {
		t.Errorf("phones = %+v", got.Phones())
	}
	addrs := got.Addresses()
	if len(addrs) != 2 {
		t.Fatalf("addresses = %+v, want 2 in order", addrs)
	}
	if addrs[0].Label() != "home" || addrs[0].Street() != "1 High St" || addrs[0].Locality() != "London" ||
		addrs[0].Region() != "Greater London" || addrs[0].PostalCode() != "E1 1AA" || addrs[0].Country() != "UK" {
		t.Errorf("first address not persisted: %+v", addrs[0])
	}
	if addrs[1].Label() != "work" || addrs[1].Street() != "2 Low St" || addrs[1].Region() != "" {
		t.Errorf("second address not persisted: %+v", addrs[1])
	}
}

func TestListContactsOrderedByName(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.SaveContact(ctx, buildTestContact(t, "c1", "Zara", 0, 0)); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.SaveContact(ctx, buildTestContact(t, "c2", "Amy", 2, 0)); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := store.ListContacts(ctx)
	if err != nil {
		t.Fatalf("ListContacts: %v", err)
	}
	if len(got) != 2 || got[0].FormattedName() != "Amy" || got[1].FormattedName() != "Zara" {
		t.Errorf("order = %v", []string{got[0].FormattedName(), got[1].FormattedName()})
	}
	if len(got[0].Emails()) != 2 {
		t.Errorf("Amy emails = %d, want 2", len(got[0].Emails()))
	}
}

func TestSaveContactReplacesChildren(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.SaveContact(ctx, buildTestContact(t, "c1", "Jo", 3, 2)); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Re-save the same id with fewer children; the old rows must be replaced, not accumulated.
	if err := store.SaveContact(ctx, buildTestContact(t, "c1", "Jo", 1, 0)); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	got, err := store.GetContact(ctx, "c1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Emails()) != 1 || len(got.Phones()) != 0 {
		t.Errorf("children not replaced: %d emails, %d phones", len(got.Emails()), len(got.Phones()))
	}
}

func TestDeleteContactRemovesMembership(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.SaveContact(ctx, buildTestContact(t, "c1", "Jo", 1, 1)); err != nil {
		t.Fatalf("save contact: %v", err)
	}
	group, _ := domain.NewContactGroup("g1", "Friends", []string{"c1"})
	if err := store.SaveContactGroup(ctx, group); err != nil {
		t.Fatalf("save group: %v", err)
	}

	if err := store.DeleteContact(ctx, "c1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.GetContact(ctx, "c1"); err == nil {
		t.Errorf("expected an error getting a deleted contact")
	}
	groups, err := store.ListContactGroups(ctx)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	if len(groups) != 1 || len(groups[0].Members()) != 0 {
		t.Errorf("deleted contact should be removed from the group, members = %v", groups[0].Members())
	}
}

func TestContactGroupRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	group, _ := domain.NewContactGroup("g1", "Team", []string{"c1", "c2"})
	if err := store.SaveContactGroup(ctx, group); err != nil {
		t.Fatalf("save group: %v", err)
	}
	got, err := store.ListContactGroups(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].Name() != "Team" {
		t.Fatalf("group = %+v", got)
	}
	members := got[0].Members()
	if len(members) != 2 || members[0] != "c1" || members[1] != "c2" {
		t.Errorf("members = %v, want [c1 c2] in order", members)
	}

	if err := store.DeleteContactGroup(ctx, "g1"); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	after, err := store.ListContactGroups(ctx)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("group not deleted: %v", after)
	}
}
