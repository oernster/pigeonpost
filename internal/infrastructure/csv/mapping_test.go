package csv

import (
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// addressByLabel indexes a contact's addresses so a test can assert on one without depending on order.
func addressByLabel(c domain.Contact) map[string]domain.ContactAddress {
	out := map[string]domain.ContactAddress{}
	for _, a := range c.Addresses() {
		out[a.Label()] = a
	}
	return out
}

// phonesByLabel collects a contact's phone numbers under each label.
func phonesByLabel(c domain.Contact) map[string][]string {
	out := map[string][]string{}
	for _, p := range c.Phones() {
		out[p.Label()] = append(out[p.Label()], p.Number())
	}
	return out
}

func TestDecodeOutlookExport(t *testing.T) {
	data := lines(
		"First Name,Middle Name,Last Name,Title,Suffix,Birthday,Notes,E-mail Address,E-mail 2 Address,"+
			"Home Phone,Mobile Phone,Business Phone,Home Address,Home Street,Home City,Home State,"+
			"Home Postal Code,Home Country,Business Street,Business City,Business State,"+
			"Business Postal Code,Business Country/Region,Job Title,Department,Company",
		"Amy,Jessica,Pond,Mrs,,1/15/1980,a note,amy@example.com,amy2@example.com,"+
			"555-1000,555-2000,555-3000,\"12 Leadworth Lane\nLeadworth\",12 Leadworth Lane,Leadworth,"+
			"Gloucestershire,GL1 2AB,UK,1 Tardis Way,London,Greater London,SW1A 1AA,UK,Companion,Travel,Tardis",
	)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d, want 1", len(got))
	}
	c := got[0]
	if c.FormattedName() != "Amy Jessica Pond" {
		t.Errorf("display name = %q; Outlook has no display-name column, so it is built from the parts", c.FormattedName())
	}
	if c.Organization() != "Tardis" || c.Note() != "a note" {
		t.Errorf("organisation/note = %q/%q", c.Organization(), c.Note())
	}
	if c.Title() != "Companion" {
		t.Errorf("title = %q, want the job title rather than the honorific", c.Title())
	}
	if c.Birthday() != "1980-01-15" {
		t.Errorf("birthday = %q, want the normalised ISO date", c.Birthday())
	}
	if len(c.Emails()) != 2 {
		t.Errorf("emails = %+v, want both slots", c.Emails())
	}
	addresses := addressByLabel(c)
	home, ok := addresses["home"]
	if !ok {
		t.Fatalf("no home address decoded from %+v", c.Addresses())
	}
	// Outlook exports the combined "Home Address" as well as the street lines. Taking the street lines
	// is what keeps the city out of the street component.
	if home.Street() != "12 Leadworth Lane" || home.Locality() != "Leadworth" ||
		home.Region() != "Gloucestershire" || home.PostalCode() != "GL1 2AB" || home.Country() != "UK" {
		t.Errorf("home address = %+v", home)
	}
	work, ok := addresses["work"]
	if !ok {
		t.Fatalf("no work address decoded from %+v", c.Addresses())
	}
	if work.Street() != "1 Tardis Way" || work.Locality() != "London" ||
		work.Region() != "Greater London" || work.PostalCode() != "SW1A 1AA" || work.Country() != "UK" {
		t.Errorf("work address = %+v", work)
	}
}

func TestDecodeOutlookHonorificIsNotAJobTitle(t *testing.T) {
	// Job Title is present but empty, so the honorific in Title must not be promoted into it.
	data := lines(
		"First Name,Last Name,Title,Job Title,E-mail Address",
		"River,Song,Dr,,river@example.com",
	)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 || got[0].Title() != "" {
		t.Errorf("title = %q, want empty rather than the honorific", got[0].Title())
	}
}

func TestDecodeBareTitleUsedWhenNoJobTitleColumn(t *testing.T) {
	// With no Job Title column the file is not an Outlook export, so a bare Title is the role.
	data := lines("Display Name,Title,E-mail Address", "River Song,Professor,river@example.com")
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 || got[0].Title() != "Professor" {
		t.Errorf("title = %q, want the bare Title column", got[0].Title())
	}
}

