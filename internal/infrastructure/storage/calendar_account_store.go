package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

const calendarAccountColumns = `id, display_name, base_url, username, auth`

// ListCalendarAccounts returns every stored CalDAV/CardDAV account, ordered by display name.
func (s *Store) ListCalendarAccounts(ctx context.Context) ([]domain.CalendarAccount, error) {
	return queryRows(ctx, s.db, "calendar accounts",
		"SELECT "+calendarAccountColumns+" FROM calendar_account ORDER BY display_name;", scanCalendarAccount)
}

// GetCalendarAccount returns the account with the given id, or application.ErrCalendarAccountNotFound.
func (s *Store) GetCalendarAccount(ctx context.Context, id string) (domain.CalendarAccount, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+calendarAccountColumns+" FROM calendar_account WHERE id = ?;", id)
	account, err := scanCalendarAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CalendarAccount{}, application.ErrCalendarAccountNotFound
	}
	if err != nil {
		return domain.CalendarAccount{}, err
	}
	return account, nil
}

// SaveCalendarAccount inserts or replaces a CalDAV/CardDAV account. The password is not stored here; it
// lives in the OS keychain, keyed by the account id.
func (s *Store) SaveCalendarAccount(ctx context.Context, a domain.CalendarAccount) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO calendar_account (`+calendarAccountColumns+`) VALUES (?, ?, ?, ?, ?);`,
		a.ID(), a.DisplayName(), a.BaseURL(), a.Username(), int(a.Auth()),
	)
	if err != nil {
		return fmt.Errorf("save calendar account %q: %w", a.ID(), err)
	}
	return nil
}

// DeleteCalendarAccount removes an account row. Deleting one that does not exist is not an error.
func (s *Store) DeleteCalendarAccount(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM calendar_account WHERE id = ?;", id); err != nil {
		return fmt.Errorf("delete calendar account %q: %w", id, err)
	}
	return nil
}

func scanCalendarAccount(row scanner) (domain.CalendarAccount, error) {
	var (
		id, displayName, baseURL, username string
		auth                               int
	)
	if err := row.Scan(&id, &displayName, &baseURL, &username, &auth); err != nil {
		return domain.CalendarAccount{}, err
	}
	account, err := domain.NewCalendarAccount(id, displayName, baseURL, username, domain.AuthMethod(auth))
	if err != nil {
		return domain.CalendarAccount{}, fmt.Errorf("calendar account %q: %w", id, err)
	}
	return account, nil
}
