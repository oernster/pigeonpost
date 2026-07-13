package main

import (
	"github.com/oernster/pigeonpost/internal/domain"
)

// CalDAVAccountDTO is the JSON-serialisable view of a CalDAV account sent to the front end. The password
// is never part of this view; it lives in the OS keychain, exactly as for a mail account.
type CalDAVAccountDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	BaseURL     string `json:"baseUrl"`
	Username    string `json:"username"`
}

// toCalDAVAccountDTO maps a domain CalendarAccount to its front-end view, dropping nothing but the secret.
func toCalDAVAccountDTO(a domain.CalendarAccount) CalDAVAccountDTO {
	return CalDAVAccountDTO{
		ID:          a.ID(),
		DisplayName: a.DisplayName(),
		BaseURL:     a.BaseURL(),
		Username:    a.Username(),
	}
}

// AddCalDAVAccount adds a password-authenticated CalDAV account. It builds the domain account (which
// validates the base URL and username), then hands it and its password to the service, which stores the
// password in the keychain before persisting the account.
func (a *App) AddCalDAVAccount(displayName, baseURL, username, password string) error {
	account, err := domain.NewCalendarAccount(newCalendarAccountID(), displayName, baseURL, username, domain.AuthPassword)
	if err != nil {
		return err
	}
	return a.caldav.AddAccount(a.ctx, account, password)
}

// ListCalDAVAccounts returns every configured CalDAV account for the front end to list.
func (a *App) ListCalDAVAccounts() ([]CalDAVAccountDTO, error) {
	accounts, err := a.caldav.ListAccounts(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CalDAVAccountDTO, 0, len(accounts))
	for _, account := range accounts {
		out = append(out, toCalDAVAccountDTO(account))
	}
	return out, nil
}

// RemoveCalDAVAccount deletes a CalDAV account together with its keychain password.
func (a *App) RemoveCalDAVAccount(id string) error {
	return a.caldav.RemoveAccount(a.ctx, id)
}

// PullCalDAV fetches the account's remote calendars into the local store, returning the number of events
// saved. This is the one-way phase-1 pull. Prefer SyncCalDAV once the account has local edits: a naive pull
// overwrites an unpushed local change, which the two-way sync guards against.
func (a *App) PullCalDAV(id string) (int, error) {
	return a.caldav.Pull(a.ctx, id)
}

// SyncCalDAV runs the two-way sync for an account: it pushes the account's pending local calendar changes to
// the server, then reconciles the server's calendars back into the local store, resolving a conflict in the
// server's favour while preserving the losing local edit as a safety copy.
func (a *App) SyncCalDAV(id string) error {
	return a.caldav.Sync(a.ctx, id)
}
