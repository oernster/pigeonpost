package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// fakeContactStore is a hand-written in-memory ContactStore with error-injection fields.
type fakeContactStore struct {
	contacts   []domain.Contact
	groups     []domain.ContactGroup
	got        domain.Contact
	listErr    error
	getErr     error
	saveErr    error
	deleteErr  error
	listGrpErr error
	saveGrpErr error
	delGrpErr  error
	savedC     []domain.Contact
	deletedC   []string
	savedG     []domain.ContactGroup
	deletedG   []string
}

func (f *fakeContactStore) ListContacts(context.Context) ([]domain.Contact, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.contacts, nil
}

func (f *fakeContactStore) GetContact(context.Context, string) (domain.Contact, error) {
	if f.getErr != nil {
		return domain.Contact{}, f.getErr
	}
	return f.got, nil
}

func (f *fakeContactStore) SaveContact(_ context.Context, c domain.Contact) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.savedC = append(f.savedC, c)
	return nil
}

func (f *fakeContactStore) DeleteContact(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedC = append(f.deletedC, id)
	return nil
}

func (f *fakeContactStore) ListContactGroups(context.Context) ([]domain.ContactGroup, error) {
	if f.listGrpErr != nil {
		return nil, f.listGrpErr
	}
	return f.groups, nil
}

func (f *fakeContactStore) SaveContactGroup(_ context.Context, g domain.ContactGroup) error {
	if f.saveGrpErr != nil {
		return f.saveGrpErr
	}
	f.savedG = append(f.savedG, g)
	return nil
}

func (f *fakeContactStore) DeleteContactGroup(_ context.Context, id string) error {
	if f.delGrpErr != nil {
		return f.delGrpErr
	}
	f.deletedG = append(f.deletedG, id)
	return nil
}

func fixedID(id string) IDGenerator { return func() string { return id } }

// fakeContactCodec is a hand-written ContactCodec with error-injection fields.
type fakeContactCodec struct {
	decoded   []domain.Contact
	decodeErr error
	encoded   []byte
	encodeErr error
	gotEncode []domain.Contact
}

func (f *fakeContactCodec) Decode([]byte) ([]domain.Contact, error) {
	if f.decodeErr != nil {
		return nil, f.decodeErr
	}
	return f.decoded, nil
}

func (f *fakeContactCodec) Encode(cs []domain.Contact) ([]byte, error) {
	if f.encodeErr != nil {
		return nil, f.encodeErr
	}
	f.gotEncode = cs
	return f.encoded, nil
}

func mustContact(t *testing.T, id, name string) domain.Contact {
	t.Helper()
	c, err := domain.NewContact(domain.ContactInput{ID: id, UID: id, FormattedName: name})
	if err != nil {
		t.Fatalf("contact: %v", err)
	}
	return c
}

func TestImportContactsDecodeError(t *testing.T) {
	svc := NewContactService(&fakeContactStore{}, fixedID("x"))
	codec := &fakeContactCodec{decodeErr: errBoom}
	if n, err := svc.ImportContacts(context.Background(), codec, nil); n != (ContactImportResult{}) || !errors.Is(err, errBoom) {
		t.Errorf("import = %+v, %v; want a zero result and wrapped errBoom", n, err)
	}
}

func TestImportContactsListError(t *testing.T) {
	store := &fakeContactStore{listErr: errBoom}
	codec := &fakeContactCodec{decoded: []domain.Contact{mustContact(t, "c1", "Jo")}}
	svc := NewContactService(store, fixedID("x"))
	if n, err := svc.ImportContacts(context.Background(), codec, nil); n != (ContactImportResult{}) || !errors.Is(err, errBoom) {
		t.Errorf("import = %+v, %v; want a zero result and wrapped errBoom", n, err)
	}
}

