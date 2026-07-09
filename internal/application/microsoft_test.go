package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// msCredential is a representative OAuth credential returned by the fake authorizer.
var msCredential = OAuthCredential{Email: "user@example.com", AccessToken: "access-tok", Secret: "{token-blob}"}

// okBuilder is a MicrosoftAccountBuilder that returns a fixed test account, ignoring its inputs. It stands
// in for the real Microsoft account construction, which lives in the composition root.
func okBuilder(t *testing.T) MicrosoftAccountBuilder {
	return func(email, displayName string) (domain.Account, error) {
		return testAccount(t, email), nil
	}
}

func newMicrosoftService(t *testing.T, build MicrosoftAccountBuilder) (
	*MicrosoftSetupService, *fakeAccountStore, *fakeCredentialStore, *fakeVerifier, *fakeAuthorizer,
) {
	store := newFakeAccountStore()
	creds := newFakeCredentialStore()
	verifier := newFakeVerifier()
	authorizer := &fakeAuthorizer{cred: msCredential}
	return NewMicrosoftSetupService(store, creds, verifier, authorizer, build), store, creds, verifier, authorizer
}

func TestMicrosoftConfigureSuccess(t *testing.T) {
	svc, store, creds, verifier, authorizer := newMicrosoftService(t, okBuilder(t))

	account, err := svc.Configure(context.Background(), "Jane")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !authorizer.authorized {
		t.Error("expected the authorizer to have run")
	}
	if account.ID() != msCredential.Email {
		t.Errorf("account id = %q, want %q", account.ID(), msCredential.Email)
	}
	// Verification is proven with the live access token, not the stored blob.
	if verifier.verified[msCredential.Email] != msCredential.AccessToken {
		t.Errorf("verify saw %q, want the access token", verifier.verified[msCredential.Email])
	}
	// The keychain holds the opaque token blob, standing in for a password.
	if creds.passwords[msCredential.Email] != msCredential.Secret {
		t.Errorf("stored secret = %q, want the token blob", creds.passwords[msCredential.Email])
	}
	if len(store.saved) != 1 {
		t.Errorf("expected the account saved, got %d", len(store.saved))
	}
	if len(creds.deleted) != 0 {
		t.Errorf("expected no rollback on success, got %v", creds.deleted)
	}
}

func TestMicrosoftConfigureAuthorizeError(t *testing.T) {
	svc, store, creds, _, authorizer := newMicrosoftService(t, okBuilder(t))
	authorizer.err = errBoom

	if _, err := svc.Configure(context.Background(), "Jane"); !errors.Is(err, errBoom) {
		t.Errorf("Configure error = %v, want wrapped boom", err)
	}
	if len(store.saved) != 0 || len(creds.passwords) != 0 {
		t.Errorf("a failed sign-in must not touch the store or keychain: saved=%v pw=%v", store.saved, creds.passwords)
	}
}

func TestMicrosoftConfigureBuildError(t *testing.T) {
	build := func(string, string) (domain.Account, error) { return domain.Account{}, errBoom }
	svc, store, creds, _, _ := newMicrosoftService(t, build)

	if _, err := svc.Configure(context.Background(), "Jane"); !errors.Is(err, errBoom) {
		t.Errorf("Configure error = %v, want wrapped boom", err)
	}
	if len(store.saved) != 0 || len(creds.passwords) != 0 {
		t.Errorf("a build failure must not touch the store or keychain")
	}
}

func TestMicrosoftConfigureVerifyError(t *testing.T) {
	svc, store, creds, verifier, _ := newMicrosoftService(t, okBuilder(t))
	verifier.verifyErr = errBoom

	if _, err := svc.Configure(context.Background(), "Jane"); !errors.Is(err, errBoom) {
		t.Errorf("Configure error = %v, want wrapped boom", err)
	}
	// Verify runs before anything is written.
	if len(store.saved) != 0 || len(creds.passwords) != 0 {
		t.Errorf("a failed verify must not write the token or account: saved=%v pw=%v", store.saved, creds.passwords)
	}
}

func TestMicrosoftConfigureStoreTokenError(t *testing.T) {
	svc, store, creds, _, _ := newMicrosoftService(t, okBuilder(t))
	creds.setErr = errBoom

	if _, err := svc.Configure(context.Background(), "Jane"); !errors.Is(err, errBoom) {
		t.Errorf("Configure error = %v, want wrapped boom", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("the account must not be saved when the token store fails")
	}
}

func TestMicrosoftConfigureSaveErrorRollsBack(t *testing.T) {
	svc, store, creds, _, _ := newMicrosoftService(t, okBuilder(t))
	store.saveErr = errBoom

	if _, err := svc.Configure(context.Background(), "Jane"); !errors.Is(err, errBoom) {
		t.Errorf("Configure error = %v, want wrapped boom", err)
	}
	if len(creds.deleted) != 1 || creds.deleted[0] != msCredential.Email {
		t.Errorf("expected the stored token rolled back, got %v", creds.deleted)
	}
}
