package keychain

import (
	"context"
	"errors"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/oernster/pigeonpost/internal/domain"
)

func testAccount(t *testing.T) domain.Account {
	t.Helper()
	addr, err := domain.NewEmailAddress("", "user@example.com")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	sc, err := domain.NewServerConfig("host.example.com", 993, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("server config: %v", err)
	}
	account, err := domain.NewAccount("acc-1", "Test", addr, domain.ProtocolIMAP, sc, sc, domain.AuthPassword)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	return account
}

func TestVaultRoundTrip(t *testing.T) {
	keyring.MockInit()
	vault := NewVault()
	account := testAccount(t)

	if err := vault.SetPassword(context.Background(), account, "s3cret"); err != nil {
		t.Fatalf("set: %v", err)
	}
	secret, err := vault.Password(context.Background(), account)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if secret != "s3cret" {
		t.Errorf("secret = %q, want s3cret", secret)
	}

	if err := vault.DeletePassword(context.Background(), account); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := vault.Password(context.Background(), account); err == nil {
		t.Error("expected error reading a deleted secret")
	}
}

func TestVaultPurgeAll(t *testing.T) {
	keyring.MockInit()
	vault := NewVault()
	account := testAccount(t)

	if err := vault.SetPassword(context.Background(), account, "s3cret"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := keyring.Set(serviceName, "acc-2", "another"); err != nil {
		t.Fatalf("set second: %v", err)
	}

	if err := vault.PurgeAll(); err != nil {
		t.Fatalf("purge: %v", err)
	}

	if _, err := vault.Password(context.Background(), account); err == nil {
		t.Error("expected error reading a purged secret")
	}
	if _, err := keyring.Get(serviceName, "acc-2"); err == nil {
		t.Error("expected the second secret to be purged too")
	}
}

func TestVaultWrapsErrors(t *testing.T) {
	boom := errors.New("keychain unavailable")
	keyring.MockInitWithError(boom)
	vault := NewVault()
	account := testAccount(t)

	if err := vault.SetPassword(context.Background(), account, "x"); !errors.Is(err, boom) {
		t.Errorf("SetPassword error = %v, want wrapped boom", err)
	}
	if _, err := vault.Password(context.Background(), account); !errors.Is(err, boom) {
		t.Errorf("Password error = %v, want wrapped boom", err)
	}
	if err := vault.DeletePassword(context.Background(), account); !errors.Is(err, boom) {
		t.Errorf("DeletePassword error = %v, want wrapped boom", err)
	}
	if err := vault.PurgeAll(); !errors.Is(err, boom) {
		t.Errorf("PurgeAll error = %v, want wrapped boom", err)
	}
}
