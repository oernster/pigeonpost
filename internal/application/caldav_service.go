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
	writers     CalDAVWriterFactory
	codec       CalendarCodec
	calendar    CalendarSyncStore
	newID       func() string
}

// NewCalDAVService wires the orchestrator over its stores, credential vault, source and writer factories, the
// codec and an id generator (used for the reconcile's safety-copy rows).
func NewCalDAVService(
	accounts CalendarAccountStore,
	credentials CalendarCredentialStore,
	factory CalDAVSourceFactory,
	writers CalDAVWriterFactory,
	codec CalendarCodec,
	calendar CalendarSyncStore,
	newID func() string,
) *CalDAVService {
	return &CalDAVService{
		accounts: accounts, credentials: credentials, factory: factory, writers: writers,
		codec: codec, calendar: calendar, newID: newID,
	}
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

// Sync runs the two-way sync for an account: it pushes the account's pending local changes to the server, then
// discovers its collections and reconciles the server's state into the local store. The order mirrors the tag
// two-way sync (flush before the pull so the pull reflects the pushed changes, reconcile after). The flush and
// the reconcile are best-effort, so a transient failure in either leaves the pending intents in place for the
// next run rather than failing the whole sync; only resolving the account, its password, the source, the
// writer or discovering the collections is fatal, since without those there is nothing to sync.
func (s *CalDAVService) Sync(ctx context.Context, accountID string) error {
	account, err := s.accounts.GetCalendarAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("caldav: account: %w", err)
	}
	password, err := s.credentials.CalendarPassword(ctx, account)
	if err != nil {
		return fmt.Errorf("caldav: password: %w", err)
	}
	source, err := s.factory.NewSource(account, password)
	if err != nil {
		return fmt.Errorf("caldav: source: %w", err)
	}
	writer, err := s.writers.NewWriter(account, password)
	if err != nil {
		return fmt.Errorf("caldav: writer: %w", err)
	}
	_ = NewCalDAVWriteService(s.calendar, s.codec).Flush(ctx, writer)
	records, err := NewCalDAVSyncService(source, s.codec, s.calendar, accountID).Discover(ctx)
	if err != nil {
		return err
	}
	_ = NewCalDAVReconcileService(s.calendar, s.codec, s.newID).Reconcile(ctx, source, records)
	return nil
}
