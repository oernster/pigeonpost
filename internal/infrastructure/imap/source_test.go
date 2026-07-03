package imap

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// staticPassword is a PasswordProvider returning a fixed secret, used by the integration test.
type staticPassword struct{ secret string }

func (s staticPassword) Password(context.Context, domain.Account) (string, error) {
	return s.secret, nil
}

// TestSourceLive exercises the adapter against a real IMAP server. It is skipped unless the
// connection environment variables are set, so the normal `go test ./...` run stays offline.
//
// Set: PIGEONPOST_IMAP_HOST, PIGEONPOST_IMAP_PORT, PIGEONPOST_IMAP_EMAIL, PIGEONPOST_IMAP_PASSWORD.
func TestSourceLive(t *testing.T) {
	host := os.Getenv("PIGEONPOST_IMAP_HOST")
	portStr := os.Getenv("PIGEONPOST_IMAP_PORT")
	email := os.Getenv("PIGEONPOST_IMAP_EMAIL")
	password := os.Getenv("PIGEONPOST_IMAP_PASSWORD")
	if host == "" || portStr == "" || email == "" || password == "" {
		t.Skip("live IMAP env not set; skipping")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid PIGEONPOST_IMAP_PORT: %v", err)
	}

	addr, err := domain.NewEmailAddress("", email)
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	server, err := domain.NewServerConfig(host, port, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("server config: %v", err)
	}
	account, err := domain.NewAccount("live", "Live", addr, domain.ProtocolIMAP, server, server, domain.AuthPassword)
	if err != nil {
		t.Fatalf("account: %v", err)
	}

	source := NewSource(staticPassword{secret: password})
	if err := source.Verify(context.Background(), account, password); err != nil {
		t.Fatalf("verify: %v", err)
	}
	folders, err := source.FetchFolders(context.Background(), account)
	if err != nil {
		t.Fatalf("fetch folders: %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected at least one folder")
	}
	t.Logf("fetched %d folders; first = %q", len(folders), folders[0].Path())

	messages, err := source.FetchMessages(context.Background(), account, folders[0])
	if err != nil {
		t.Fatalf("fetch messages: %v", err)
	}
	t.Logf("fetched %d messages from %q", len(messages), folders[0].Path())
}
