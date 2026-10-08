package mailrouter

import (
	"context"
	"errors"

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

// listingSource is an adapter that can list a folder's UIDs and flags without headers and fetch chosen
// summaries by UID: what the incremental sync needs. IMAP is one; POP3 has no UIDVALIDITY and is not.
type listingSource interface {
	FetchListing(ctx context.Context, account domain.Account, folder domain.Folder) ([]domain.MessageState, uint32, error)
	FetchMessagesByUID(ctx context.Context, account domain.Account, folder domain.Folder, uids []string) ([]domain.MessageSummary, uint32, error)
}

// errNoListing answers a by-UID fetch on an adapter with no listing. The sync never asks for one, since
// such an adapter reports no UIDVALIDITY and the sync then fetches in full.
var errNoListing = errors.New("mailrouter: this protocol cannot fetch messages by UID")

// FetchListing delegates to the account's protocol adapter where it can list; otherwise it reports no
// UIDVALIDITY, so the sync fetches the folder in full. It is the application.MailSource listing.
func (r *Router) FetchListing(ctx context.Context, account domain.Account, folder domain.Folder) ([]domain.MessageState, uint32, error) {
	if lister, ok := r.sourceFor(account).(listingSource); ok {
		return lister.FetchListing(ctx, account, folder)
	}
	return nil, noUIDValidity, nil
}

// FetchMessagesByUID delegates to the account's protocol adapter where it can list. It is the
// application.MailSource by-UID fetch.
func (r *Router) FetchMessagesByUID(ctx context.Context, account domain.Account, folder domain.Folder, uids []string) ([]domain.MessageSummary, uint32, error) {
	if lister, ok := r.sourceFor(account).(listingSource); ok {
		return lister.FetchMessagesByUID(ctx, account, folder, uids)
	}
	return nil, noUIDValidity, errNoListing
}
