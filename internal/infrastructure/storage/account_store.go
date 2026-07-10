package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

const accountColumns = `id, display_name, email, protocol,
	in_host, in_port, in_security, out_host, out_port, out_security, auth, signature, identities`

// identityRow is the JSON shape of one stored sender identity: its display name and bare address.
type identityRow struct {
	Display string `json:"display"`
	Address string `json:"address"`
}

// ListAccounts returns all accounts in the user's chosen sidebar order (the position column), falling
// back to display name for accounts that share a position (all of them until the first manual reorder).
func (s *Store) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	return queryRows(ctx, s.db, "accounts",
		"SELECT "+accountColumns+" FROM account ORDER BY position, display_name;", scanAccount)
}

// GetAccount returns the account with the given id, or application.ErrAccountNotFound.
func (s *Store) GetAccount(ctx context.Context, id string) (domain.Account, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+accountColumns+" FROM account WHERE id = ?;", id)
	account, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, application.ErrAccountNotFound
	}
	if err != nil {
		return domain.Account{}, err
	}
	return account, nil
}

// SaveAccount inserts or replaces an account.
func (s *Store) SaveAccount(ctx context.Context, a domain.Account) error {
	identities, err := encodeIdentities(a.Identities())
	if err != nil {
		return fmt.Errorf("save account %q: %w", a.ID(), err)
	}
	// position is not a domain attribute, so it is set here rather than carried on the account. An
	// existing row keeps its position (the subquery is evaluated against the table before the REPLACE
	// removes the old row, so editing an account does not reset the sidebar order); a brand-new account
	// is appended at the end: its position is the current maximum plus one (0 when it is the first).
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO account (`+accountColumns+`, position)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(
		     (SELECT position FROM account WHERE id = ?),
		     (SELECT COALESCE(MAX(position) + 1, 0) FROM account)));`,
		a.ID(), a.DisplayName(), a.Address().Address(), int(a.Protocol()),
		a.Incoming().Host(), a.Incoming().Port(), int(a.Incoming().Security()),
		a.Outgoing().Host(), a.Outgoing().Port(), int(a.Outgoing().Security()),
		int(a.Auth()), a.Signature(), identities, a.ID(),
	)
	if err != nil {
		return fmt.Errorf("save account %q: %w", a.ID(), err)
	}
	return nil
}

// SetAccountPositions writes each account's sidebar position from the given order: the account at index
// i in orderedIDs gets position i. It runs in one transaction so the reorder is atomic; it rewrites the
// full list rather than swapping pairs so positions can never collide. Ids not present are left as they
// are; an id in the list that no longer exists updates no row and is harmless.
func (s *Store) SetAccountPositions(ctx context.Context, orderedIDs []string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for i, id := range orderedIDs {
			if _, err := tx.ExecContext(ctx,
				"UPDATE account SET position = ? WHERE id = ?;", i, id); err != nil {
				return fmt.Errorf("set account %q position: %w", id, err)
			}
		}
		return nil
	})
}

// encodeIdentities serialises an account's identities to the stored JSON array.
func encodeIdentities(identities []domain.EmailAddress) (string, error) {
	rows := make([]identityRow, 0, len(identities))
	for _, id := range identities {
		rows = append(rows, identityRow{Display: id.Display(), Address: id.Address()})
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return "", fmt.Errorf("encode identities: %w", err)
	}
	return string(data), nil
}

// decodeIdentities parses the stored JSON array back into validated addresses, skipping any that no
// longer parse so one bad row cannot make the whole account unreadable.
func decodeIdentities(data string) ([]domain.EmailAddress, error) {
	var rows []identityRow
	if err := json.Unmarshal([]byte(data), &rows); err != nil {
		return nil, fmt.Errorf("decode identities: %w", err)
	}
	identities := make([]domain.EmailAddress, 0, len(rows))
	for _, row := range rows {
		addr, err := domain.NewEmailAddress(row.Display, row.Address)
		if err != nil {
			continue
		}
		identities = append(identities, addr)
	}
	return identities, nil
}

// DeleteAccount removes an account row. Deleting an account that does not exist is not an error.
func (s *Store) DeleteAccount(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM account WHERE id = ?;", id); err != nil {
		return fmt.Errorf("delete account %q: %w", id, err)
	}
	return nil
}

// scanner abstracts *sql.Row and *sql.Rows so scanAccount can serve both.
type scanner interface {
	Scan(dest ...any) error
}

func scanAccount(row scanner) (domain.Account, error) {
	var (
		id, displayName, email                   string
		protocol, auth                           int
		inHost, outHost                          string
		inPort, inSecurity, outPort, outSecurity int
		signature, identitiesJSON                string
	)
	if err := row.Scan(&id, &displayName, &email, &protocol,
		&inHost, &inPort, &inSecurity, &outHost, &outPort, &outSecurity, &auth, &signature, &identitiesJSON); err != nil {
		return domain.Account{}, err
	}
	identities, err := decodeIdentities(identitiesJSON)
	if err != nil {
		return domain.Account{}, fmt.Errorf("account %q identities: %w", id, err)
	}

	address, err := domain.NewEmailAddress("", email)
	if err != nil {
		return domain.Account{}, fmt.Errorf("account %q address: %w", id, err)
	}
	incoming, err := domain.NewServerConfig(inHost, inPort, domain.Security(inSecurity))
	if err != nil {
		return domain.Account{}, fmt.Errorf("account %q incoming: %w", id, err)
	}
	outgoing, err := domain.NewServerConfig(outHost, outPort, domain.Security(outSecurity))
	if err != nil {
		return domain.Account{}, fmt.Errorf("account %q outgoing: %w", id, err)
	}
	account, err := domain.NewAccount(id, displayName, address,
		domain.Protocol(protocol), incoming, outgoing, domain.AuthMethod(auth))
	if err != nil {
		return domain.Account{}, fmt.Errorf("account %q: %w", id, err)
	}
	return account.WithSignature(signature).WithIdentities(identities), nil
}
