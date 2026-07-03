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
