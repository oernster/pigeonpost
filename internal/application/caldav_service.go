package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// CalDAVService is the account-aware orchestrator for two-way DAV sync. It adds and removes DAV accounts
// (storing the account and its keychain password together), lists them and pulls an account's calendars
// into the local store. The per-pull source is built from the account and its password through the
// CalDAVSourceFactory, so a different server or credential is used for each account.
type CalDAVService struct {
	accounts    CalendarAccountStore
	credentials CalendarCredentialStore
	factory     CalDAVSourceFactory
	codec       CalendarCodec
	calendar    CalendarSyncStore
}

// NewCalDAVService wires the orchestrator over its stores, credential vault, source factory and codec.
func NewCalDAVService(
	accounts CalendarAccountStore,
	credentials CalendarCredentialStore,
	factory CalDAVSourceFactory,
	codec CalendarCodec,
	calendar CalendarSyncStore,
) *CalDAVService {
	return &CalDAVService{accounts: accounts, credentials: credentials, factory: factory, codec: codec, calendar: calendar}
}

// AddAccount stores a DAV account and its password. The password is written to the keychain first so a
// stored account is never left without its credential.
func (s *CalDAVService) AddAccount(ctx context.Context, account domain.CalendarAccount, password string) error {
	if err := s.credentials.SetCalendarPassword(ctx, account, password); err != nil {
		return fmt.Errorf("caldav: store password: %w", err)
	}
	if err := s.accounts.SaveCalendarAccount(ctx, account); err != nil {
		return fmt.Errorf("caldav: save account: %w", err)
	}
	return nil
}

// ListAccounts returns every configured DAV account.
func (s *CalDAVService) ListAccounts(ctx context.Context) ([]domain.CalendarAccount, error) {
	return s.accounts.ListCalendarAccounts(ctx)
}

// RemoveAccount deletes a DAV account and its keychain password.
func (s *CalDAVService) RemoveAccount(ctx context.Context, id string) error {
	account, err := s.accounts.GetCalendarAccount(ctx, id)
	if err != nil {
		return fmt.Errorf("caldav: account: %w", err)
	}
	if err := s.accounts.DeleteCalendarAccount(ctx, id); err != nil {
		return fmt.Errorf("caldav: delete account: %w", err)
	}
	if err := s.credentials.DeleteCalendarPassword(ctx, account); err != nil {
		return fmt.Errorf("caldav: delete password: %w", err)
	}
	return nil
}

// Pull fetches the account's calendars into the local store, returning the number of events saved. It
// resolves the account and its password, builds the source and delegates to a read-only CalDAVSyncService.
func (s *CalDAVService) Pull(ctx context.Context, accountID string) (int, error) {
	account, err := s.accounts.GetCalendarAccount(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("caldav: account: %w", err)
	}
	password, err := s.credentials.CalendarPassword(ctx, account)
	if err != nil {
		return 0, fmt.Errorf("caldav: password: %w", err)
	}
	source, err := s.factory.NewSource(account, password)
	if err != nil {
		return 0, fmt.Errorf("caldav: source: %w", err)
	}
	return NewCalDAVSyncService(source, s.codec, s.calendar, accountID).Pull(ctx)
}
