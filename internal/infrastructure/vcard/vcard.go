// Package vcard converts contacts to and from vCard (RFC 6350), the address-book format Thunderbird and
// single-contact Outlook import and export. It implements the application ContactCodec port and depends
// only on the domain and the go-vcard library.
package vcard

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	govcard "github.com/emersion/go-vcard"

	"github.com/oernster/pigeonpost/internal/domain"
)

// generatedIDBytes is the length of a random id assigned to a card that carries no UID.
const generatedIDBytes = 16

// Codec is the vCard implementation of the application ContactCodec port.
type Codec struct{}

// New constructs a vCard codec.
func New() Codec { return Codec{} }

// Decode parses one or more vCards into contacts. A card's UID becomes the contact id so a re-import
// updates the same record; a card without a UID is given a generated id (also used as its UID) so a
// later export still round-trips.
func (Codec) Decode(data []byte) ([]domain.Contact, error) {
	dec := govcard.NewDecoder(bytes.NewReader(data))
	var contacts []domain.Contact
	for {
		card, err := dec.Decode()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("vcard: decode: %w", err)
		}
		contact, err := cardToContact(card)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, contact)
	}
	return contacts, nil
}

// cardToContact rebuilds a validated domain contact from a parsed card.
func cardToContact(card govcard.Card) (domain.Contact, error) {
	uid := strings.TrimSpace(card.Value(govcard.FieldUID))
	id := uid
	if id == "" {
		id = generatedID()
		uid = id
	}
	given, family := "", ""
	if name := card.Name(); name != nil {
		given = strings.TrimSpace(name.GivenName)
		family = strings.TrimSpace(name.FamilyName)
	}
	fn := strings.TrimSpace(card.PreferredValue(govcard.FieldFormattedName))
	if fn == "" {
		fn = strings.TrimSpace(given + " " + family)
	}
	return domain.NewContact(domain.ContactInput{
		ID:            id,
		UID:           uid,
		FormattedName: fn,
		GivenName:     given,
		FamilyName:    family,
		Organization:  card.Value(govcard.FieldOrganization),
		Title:         card.Value(govcard.FieldTitle),
		Note:          card.Value(govcard.FieldNote),
		Birthday:      strings.TrimSpace(card.Value(govcard.FieldBirthday)),
		Emails:        cardEmails(card),
		Phones:        cardPhones(card),
		Addresses:     cardAddresses(card),
	})
}

// cardEmails reads the card's EMAIL fields, skipping any that will not parse rather than failing the
// whole import.
func cardEmails(card govcard.Card) []domain.ContactEmail {
	var emails []domain.ContactEmail
	for _, f := range card[govcard.FieldEmail] {
		email, err := domain.NewContactEmail(firstType(f.Params), strings.TrimSpace(f.Value))
		if err != nil {
			continue
		}
		emails = append(emails, email)
	}
	return emails
}

// cardPhones reads the card's TEL fields, skipping any that will not parse.
func cardPhones(card govcard.Card) []domain.ContactPhone {
	var phones []domain.ContactPhone
	for _, f := range card[govcard.FieldTelephone] {
		phone, err := domain.NewContactPhone(firstType(f.Params), strings.TrimSpace(f.Value))
		if err != nil {
			continue
		}
		phones = append(phones, phone)
	}
	return phones
}

// cardAddresses reads the card's ADR fields, skipping any whose components are all empty (which would
// not form a valid address) rather than failing the whole import.
func cardAddresses(card govcard.Card) []domain.ContactAddress {
	var addresses []domain.ContactAddress
	for _, a := range card.Addresses() {
		address, err := domain.NewContactAddress(firstType(a.Params), a.StreetAddress, a.Locality,
			a.Region, a.PostalCode, a.Country)
		if err != nil {
			continue
		}
		addresses = append(addresses, address)
	}
	return addresses
}

// firstType returns the first TYPE parameter (the label), or an empty string when there is none.
func firstType(p govcard.Params) string {
	types := p[govcard.ParamType]
	if len(types) == 0 {
		return ""
	}
	return types[0]
}

// Encode writes every contact as a vCard 4.0 record. A contact with no UID uses its id as the UID so
// the export round-trips.
func (Codec) Encode(contacts []domain.Contact) ([]byte, error) {
	var buf bytes.Buffer
	enc := govcard.NewEncoder(&buf)
	for _, c := range contacts {
		if err := enc.Encode(contactToCard(c)); err != nil {
			return nil, fmt.Errorf("vcard: encode %q: %w", c.ID(), err)
		}
	}
	return buf.Bytes(), nil
}

// contactToCard builds a vCard 4.0 card from a contact.
func contactToCard(c domain.Contact) govcard.Card {
	card := govcard.Card{}
	uid := c.UID()
	if uid == "" {
		uid = c.ID()
	}
	card.SetValue(govcard.FieldUID, uid)
	card.SetValue(govcard.FieldFormattedName, c.FormattedName())
	// N is FamilyName;GivenName;AdditionalName;HonorificPrefix;HonorificSuffix.
	card.Add(govcard.FieldName, &govcard.Field{Value: c.FamilyName() + ";" + c.GivenName() + ";;;"})
	setIfPresent(card, govcard.FieldOrganization, c.Organization())
	setIfPresent(card, govcard.FieldTitle, c.Title())
	setIfPresent(card, govcard.FieldNote, c.Note())
	setIfPresent(card, govcard.FieldBirthday, c.Birthday())
	for _, e := range c.Emails() {
		card.Add(govcard.FieldEmail, &govcard.Field{Value: e.Address().Address(), Params: typeParam(e.Label())})
	}
	for _, p := range c.Phones() {
		card.Add(govcard.FieldTelephone, &govcard.Field{Value: p.Number(), Params: typeParam(p.Label())})
	}
	for _, a := range c.Addresses() {
		// ADR is PostOfficeBox;ExtendedAddress;StreetAddress;Locality;Region;PostalCode;Country; the two
		// leading semicolons leave the PO-box and extended-address components empty.
		value := ";;" + a.Street() + ";" + a.Locality() + ";" + a.Region() + ";" + a.PostalCode() + ";" + a.Country()
		card.Add(govcard.FieldAddress, &govcard.Field{Value: value, Params: typeParam(a.Label())})
	}
	govcard.ToV4(card)
	return card
}

// setIfPresent sets a single-valued field only when the value is non-empty, so absent fields are
// omitted rather than written blank.
func setIfPresent(card govcard.Card, field, value string) {
	if value != "" {
		card.SetValue(field, value)
	}
}

// typeParam builds a TYPE parameter for a label, or nil when the label is empty.
func typeParam(label string) govcard.Params {
	if label == "" {
		return nil
	}
	return govcard.Params{govcard.ParamType: []string{label}}
}

// generatedID returns a random hex id for a card that carries no UID.
func generatedID() string {
	var b [generatedIDBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "contact"
	}
	return hex.EncodeToString(b[:])
}
