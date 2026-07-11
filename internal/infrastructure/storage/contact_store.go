package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// contactRow holds a contact's base columns before its emails, phones and addresses are attached.
type contactRow struct {
	id, uid, formattedName, given, family, org, title, note, birthday string
}

// ListContacts returns every contact with its emails, phones and addresses, ordered by formatted name.
func (s *Store) ListContacts(ctx context.Context) ([]domain.Contact, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, uid, formatted_name, given_name, family_name, organization, title, note, birthday
		 FROM contact ORDER BY formatted_name;`)
	if err != nil {
		return nil, fmt.Errorf("query contacts: %w", err)
	}
	var bases []contactRow
	for rows.Next() {
		var b contactRow
		if err := rows.Scan(&b.id, &b.uid, &b.formattedName, &b.given, &b.family, &b.org, &b.title, &b.note, &b.birthday); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		bases = append(bases, b)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate contacts: %w", err)
	}
	// Close before assembling, since each contact issues further queries on the same connection.
	_ = rows.Close()

	contacts := make([]domain.Contact, 0, len(bases))
	for _, b := range bases {
		c, err := s.assembleContact(ctx, b)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, nil
}

// GetContact returns one contact by id, with its emails and phones.
func (s *Store) GetContact(ctx context.Context, id string) (domain.Contact, error) {
	var b contactRow
	err := s.db.QueryRowContext(ctx,
		`SELECT id, uid, formatted_name, given_name, family_name, organization, title, note, birthday
		 FROM contact WHERE id = ?;`, id).
		Scan(&b.id, &b.uid, &b.formattedName, &b.given, &b.family, &b.org, &b.title, &b.note, &b.birthday)
	if err != nil {
		return domain.Contact{}, fmt.Errorf("get contact %q: %w", id, err)
	}
	return s.assembleContact(ctx, b)
}

// assembleContact loads a contact's emails, phones and addresses and rebuilds the validated domain
// value.
func (s *Store) assembleContact(ctx context.Context, b contactRow) (domain.Contact, error) {
	emails, err := s.contactEmails(ctx, b.id)
	if err != nil {
		return domain.Contact{}, err
	}
	phones, err := s.contactPhones(ctx, b.id)
	if err != nil {
		return domain.Contact{}, err
	}
	addresses, err := s.contactAddresses(ctx, b.id)
	if err != nil {
		return domain.Contact{}, err
	}
	c, err := domain.NewContact(domain.ContactInput{
		ID: b.id, UID: b.uid, FormattedName: b.formattedName, GivenName: b.given,
		FamilyName: b.family, Organization: b.org, Title: b.title, Note: b.note, Birthday: b.birthday,
		Emails: emails, Phones: phones, Addresses: addresses,
	})
	if err != nil {
		return domain.Contact{}, fmt.Errorf("rebuild contact %q: %w", b.id, err)
	}
	return c, nil
}

// contactEmails returns a contact's labelled emails in stored order.
func (s *Store) contactEmails(ctx context.Context, contactID string) ([]domain.ContactEmail, error) {
	return queryRows(ctx, s.db, "contact emails",
		"SELECT label, address FROM contact_email WHERE contact_id = ? ORDER BY position;",
		func(row scanner) (domain.ContactEmail, error) {
			var label, address string
			if err := row.Scan(&label, &address); err != nil {
				return domain.ContactEmail{}, fmt.Errorf("scan contact email: %w", err)
			}
			email, err := domain.NewContactEmail(label, address)
			if err != nil {
				return domain.ContactEmail{}, fmt.Errorf("rebuild contact email: %w", err)
			}
			return email, nil
		}, contactID)
}

// contactPhones returns a contact's labelled phone numbers in stored order.
func (s *Store) contactPhones(ctx context.Context, contactID string) ([]domain.ContactPhone, error) {
	return queryRows(ctx, s.db, "contact phones",
		"SELECT label, number FROM contact_phone WHERE contact_id = ? ORDER BY position;",
		func(row scanner) (domain.ContactPhone, error) {
			var label, number string
			if err := row.Scan(&label, &number); err != nil {
				return domain.ContactPhone{}, fmt.Errorf("scan contact phone: %w", err)
			}
			phone, err := domain.NewContactPhone(label, number)
			if err != nil {
				return domain.ContactPhone{}, fmt.Errorf("rebuild contact phone: %w", err)
			}
			return phone, nil
		}, contactID)
}

// contactAddresses returns a contact's labelled postal addresses in stored order.
func (s *Store) contactAddresses(ctx context.Context, contactID string) ([]domain.ContactAddress, error) {
	return queryRows(ctx, s.db, "contact addresses",
		"SELECT label, street, locality, region, postal_code, country FROM contact_address WHERE contact_id = ? ORDER BY position;",
		func(row scanner) (domain.ContactAddress, error) {
			var label, street, locality, region, postalCode, country string
			if err := row.Scan(&label, &street, &locality, &region, &postalCode, &country); err != nil {
				return domain.ContactAddress{}, fmt.Errorf("scan contact address: %w", err)
			}
			address, err := domain.NewContactAddress(label, street, locality, region, postalCode, country)
			if err != nil {
				return domain.ContactAddress{}, fmt.Errorf("rebuild contact address: %w", err)
			}
			return address, nil
		}, contactID)
}

// SaveContact inserts or updates a contact and replaces its emails and phones in one transaction, so a
// save is idempotent.
func (s *Store) SaveContact(ctx context.Context, c domain.Contact) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO contact (id, uid, formatted_name, given_name, family_name, organization, title, note, birthday)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET uid = excluded.uid, formatted_name = excluded.formatted_name,
			     given_name = excluded.given_name, family_name = excluded.family_name,
			     organization = excluded.organization, title = excluded.title, note = excluded.note,
			     birthday = excluded.birthday;`,
			c.ID(), c.UID(), c.FormattedName(), c.GivenName(), c.FamilyName(),
			c.Organization(), c.Title(), c.Note(), c.Birthday()); err != nil {
			return fmt.Errorf("save contact %q: %w", c.ID(), err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM contact_email WHERE contact_id = ?;", c.ID()); err != nil {
			return fmt.Errorf("clear contact emails: %w", err)
		}
		for i, e := range c.Emails() {
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO contact_email (contact_id, position, label, address) VALUES (?, ?, ?, ?);",
				c.ID(), i, e.Label(), e.Address().Address()); err != nil {
				return fmt.Errorf("insert contact email: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM contact_phone WHERE contact_id = ?;", c.ID()); err != nil {
			return fmt.Errorf("clear contact phones: %w", err)
		}
		for i, p := range c.Phones() {
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO contact_phone (contact_id, position, label, number) VALUES (?, ?, ?, ?);",
				c.ID(), i, p.Label(), p.Number()); err != nil {
				return fmt.Errorf("insert contact phone: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM contact_address WHERE contact_id = ?;", c.ID()); err != nil {
			return fmt.Errorf("clear contact addresses: %w", err)
		}
		for i, a := range c.Addresses() {
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO contact_address (contact_id, position, label, street, locality, region, postal_code, country) VALUES (?, ?, ?, ?, ?, ?, ?, ?);",
				c.ID(), i, a.Label(), a.Street(), a.Locality(), a.Region(), a.PostalCode(), a.Country()); err != nil {
				return fmt.Errorf("insert contact address: %w", err)
			}
		}
		return nil
	})
}

// DeleteContact removes a contact with its emails, phones and group memberships in one transaction.
func (s *Store) DeleteContact(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for _, stmt := range []string{
			"DELETE FROM contact_email WHERE contact_id = ?;",
			"DELETE FROM contact_phone WHERE contact_id = ?;",
			"DELETE FROM contact_address WHERE contact_id = ?;",
			"DELETE FROM contact_group_member WHERE contact_id = ?;",
			"DELETE FROM contact WHERE id = ?;",
		} {
			if _, err := tx.ExecContext(ctx, stmt, id); err != nil {
				return fmt.Errorf("delete contact %q: %w", id, err)
			}
		}
		return nil
	})
}
