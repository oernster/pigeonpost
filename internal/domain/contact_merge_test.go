package domain

import "testing"

// mergeEmail builds a labelled email or fails the test.
func mergeEmail(t *testing.T, label, address string) ContactEmail {
	t.Helper()
	e, err := NewContactEmail(label, address)
	if err != nil {
		t.Fatalf("email %q: %v", address, err)
	}
	return e
}

// mergePhone builds a labelled phone or fails the test.
func mergePhone(t *testing.T, label, number string) ContactPhone {
	t.Helper()
	p, err := NewContactPhone(label, number)
	if err != nil {
		t.Fatalf("phone %q: %v", number, err)
	}
	return p
}

// mergeContact builds a contact or fails the test.
func mergeContact(t *testing.T, in ContactInput) Contact {
	t.Helper()
	c, err := NewContact(in)
	if err != nil {
		t.Fatalf("contact: %v", err)
	}
	return c
}

func TestMergedWithKeepsIdentityAndExistingValues(t *testing.T) {
	stored := mergeContact(t, ContactInput{
		ID: "stored", UID: "stored-uid", FormattedName: "Jo Bloggs",
		Organization: "Acme", Note: "my own note",
	})
	incoming := mergeContact(t, ContactInput{
		ID: "incoming", UID: "incoming-uid", FormattedName: "Josephine Bloggs",
		Organization: "Globex", Title: "Engineer", Note: "imported note", Birthday: "1980-01-15",
	})

	merged := stored.MergedWith(incoming)

	if merged.ID() != "stored" || merged.UID() != "stored-uid" {
		t.Errorf("identity changed: %q/%q", merged.ID(), merged.UID())
	}
	if merged.FormattedName() != "Jo Bloggs" || merged.Organization() != "Acme" || merged.Note() != "my own note" {
		t.Errorf("existing values were overwritten: %+v", merged)
	}
	if merged.Title() != "Engineer" || merged.Birthday() != "1980-01-15" {
		t.Errorf("blank fields were not filled from the import: %+v", merged)
	}
}

func TestMergedWithUnionsEmailsIgnoringCase(t *testing.T) {
	stored := mergeContact(t, ContactInput{
		ID: "c1", FormattedName: "Jo",
		Emails: []ContactEmail{mergeEmail(t, "home", "jo@example.com")},
	})
	incoming := mergeContact(t, ContactInput{
		ID: "c2", FormattedName: "Jo",
		Emails: []ContactEmail{
			mergeEmail(t, "work", "JO@EXAMPLE.COM"),
			mergeEmail(t, "work", "jo@work.example.com"),
		},
	})

	merged := stored.MergedWith(incoming)

	if len(merged.Emails()) != 2 {
		t.Fatalf("emails = %+v, want the case-different repeat dropped", merged.Emails())
	}
	if merged.Emails()[0].Address().Address() != "jo@example.com" {
		t.Errorf("the stored email lost its position: %+v", merged.Emails())
	}
}

func TestMergedWithUnionsPhonesByNumber(t *testing.T) {
	stored := mergeContact(t, ContactInput{
		ID: "c1", FormattedName: "Jo",
		Phones: []ContactPhone{mergePhone(t, "home", "555-1000")},
	})
	incoming := mergeContact(t, ContactInput{
		ID: "c2", FormattedName: "Jo",
		Phones: []ContactPhone{
			mergePhone(t, "mobile", "555-1000"), // same number, different label
			mergePhone(t, "mobile", "555-2000"),
		},
	})

	merged := stored.MergedWith(incoming)

	if len(merged.Phones()) != 2 {
		t.Errorf("phones = %+v, want the relabelled repeat dropped", merged.Phones())
	}
}

func TestMergedWithUnionsAddressesByComponents(t *testing.T) {
	same, err := NewContactAddress("home", "1 High St", "Leadworth", "Gloucestershire", "GL1 2AB", "UK")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	relabelled, err := NewContactAddress("work", "1 High St", "Leadworth", "Gloucestershire", "GL1 2AB", "UK")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	other, err := NewContactAddress("work", "1 Tardis Way", "London", "", "SW1A 1AA", "UK")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	stored := mergeContact(t, ContactInput{ID: "c1", FormattedName: "Jo", Addresses: []ContactAddress{same}})
	incoming := mergeContact(t, ContactInput{
		ID: "c2", FormattedName: "Jo", Addresses: []ContactAddress{relabelled, other},
	})

	merged := stored.MergedWith(incoming)

	if len(merged.Addresses()) != 2 {
		t.Errorf("addresses = %+v, want the relabelled duplicate dropped", merged.Addresses())
	}
}

func TestMergedWithEmptyIncomingChangesNothing(t *testing.T) {
	stored := mergeContact(t, ContactInput{
		ID: "c1", FormattedName: "Jo", Organization: "Acme",
		Emails: []ContactEmail{mergeEmail(t, "", "jo@example.com")},
	})
	incoming := mergeContact(t, ContactInput{ID: "c2", FormattedName: "Jo"})

	merged := stored.MergedWith(incoming)

	if merged.Organization() != "Acme" || len(merged.Emails()) != 1 {
		t.Errorf("merge with a bare record altered the stored contact: %+v", merged)
	}
}
