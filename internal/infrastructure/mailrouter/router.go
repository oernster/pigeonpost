// Package mailrouter dispatches mail read and verification operations to the adapter for an account's
// protocol (IMAP or POP3), so the application sees a single MailSource and AccountVerifier regardless
// of which protocol an account uses. It is a thin composition-time adapter and holds no protocol logic
// of its own beyond the per-account selection.
package mailrouter

import (
	"context"

	"github.com/oernster/pigeonpost/internal/domain"
)

// protocolSource is the read and verify surface each protocol adapter provides. Both imap.Source and
// pop3.Source satisfy it structurally.
type protocolSource interface {
	FetchFolders(ctx context.Context, account domain.Account) ([]domain.Folder, error)
	FetchMessages(ctx context.Context, account domain.Account, folder domain.Folder) ([]domain.MessageSummary, error)
	FetchBody(ctx context.Context, account domain.Account, folder domain.Folder, uid string) (string, string, error)
	FetchRaw(ctx context.Context, account domain.Account, folder domain.Folder, uid string) ([]byte, error)
	Verify(ctx context.Context, account domain.Account, password string) error
}

// Router selects the protocol adapter for each account and satisfies application.MailSource and
// application.AccountVerifier by delegating to it.
type Router struct {
	imap protocolSource
	pop3 protocolSource
}

// NewRouter constructs the router from the IMAP and POP3 adapters.
func NewRouter(imapSource, pop3Source protocolSource) *Router {
	return &Router{imap: imapSource, pop3: pop3Source}
}

// sourceFor returns the adapter for the account's protocol, defaulting to IMAP for any non-POP3
// protocol.
func (r *Router) sourceFor(account domain.Account) protocolSource {
	if account.Protocol() == domain.ProtocolPOP3 {
		return r.pop3
	}
	return r.imap
}

// FetchFolders delegates to the account's protocol adapter.
func (r *Router) FetchFolders(ctx context.Context, account domain.Account) ([]domain.Folder, error) {
	return r.sourceFor(account).FetchFolders(ctx, account)
}

// FetchMessages delegates to the account's protocol adapter.
func (r *Router) FetchMessages(ctx context.Context, account domain.Account, folder domain.Folder) ([]domain.MessageSummary, error) {
	return r.sourceFor(account).FetchMessages(ctx, account, folder)
}

// FetchBody delegates to the account's protocol adapter.
func (r *Router) FetchBody(ctx context.Context, account domain.Account, folder domain.Folder, uid string) (string, string, error) {
	return r.sourceFor(account).FetchBody(ctx, account, folder, uid)
}

// FetchRaw delegates to the account's protocol adapter.
func (r *Router) FetchRaw(ctx context.Context, account domain.Account, folder domain.Folder, uid string) ([]byte, error) {
	return r.sourceFor(account).FetchRaw(ctx, account, folder, uid)
}

// Verify delegates to the account's protocol adapter.
func (r *Router) Verify(ctx context.Context, account domain.Account, password string) error {
	return r.sourceFor(account).Verify(ctx, account, password)
}
