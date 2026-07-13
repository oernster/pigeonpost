// Package keychain stores and retrieves account secrets in the operating system's credential store
// (Windows Credential Manager, macOS Keychain, Secret Service on Linux) via zalando/go-keyring.
// Secrets never touch the local database.
package keychain

import (
	"context"
	"fmt"

	"github.com/zalando/go-keyring"

	"github.com/oernster/pigeonpost/internal/domain"
)

// serviceName is the keychain service under which all PigeonPost secrets are grouped.
const serviceName = "PigeonPost"

// Vault reads and writes account passwords in the OS keychain. It satisfies imap.PasswordProvider.
type Vault struct {
	service string
}

// NewVault constructs a vault using the default service name.
func NewVault() *Vault {
	return &Vault{service: serviceName}
}

// Password returns the stored secret for an account.
func (v *Vault) Password(_ context.Context, account domain.Account) (string, error) {
	secret, err := keyring.Get(v.service, account.ID())
	if err != nil {
		return "", fmt.Errorf("keychain: get password for %q: %w", account.ID(), err)
	}
	return secret, nil
}

// SetPassword stores (or replaces) the secret for an account. The context is part of the
// application.CredentialStore contract; the underlying keyring call is synchronous and does not use it.
func (v *Vault) SetPassword(_ context.Context, account domain.Account, secret string) error {
	if err := keyring.Set(v.service, account.ID(), secret); err != nil {
		return fmt.Errorf("keychain: set password for %q: %w", account.ID(), err)
	}
	return nil
}

// DeletePassword removes the stored secret for an account.
func (v *Vault) DeletePassword(_ context.Context, account domain.Account) error {
	if err := keyring.Delete(v.service, account.ID()); err != nil {
		return fmt.Errorf("keychain: delete password for %q: %w", account.ID(), err)
	}
	return nil
}

// caldavKeyPrefix namespaces a CalDAV/CardDAV account's key so it cannot collide with a mail account id
// under the shared keychain service. PurgeAll still removes it because the service name is the same.
const caldavKeyPrefix = "caldav:"

// CalendarPassword returns the stored secret for a CalDAV/CardDAV account.
func (v *Vault) CalendarPassword(_ context.Context, account domain.CalendarAccount) (string, error) {
	secret, err := keyring.Get(v.service, caldavKeyPrefix+account.ID())
	if err != nil {
		return "", fmt.Errorf("keychain: get calendar password for %q: %w", account.ID(), err)
	}
	return secret, nil
}

// SetCalendarPassword stores (or replaces) the secret for a CalDAV/CardDAV account.
func (v *Vault) SetCalendarPassword(_ context.Context, account domain.CalendarAccount, secret string) error {
	if err := keyring.Set(v.service, caldavKeyPrefix+account.ID(), secret); err != nil {
		return fmt.Errorf("keychain: set calendar password for %q: %w", account.ID(), err)
	}
	return nil
}

// DeleteCalendarPassword removes the stored secret for a CalDAV/CardDAV account.
func (v *Vault) DeleteCalendarPassword(_ context.Context, account domain.CalendarAccount) error {
	if err := keyring.Delete(v.service, caldavKeyPrefix+account.ID()); err != nil {
		return fmt.Errorf("keychain: delete calendar password for %q: %w", account.ID(), err)
	}
	return nil
}

// PurgeAll removes every stored PigeonPost secret from the OS keychain in a single call, without
// needing the individual account IDs. The uninstaller uses it so that choosing to delete user data
// leaves no saved passwords behind in the credential store.
func (v *Vault) PurgeAll() error {
	if err := keyring.DeleteAll(v.service); err != nil {
		return fmt.Errorf("keychain: purge all secrets: %w", err)
	}
	return nil
}