func TestDecodeThunderbirdUKExport(t *testing.T) {
	// The exact header Thunderbird writes on a UK build, including its Organisation spelling and the
	// County and Post Code wording that a US build words as State and ZipCode.
	data := lines(
		"First Name,Last Name,Display Name,Nickname,Primary Email,Secondary Email,Screen Name,Work Phone,"+
			"Home Phone,Fax Number,Pager Number,Mobile Number,Home Address,Home Address 2,Home City,"+
			"Home County,Home Post Code,Home Country,Work Address,Work Address 2,Work City,Work County,"+
			"Work Post Code,Work Country,Job Title,Department,Organisation,Web Page 1,Web Page 2,"+
			"Birth Year,Birth Month,Birth Day,Custom 1,Custom 2,Custom 3,Custom 4,Notes",
		"Rory,Williams,Rory Williams,,rory@example.com,rory2@example.com,,555-3000,555-1000,555-4000,"+
			"555-5000,555-2000,12 Leadworth Lane,Flat B,Leadworth,Gloucestershire,GL1 2AB,UK,"+
			"1 Tardis Way,,London,Greater London,SW1A 1AA,UK,Nurse,Ward,Leadworth Hospital,,,"+
			"1980,1,15,,,,,a note",
	)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d, want 1", len(got))
	}
	c := got[0]
	if c.FormattedName() != "Rory Williams" || c.Organization() != "Leadworth Hospital" || c.Title() != "Nurse" {
		t.Errorf("core fields = %+v", c)
	}
	if c.Birthday() != "1980-01-15" {
		t.Errorf("birthday = %q, want the three columns combined", c.Birthday())
	}
	phones := phonesByLabel(c)
	if len(phones["mobile"]) != 1 || len(phones["home"]) != 1 || len(phones["work"]) != 1 ||
		len(phones["fax"]) != 1 || len(phones["pager"]) != 1 {
		t.Errorf("phones by label = %v, want all five Thunderbird columns read", phones)
	}
	addresses := addressByLabel(c)
	home, ok := addresses["home"]
	if !ok {
		t.Fatalf("no home address decoded from %+v", c.Addresses())
	}
	// Thunderbird splits the street over two columns, which join into the single domain component.
	if home.Street() != "12 Leadworth Lane, Flat B" || home.Region() != "Gloucestershire" ||
		home.PostalCode() != "GL1 2AB" {
		t.Errorf("home address = %+v", home)
	}
	if work := addresses["work"]; work.Street() != "1 Tardis Way" || work.Region() != "Greater London" {
		t.Errorf("work address = %+v", work)
	}
}

func TestDecodeThunderbirdUSWording(t *testing.T) {
	// A US Thunderbird build words the same columns State, ZipCode and Organization.
	data := lines(
		"Display Name,Primary Email,Home Address,Home City,Home State,Home ZipCode,Home Country,Organization",
		"Jack Harkness,jack@example.com,1 Roald Dahl Plass,Cardiff,Wales,CF10 5AL,UK,Torchwood",
	)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d, want 1", len(got))
	}
	if got[0].Organization() != "Torchwood" {
		t.Errorf("organization = %q, want the US spelling matched", got[0].Organization())
	}
	home := addressByLabel(got[0])["home"]
	if home.Street() != "1 Roald Dahl Plass" || home.Region() != "Wales" || home.PostalCode() != "CF10 5AL" {
		t.Errorf("home address = %+v, want State and ZipCode matched", home)
	}
}

func TestDecodeRepeatedNumberStoredOnce(t *testing.T) {
	// Both exporters happily write the same number into several columns.
	data := lines(
		"Display Name,Home Phone,Business Phone,Mobile Phone",
		"Jo Bloggs,555-1000,555-1000,555-2000",
	)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got[0].Phones()) != 2 {
		t.Errorf("phones = %+v, want the repeated number stored once", got[0].Phones())
	}
}

func TestDecodeAddressBlockAllEmptyIsSkipped(t *testing.T) {
	data := lines(
		"Display Name,Primary Email,Home Address,Home City,Home Country",
		"Jo Bloggs,jo@example.com,,,",
	)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got[0].Addresses()) != 0 {
		t.Errorf("addresses = %+v, want an entirely empty block skipped", got[0].Addresses())
	}
}

func TestDecodeShortRowIsTolerated(t *testing.T) {
	// Outlook drops trailing empty columns, so a row can be shorter than the header.
	data := lines(
		"First Name,Last Name,E-mail Address,Home City,Notes",
		"Jo,Bloggs,jo@example.com",
	)
	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 1 || got[0].FormattedName() != "Jo Bloggs" {
		t.Errorf("short row not tolerated: %+v", got)
	}
}
