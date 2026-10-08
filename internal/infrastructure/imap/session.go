package imap

import (
	"context"

	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/oernster/pigeonpost/internal/domain"
)

// sessionKey carries a sync session on a context.
type sessionKey struct{}

// session is one logged-in connection shared by every operation run under its context for one account.
// It is opened by the first operation that needs it and logged out when the session ends.
type session struct {
	accountID string
	client    *imapclient.Client
}

// BeginSession answers a context under which every operation on account reuses one connection, plus the
// function that ends the session and logs that connection out. A full account sync runs under one: it
// lists the folders, replays pending flags and fetches every folder. Each of those used to log in on its
// own, so a sync of StartMail's 49 folders made about 51 logins in a minute. A session serves its
// operations one after another, never at once. An operation for another account uses a connection of
// its own; so does one run under a context without a session (a user's action, the IDLE watcher). When
// the session ends its connection is parked rather than logged out, so the user's next action reuses it.
// It satisfies application.MailSource.
func (s *Source) BeginSession(ctx context.Context, account domain.Account) (context.Context, func()) {
	shared := &session{accountID: account.ID()}
	end := func() {
		if shared.client != nil {
			s.parking.giveBack(shared.client)
			shared.client = nil
		}
	}
	return context.WithValue(ctx, sessionKey{}, shared), end
}

// sessionFor answers the session ctx carries for account; nil when it carries none for that account.
func sessionFor(ctx context.Context, account domain.Account) *session {
	shared, ok := ctx.Value(sessionKey{}).(*session)
	if !ok || shared.accountID != account.ID() {
		return nil
	}
	return shared
}

// release ends an operation's use of client: it leaves the session's shared one open for the session's
// next operation and gives any other back to the lot, which parks it for the account's next operation.
func (s *Source) release(ctx context.Context, client *imapclient.Client) {
	if shared, ok := ctx.Value(sessionKey{}).(*session); ok && shared.client == client {
		return
	}
	s.parking.giveBack(client)
}
