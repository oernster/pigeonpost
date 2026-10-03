package mailrouter

import (
	"context"

	"github.com/oernster/pigeonpost/internal/domain"
)

// validitySource is an adapter that can report a folder's UIDVALIDITY with its messages. IMAP has one;
// POP3 has no such notion and does not implement it.
type validitySource interface {
	FetchMessagesValidity(ctx context.Context, account domain.Account, folder domain.Folder) ([]domain.MessageSummary, uint32, error)
}

// noUIDValidity is what the router reports for an adapter with no UIDVALIDITY. RFC 3501 makes a real
// value non-zero, so the sync reads zero as "none reported" and keeps its earlier behaviour.
const noUIDValidity uint32 = 0

// FetchMessagesValidity delegates to the account's protocol adapter, asking for the folder's UIDVALIDITY
// where the adapter can report one and falling back to a plain fetch where it cannot. It is the
// application.MailSource fetch, which is how the sync tells a renumbered mailbox from new mail.
func (r *Router) FetchMessagesValidity(ctx context.Context, account domain.Account, folder domain.Folder) ([]domain.MessageSummary, uint32, error) {
	source := r.sourceFor(account)
	if reporter, ok := source.(validitySource); ok {
		return reporter.FetchMessagesValidity(ctx, account, folder)
	}
	messages, err := source.FetchMessages(ctx, account, folder)
	return messages, noUIDValidity, err
}
