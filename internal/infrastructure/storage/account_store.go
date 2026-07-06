package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

const accountColumns = `id, display_name, email, protocol,
	in_host, in_port, in_security, out_host, out_port, out_security, auth, signature`

// ListAccounts returns all accounts ordered by display name.
func (s *Store) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+accountColumns+" FROM account ORDER BY display_name;")
	if err != nil {
		return nil, fmt.Errorf("query accounts: %w", err)
	}
	defer rows.Close()

	var accounts []domain.Account
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accounts: %w", err)
	}
	return accounts, nil
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
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO account (`+accountColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		a.ID(), a.DisplayName(), a.Address().Address(), int(a.Protocol()),
		a.Incoming().Host(), a.Incoming().Port(), int(a.Incoming().Security()),
		a.Outgoing().Host(), a.Outgoing().Port(), int(a.Outgoing().Security()),
		int(a.Auth()), a.Signature(),
	)
	if err != nil {
		return fmt.Errorf("save account %q: %w", a.ID(), err)
	}
	return nil
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
		signature                                string
	)
	if err := row.Scan(&id, &displayName, &email, &protocol,
		&inHost, &inPort, &inSecurity, &outHost, &outPort, &outSecurity, &auth, &signature); err != nil {
		return domain.Account{}, err
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
	return account.WithSignature(signature), nil
}
