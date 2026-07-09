package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// OAuthCredential is the outcome of an interactive OAuth sign-in: the signed-in address, a live access
// token used to verify mailbox access immediately, and the opaque secret (a JSON token blob) to persist
// in the keychain for later silent refresh.
type OAuthCredential struct {
	Email       string
	AccessToken string
	Secret      string
}

// OAuthAuthorizer runs the interactive OAuth flow (browser consent plus token exchange) and returns the
// resulting credential. It is the seam that keeps the browser, the loopback redirect and the HTTP token
// exchange in infrastructure, out of this use case.
type OAuthAuthorizer interface {
	Authorize(ctx context.Context) (OAuthCredential, error)
}

// MicrosoftAccountBuilder builds a validated Microsoft OAuth account for a signed-in address. It is
// injected so this use case holds no provider server details: the concrete Microsoft endpoints live in
// the composition root alongside the other provider construction.
type MicrosoftAccountBuilder func(email, displayName string) (domain.Account, error)

// MicrosoftSetupService configures a Microsoft account through OAuth: it runs the sign-in, builds the
// account, verifies mailbox access with the fresh token, stores the token blob in the keychain and
// persists the account. It mirrors AccountSetupService's verify-then-store ordering, rolling the stored
// token back if the account fails to save.
type MicrosoftSetupService struct {
	accounts    AccountStore
	credentials CredentialStore
	verifier    AccountVerifier
	authorizer  OAuthAuthorizer
	build       MicrosoftAccountBuilder
}

// NewMicrosoftSetupService constructs the service with its injected store, credential store, verifier,
// authorizer and account builder.
func NewMicrosoftSetupService(
	accounts AccountStore,
	credentials CredentialStore,
	verifier AccountVerifier,
	authorizer OAuthAuthorizer,
	build MicrosoftAccountBuilder,
) *MicrosoftSetupService {
	return &MicrosoftSetupService{
		accounts:    accounts,
		credentials: credentials,
		verifier:    verifier,
		authorizer:  authorizer,
		build:       build,
	}
}

// Configure signs the user in, builds and verifies the account and persists it, returning the saved
// account so the caller can start its mail watcher. The signed-in address becomes the account id, so
// re-adding the same Microsoft account updates it in place. displayName may be empty, in which case the
// address is used.
func (s *MicrosoftSetupService) Configure(ctx context.Context, displayName string) (domain.Account, error) {
	cred, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return domain.Account{}, fmt.Errorf("microsoft sign-in: %w", err)
	}
	account, err := s.build(cred.Email, displayName)
	if err != nil {
		return domain.Account{}, fmt.Errorf("build microsoft account: %w", err)
	}
	if err := s.verifier.Verify(ctx, account, cred.AccessToken); err != nil {
		return domain.Account{}, fmt.Errorf("verify microsoft account %q: %w", account.ID(), err)
	}
	if err := s.credentials.SetPassword(ctx, account, cred.Secret); err != nil {
		return domain.Account{}, fmt.Errorf("store token for %q: %w", account.ID(), err)
	}
	if err := s.accounts.SaveAccount(ctx, account); err != nil {
		_ = s.credentials.DeletePassword(ctx, account)
		return domain.Account{}, fmt.Errorf("save microsoft account %q: %w", account.ID(), err)
	}
	return account, nil
}
