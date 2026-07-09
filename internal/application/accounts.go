package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// AccountService is the use-case boundary for reading, adding and removing accounts.
type AccountService struct {
	accounts    AccountStore
	credentials CredentialStore
	mail        MailStore
}

// NewAccountService constructs the service with its injected account store, credential store and mail
// cache. The latter two are used when removing an account so its secret and cached mail go with it.
func NewAccountService(accounts AccountStore, credentials CredentialStore, mail MailStore) *AccountService {
	return &AccountService{accounts: accounts, credentials: credentials, mail: mail}
}

// List returns all configured accounts.
func (s *AccountService) List(ctx context.Context) ([]domain.Account, error) {
	accounts, err := s.accounts.ListAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	return accounts, nil
}

// Add persists a new or updated account.
func (s *AccountService) Add(ctx context.Context, account domain.Account) error {
	if err := s.accounts.SaveAccount(ctx, account); err != nil {
		return fmt.Errorf("add account %q: %w", account.ID(), err)
	}
	return nil
}

// Get returns a single account by id.
func (s *AccountService) Get(ctx context.Context, id string) (domain.Account, error) {
	account, err := s.accounts.GetAccount(ctx, id)
	if err != nil {
		return domain.Account{}, fmt.Errorf("get account %q: %w", id, err)
	}
	return account, nil
}

// UpdateProfile changes an account's editable profile fields (display name, signature and alternate
// sender identities) and nothing else: it leaves the servers, protocol, auth method and stored
// credential untouched. That makes it the correct edit path for an OAuth account, whose token must not
// be re-verified with a password; it is safe for any account because it changes no server setting.
// The account is loaded, the changes are applied to the immutable value and it is saved back.
func (s *AccountService) UpdateProfile(
	ctx context.Context,
	id string,
	displayName string,
	signature string,
	identities []domain.EmailAddress,
) error {
	account, err := s.accounts.GetAccount(ctx, id)
	if err != nil {
		return fmt.Errorf("update account %q: %w", id, err)
	}
	renamed, err := account.WithDisplayName(displayName)
	if err != nil {
		return fmt.Errorf("update account %q: %w", id, err)
	}
	updated := renamed.WithSignature(signature).WithIdentities(identities)
	if err := s.accounts.SaveAccount(ctx, updated); err != nil {
		return fmt.Errorf("update account %q: %w", id, err)
	}
	return nil
}

// Reorder sets the accounts' sidebar order from the given full list of ids, so the account at index i
// takes position i. The caller passes the complete ordered id list (not a single move), which keeps the
// stored positions distinct and free of collisions.
func (s *AccountService) Reorder(ctx context.Context, orderedIDs []string) error {
	if err := s.accounts.SetAccountPositions(ctx, orderedIDs); err != nil {
		return fmt.Errorf("reorder accounts: %w", err)
	}
	return nil
}

// Remove deletes an account together with its cached mail and its keychain secret. The account row is
// removed first so it disappears from the UI immediately; its cached folders/messages and its stored
// password are then cleaned up.
func (s *AccountService) Remove(ctx context.Context, id string) error {
	account, err := s.accounts.GetAccount(ctx, id)
	if err != nil {
		return fmt.Errorf("remove account %q: %w", id, err)
	}
	if err := s.accounts.DeleteAccount(ctx, id); err != nil {
		return fmt.Errorf("remove account %q: %w", id, err)
	}
	if err := s.mail.DeleteAccountData(ctx, id); err != nil {
		return fmt.Errorf("remove account %q cached mail: %w", id, err)
	}
	if err := s.credentials.DeletePassword(ctx, account); err != nil {
		return fmt.Errorf("remove account %q credential: %w", id, err)
	}
	return nil
}

// AccountSetupService is the use-case boundary for configuring a new account from the setup wizard.
// It stores the credential, verifies it against the incoming server and only then persists the
// account, rolling the credential back if verification or persistence fails so a failed setup leaves
// no orphaned secret behind.
type AccountSetupService struct {
	accounts    AccountStore
	credentials CredentialStore
	verifier    AccountVerifier
}

// NewAccountSetupService constructs the service with its injected store, credential store and verifier.
func NewAccountSetupService(
	accounts AccountStore,
	credentials CredentialStore,
	verifier AccountVerifier,
) *AccountSetupService {
	return &AccountSetupService{accounts: accounts, credentials: credentials, verifier: verifier}
}

// Configure verifies the secret against the incoming server, then stores it and persists the account.
// Verification runs first so nothing is written to the keychain or the store until the credentials are
// known good. If persistence fails after the secret is stored, the secret is removed again.
func (s *AccountSetupService) Configure(ctx context.Context, account domain.Account, secret string) error {
	if err := s.verifier.Verify(ctx, account, secret); err != nil {
		return fmt.Errorf("verify account %q: %w", account.ID(), err)
	}
	if err := s.credentials.SetPassword(ctx, account, secret); err != nil {
		return fmt.Errorf("store credential for %q: %w", account.ID(), err)
	}
	if err := s.accounts.SaveAccount(ctx, account); err != nil {
		_ = s.credentials.DeletePassword(ctx, account)
		return fmt.Errorf("save account %q: %w", account.ID(), err)
	}
	return nil
}

// Update re-configures an existing account. A non-empty newSecret replaces the stored password; an
// empty newSecret keeps the current one (read from the keychain to re-verify the possibly-changed
// server settings). Verification always runs first, so a failed update never disturbs the working
// account's stored password.
func (s *AccountSetupService) Update(ctx context.Context, account domain.Account, newSecret string) error {
	secret := newSecret
	if newSecret == "" {
		existing, err := s.credentials.Password(ctx, account)
		if err != nil {
			return fmt.Errorf("read existing credential for %q: %w", account.ID(), err)
		}
		secret = existing
	}
	if err := s.verifier.Verify(ctx, account, secret); err != nil {
		return fmt.Errorf("verify account %q: %w", account.ID(), err)
	}
	if newSecret != "" {
		if err := s.credentials.SetPassword(ctx, account, newSecret); err != nil {
			return fmt.Errorf("update credential for %q: %w", account.ID(), err)
		}
	}
	if err := s.accounts.SaveAccount(ctx, account); err != nil {
		return fmt.Errorf("save account %q: %w", account.ID(), err)
	}
	return nil
}