func TestImportContactsSaveError(t *testing.T) {
	store := &fakeContactStore{saveErr: errBoom}
	codec := &fakeContactCodec{decoded: []domain.Contact{mustContact(t, "c1", "Jo"), mustContact(t, "c2", "Amy")}}
	svc := NewContactService(store, fixedID("x"))
	if n, err := svc.ImportContacts(context.Background(), codec, nil); n != (ContactImportResult{}) || !errors.Is(err, errBoom) {
		t.Errorf("import = %+v, %v; want a zero result and wrapped errBoom", n, err)
	}
}

func TestImportContactsSuccess(t *testing.T) {
	store := &fakeContactStore{}
	codec := &fakeContactCodec{decoded: []domain.Contact{mustContact(t, "c1", "Jo"), mustContact(t, "c2", "Amy")}}
	svc := NewContactService(store, fixedID("x"))
	n, err := svc.ImportContacts(context.Background(), codec, []byte("data"))
	if err != nil || n != (ContactImportResult{Added: 2}) {
		t.Fatalf("import = %+v, %v; want 2 added and no error", n, err)
	}
	if len(store.savedC) != 2 {
		t.Errorf("saved %d contacts, want 2", len(store.savedC))
	}
}

func TestExportContactsListError(t *testing.T) {
	store := &fakeContactStore{listErr: errBoom}
	svc := NewContactService(store, fixedID("x"))
	if _, err := svc.ExportContacts(context.Background(), &fakeContactCodec{}); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestExportContactsEncodeError(t *testing.T) {
	store := &fakeContactStore{contacts: []domain.Contact{mustContact(t, "c1", "Jo")}}
	svc := NewContactService(store, fixedID("x"))
	if _, err := svc.ExportContacts(context.Background(), &fakeContactCodec{encodeErr: errBoom}); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestExportContactsSuccess(t *testing.T) {
	store := &fakeContactStore{contacts: []domain.Contact{mustContact(t, "c1", "Jo")}}
	codec := &fakeContactCodec{encoded: []byte("BEGIN:VCARD")}
	svc := NewContactService(store, fixedID("x"))
	data, err := svc.ExportContacts(context.Background(), codec)
	if err != nil || string(data) != "BEGIN:VCARD" {
		t.Fatalf("export = %q, %v", data, err)
	}
	if len(codec.gotEncode) != 1 || codec.gotEncode[0].ID() != "c1" {
		t.Errorf("codec received %+v", codec.gotEncode)
	}
}

func TestContactServiceListContacts(t *testing.T) {
	jo, _ := domain.NewContact(domain.ContactInput{ID: "c1", FormattedName: "Jo"})
	store := &fakeContactStore{contacts: []domain.Contact{jo}}
	svc := NewContactService(store, fixedID("x"))

	got, err := svc.ListContacts(context.Background())
	if err != nil || len(got) != 1 || got[0].ID() != "c1" {
		t.Fatalf("ListContacts = %v, %v", got, err)
	}

	store.listErr = errBoom
	if _, err := svc.ListContacts(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("list error = %v, want wrapped errBoom", err)
	}
}

func TestContactServiceGetContact(t *testing.T) {
	jo, _ := domain.NewContact(domain.ContactInput{ID: "c1", FormattedName: "Jo"})
	store := &fakeContactStore{got: jo}
	svc := NewContactService(store, fixedID("x"))

	got, err := svc.GetContact(context.Background(), "c1")
	if err != nil || got.ID() != "c1" {
		t.Fatalf("GetContact = %v, %v", got, err)
	}

	store.getErr = errBoom
	if _, err := svc.GetContact(context.Background(), "c1"); !errors.Is(err, errBoom) {
		t.Errorf("get error = %v, want wrapped errBoom", err)
	}
}

func TestContactServiceSaveContactNewGeneratesID(t *testing.T) {
	store := &fakeContactStore{}
	svc := NewContactService(store, fixedID("generated"))

	err := svc.SaveContact(context.Background(), ContactInput{
		FormattedName: "Jo Bloggs",
		Emails:        []ContactEmailInput{{Label: "work", Address: "jo@example.com"}},
		Phones:        []ContactPhoneInput{{Label: "mobile", Number: "12345"}},
	})
	if err != nil {
		t.Fatalf("SaveContact: %v", err)
	}
	if len(store.savedC) != 1 || store.savedC[0].ID() != "generated" {
		t.Fatalf("saved = %+v, want a contact with the generated id", store.savedC)
	}
	saved := store.savedC[0]
	if len(saved.Emails()) != 1 || len(saved.Phones()) != 1 {
		t.Errorf("emails/phones not persisted: %+v", saved)
	}
}

func TestContactServiceSaveContactPersistsBirthdayAndAddresses(t *testing.T) {
	store := &fakeContactStore{}
	svc := NewContactService(store, fixedID("generated"))

	err := svc.SaveContact(context.Background(), ContactInput{
		FormattedName: "Jo Bloggs",
		Birthday:      "1990-05-17",
		Addresses: []ContactAddressInput{
			{Label: "home", Street: "1 High St", Locality: "London", PostalCode: "E1 1AA", Country: "UK"},
			{Label: "work", Country: "UK"},
		},
	})
	if err != nil {
		t.Fatalf("SaveContact: %v", err)
	}
	saved := store.savedC[0]
	if saved.Birthday() != "1990-05-17" {
		t.Errorf("birthday = %q, want 1990-05-17", saved.Birthday())
	}
	addrs := saved.Addresses()
	if len(addrs) != 2 || addrs[0].Street() != "1 High St" || addrs[0].Label() != "home" || addrs[1].Country() != "UK" {
		t.Errorf("addresses not persisted: %+v", addrs)
	}
}

func TestContactServiceSaveContactAddressError(t *testing.T) {
	svc := NewContactService(&fakeContactStore{}, fixedID("x"))
	err := svc.SaveContact(context.Background(), ContactInput{
		FormattedName: "Jo",
		// An address with every component empty must fail with ErrEmptyAddress.
		Addresses: []ContactAddressInput{{Label: "home"}},
	})
	if !errors.Is(err, domain.ErrEmptyAddress) {
		t.Errorf("err = %v, want wrapped ErrEmptyAddress", err)
	}
}

func TestContactServiceSaveContactExistingIDNoEmails(t *testing.T) {
	store := &fakeContactStore{}
	svc := NewContactService(store, fixedID("unused"))

	if err := svc.SaveContact(context.Background(), ContactInput{ID: "  c1  ", FormattedName: "Jo"}); err != nil {
		t.Fatalf("SaveContact: %v", err)
	}
	if store.savedC[0].ID() != "c1" {
		t.Errorf("id = %q, want the supplied c1 (trimmed), not generated", store.savedC[0].ID())
	}
	if store.savedC[0].Emails() != nil || store.savedC[0].Phones() != nil {
		t.Errorf("expected no emails/phones")
	}
}

func TestContactServiceSaveContactEmailError(t *testing.T) {
	svc := NewContactService(&fakeContactStore{}, fixedID("x"))
	err := svc.SaveContact(context.Background(), ContactInput{
		FormattedName: "Jo",
		Emails:        []ContactEmailInput{{Label: "home", Address: "not-an-email"}},
	})
	if !errors.Is(err, domain.ErrInvalidEmailAddress) {
		t.Errorf("err = %v, want wrapped ErrInvalidEmailAddress", err)
	}
}

func TestContactServiceSaveContactPhoneError(t *testing.T) {
	svc := NewContactService(&fakeContactStore{}, fixedID("x"))
	err := svc.SaveContact(context.Background(), ContactInput{
		FormattedName: "Jo",
		Emails:        []ContactEmailInput{{Address: "jo@example.com"}},
		Phones:        []ContactPhoneInput{{Label: "home", Number: "  "}},
	})
	if !errors.Is(err, domain.ErrEmptyPhoneNumber) {
		t.Errorf("err = %v, want wrapped ErrEmptyPhoneNumber", err)
	}
}

func TestContactServiceSaveContactDomainError(t *testing.T) {
	svc := NewContactService(&fakeContactStore{}, fixedID("x"))
	if err := svc.SaveContact(context.Background(), ContactInput{FormattedName: "   "}); !errors.Is(err, domain.ErrEmptyContactName) {
		t.Errorf("err = %v, want wrapped ErrEmptyContactName", err)
	}
}

func TestContactServiceSaveContactStoreError(t *testing.T) {
	store := &fakeContactStore{saveErr: errBoom}
	svc := NewContactService(store, fixedID("x"))
	if err := svc.SaveContact(context.Background(), ContactInput{FormattedName: "Jo"}); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestContactServiceDeleteContact(t *testing.T) {
	store := &fakeContactStore{}
	svc := NewContactService(store, fixedID("x"))

	if err := svc.DeleteContact(context.Background(), "c1"); err != nil {
		t.Fatalf("DeleteContact: %v", err)
	}
	if len(store.deletedC) != 1 || store.deletedC[0] != "c1" {
		t.Errorf("deleted = %v", store.deletedC)
	}

	store.deleteErr = errBoom
	if err := svc.DeleteContact(context.Background(), "c1"); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestContactServiceListGroups(t *testing.T) {
	g, _ := domain.NewContactGroup("g1", "Friends", nil)
	store := &fakeContactStore{groups: []domain.ContactGroup{g}}
	svc := NewContactService(store, fixedID("x"))

	got, err := svc.ListGroups(context.Background())
	if err != nil || len(got) != 1 || got[0].ID() != "g1" {
		t.Fatalf("ListGroups = %v, %v", got, err)
	}

	store.listGrpErr = errBoom
	if _, err := svc.ListGroups(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestContactServiceSaveGroup(t *testing.T) {
	store := &fakeContactStore{}
	svc := NewContactService(store, fixedID("generated"))

	// New group: id generated.
	if err := svc.SaveGroup(context.Background(), ContactGroupInput{Name: "Friends", Members: []string{"c1"}}); err != nil {
		t.Fatalf("SaveGroup: %v", err)
	}
	if store.savedG[0].ID() != "generated" {
		t.Errorf("id = %q, want generated", store.savedG[0].ID())
	}

	// Existing id used as-is.
	if err := svc.SaveGroup(context.Background(), ContactGroupInput{ID: " g2 ", Name: "Work"}); err != nil {
		t.Fatalf("SaveGroup: %v", err)
	}
	if store.savedG[1].ID() != "g2" {
		t.Errorf("id = %q, want the supplied g2", store.savedG[1].ID())
	}

	// Domain validation error (blank name).
	if err := svc.SaveGroup(context.Background(), ContactGroupInput{Name: "  "}); !errors.Is(err, domain.ErrEmptyContactGroupName) {
		t.Errorf("err = %v, want wrapped ErrEmptyContactGroupName", err)
	}

	// Store error.
	store.saveGrpErr = errBoom
	if err := svc.SaveGroup(context.Background(), ContactGroupInput{Name: "Friends"}); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

func TestContactServiceDeleteGroup(t *testing.T) {
	store := &fakeContactStore{}
	svc := NewContactService(store, fixedID("x"))

	if err := svc.DeleteGroup(context.Background(), "g1"); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if len(store.deletedG) != 1 || store.deletedG[0] != "g1" {
		t.Errorf("deleted = %v", store.deletedG)
	}

	store.delGrpErr = errBoom
	if err := svc.DeleteGroup(context.Background(), "g1"); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapped errBoom", err)
	}
}

// importContact builds a decoded record as a codec would produce it: a fresh id, since CSV carries
// none, plus whatever addresses distinguish it.
func importContact(t *testing.T, id, name string, emails ...string) domain.Contact {
	t.Helper()
	var addrs []domain.ContactEmail
	for _, a := range emails {
		e, err := domain.NewContactEmail("", a)
		if err != nil {
			t.Fatalf("email %q: %v", a, err)
		}
		addrs = append(addrs, e)
	}
	c, err := domain.NewContact(domain.ContactInput{ID: id, UID: id, FormattedName: name, Emails: addrs})
	if err != nil {
		t.Fatalf("contact: %v", err)
	}
	return c
}

func TestImportContactsMatchesExistingByEmail(t *testing.T) {
	// The stored contact and the incoming row share an address but not an id, which is the whole CSV
	// case: a re-import must update rather than insert a second copy.
	stored := importContact(t, "stored", "Jo Bloggs", "jo@example.com")
	store := &fakeContactStore{contacts: []domain.Contact{stored}}
	incoming := importContact(t, "fresh-random-id", "Jo Bloggs", "JO@example.com")
	svc := NewContactService(store, fixedID("x"))

	got, err := svc.ImportContacts(context.Background(), &fakeContactCodec{decoded: []domain.Contact{incoming}}, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if got != (ContactImportResult{Updated: 1}) {
		t.Errorf("result = %+v, want one update and nothing added", got)
	}
	if len(store.savedC) != 1 || store.savedC[0].ID() != "stored" {
		t.Errorf("saved %+v, want the stored contact's id reused so the upsert replaces it", store.savedC)
	}
}

func TestImportContactsDoesNotMatchDifferentEmailOnSameName(t *testing.T) {
	// Two "Elgato Support" rows with different addresses are two contacts, not one. A name-first rule
	// would collapse them.
	stored := importContact(t, "stored", "Support", "support+one@example.com")
	store := &fakeContactStore{contacts: []domain.Contact{stored}}
	incoming := importContact(t, "fresh", "Support", "support+two@example.com")
	svc := NewContactService(store, fixedID("x"))

	got, err := svc.ImportContacts(context.Background(), &fakeContactCodec{decoded: []domain.Contact{incoming}}, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if got != (ContactImportResult{Added: 1}) {
		t.Errorf("result = %+v, want a separate contact added", got)
	}
}

func TestImportContactsFallsBackToNameWhenNeitherHasEmail(t *testing.T) {
	stored := importContact(t, "stored", "Reception Desk")
	store := &fakeContactStore{contacts: []domain.Contact{stored}}
	incoming := importContact(t, "fresh", "reception desk")
	svc := NewContactService(store, fixedID("x"))

	got, err := svc.ImportContacts(context.Background(), &fakeContactCodec{decoded: []domain.Contact{incoming}}, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if got != (ContactImportResult{Updated: 1}) {
		t.Errorf("result = %+v, want the email-less pair matched by name", got)
	}
}

func TestImportContactsMatchesExistingByID(t *testing.T) {
	// vCard supplies a stable UID as the id, so that path must keep reconciling on it.
	stored := importContact(t, "uid-1", "Jo Bloggs", "jo@example.com")
	store := &fakeContactStore{contacts: []domain.Contact{stored}}
	incoming := importContact(t, "uid-1", "Jo Bloggs", "jo2@example.com")
	svc := NewContactService(store, fixedID("x"))

	got, err := svc.ImportContacts(context.Background(), &fakeContactCodec{decoded: []domain.Contact{incoming}}, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if got != (ContactImportResult{Updated: 1}) {
		t.Errorf("result = %+v, want the uid match", got)
	}
	if len(store.savedC[0].Emails()) != 2 {
		t.Errorf("emails = %+v, want the new address merged in alongside the old", store.savedC[0].Emails())
	}
}

func TestImportContactsCollapsesRepeatsWithinOneFile(t *testing.T) {
	// The same address twice in one import must not insert twice, even against an empty address book.
	store := &fakeContactStore{}
	first := importContact(t, "row-1", "Jo Bloggs", "jo@example.com")
	second := importContact(t, "row-2", "Jo Bloggs", "jo@example.com")
	svc := NewContactService(store, fixedID("x"))

	got, err := svc.ImportContacts(context.Background(),
		&fakeContactCodec{decoded: []domain.Contact{first, second}}, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if got != (ContactImportResult{Added: 1, Updated: 1}) {
		t.Errorf("result = %+v, want the second row folded into the first", got)
	}
}
