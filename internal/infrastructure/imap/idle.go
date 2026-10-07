package imap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/oernster/pigeonpost/internal/domain"
)

// errIdleUnsupported marks a server that does not advertise the IDLE capability, so the watcher stops for
// that account rather than reconnecting in a loop; the backstop poll then covers it.
var errIdleUnsupported = errors.New("imap idle: server does not support IDLE")

const (
	// idleRefresh is how often the IDLE is torn down and reissued. Servers drop an IDLE after roughly 30
	// minutes (RFC 2177), so it is refreshed comfortably inside that window to keep the connection live.
	idleRefresh = 20 * time.Minute
	// initialBackoff and maxBackoff bound the wait before reconnecting after the IDLE connection drops, so
	// a persistently failing account retries without hammering the server.
	initialBackoff = 5 * time.Second
	maxBackoff     = 5 * time.Minute
)

// Watcher maintains a persistent IMAP IDLE connection to an account's inbox so the server pushes new-mail
// notifications the instant they arrive, rather than the caller polling. It is separate from Source: a
// watcher holds one long-lived connection per account, where Source makes one-shot fetch connections.
type Watcher struct {
	passwords PasswordProvider
	tokens    TokenProvider
}

// NewWatcher constructs the watcher with the password and OAuth token providers used to authenticate each
// IDLE connection, one or the other depending on the account's auth method.
func NewWatcher(passwords PasswordProvider, tokens TokenProvider) *Watcher {
	return &Watcher{passwords: passwords, tokens: tokens}
}

// secret returns the credential to authenticate the IDLE connection with: a refreshed OAuth access token
// for an OAuth account, otherwise the stored keychain password.
func (w *Watcher) secret(ctx context.Context, account domain.Account) (string, error) {
	return credentialFor(ctx, w.passwords, w.tokens, account, "imap idle")
}

// Watch holds an IDLE connection to the account's inbox until ctx is cancelled. It calls onChange whenever
// the server reports the inbox has grown (a message arrived; see mailboxSize) plus once at the start of each
// session, so anything that arrived while reconnecting is caught. It reconnects with capped exponential backoff after
// any error and reissues the IDLE before the server would time it out. It is only for IMAP accounts; POP3
// has no IDLE and stays on the caller's poll.
func (w *Watcher) Watch(ctx context.Context, account domain.Account, onChange func()) {
	backoff := initialBackoff
	for ctx.Err() == nil {
		err := w.session(ctx, account, onChange)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errIdleUnsupported) {
			log.Printf("imap idle: %s does not support IDLE, leaving it to the poll", account.ID())
			return
		}
		if err == nil {
			backoff = initialBackoff
			continue
		}
		log.Printf("imap idle: %s session ended: %v (retry in %s)", account.ID(), err, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff *= 2; backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// session runs one IDLE connection: it logs in, selects the inbox and loops issuing IDLE, waking on a
// mailbox change, on the refresh timer or on cancellation. It returns nil only when ctx is cancelled; any
// other ending is an error, so Watch reconnects.
func (w *Watcher) session(ctx context.Context, account domain.Account, onChange func()) error {
	secret, err := w.secret(ctx, account)
	if err != nil {
		return err
	}
	changed := make(chan struct{}, 1)
	signal := func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	}
	var size mailboxSize
	options := &imapclient.Options{
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Mailbox: func(data *imapclient.UnilateralDataMailbox) {
				if data.NumMessages != nil && size.grew(*data.NumMessages) {
					signal()
				}
			},
			Expunge: func(uint32) { size.shrink() },
		},
	}
	client, err := dial(account.Incoming(), options)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := authenticate(client, account, secret); err != nil {
		return err
	}
	if !client.Caps().Has(imap.CapIdle) {
		return errIdleUnsupported
	}
	selected, err := client.Select("INBOX", nil).Wait()
	if err != nil {
		return fmt.Errorf("imap idle: select inbox: %w", err)
	}
	size.set(selected.NumMessages)
	log.Printf("imap idle: %s watching INBOX for new mail", account.ID())
	// Catch anything that arrived while the connection was down.
	onChange()
	return w.idleLoop(ctx, client, changed, onChange)
}

// idleLoop issues IDLE repeatedly on the selected inbox, calling onChange on each mailbox change and
// reissuing on the refresh timer, until ctx is cancelled or the connection errors.
func (w *Watcher) idleLoop(ctx context.Context, client *imapclient.Client, changed <-chan struct{}, onChange func()) error {
	refresh := time.NewTicker(idleRefresh)
	defer refresh.Stop()
	for {
		idle, err := client.Idle()
		if err != nil {
			return fmt.Errorf("imap idle: start: %w", err)
		}
		select {
		case <-ctx.Done():
			_ = idle.Close()
			_ = idle.Wait()
			return nil
		case <-changed:
			if err := stopIdle(idle); err != nil {
				return err
			}
			onChange()
		case <-refresh.C:
			if err := stopIdle(idle); err != nil {
				return err
			}
		}
	}
}

// mailboxSize is the inbox size the server last reported on the IDLE connection. A server may send EXISTS
// on every IDLE whether or not anything arrived (RFC 9051 lets it report the size at any time). Every
// change the watcher signals costs a full sync of every account. Reading each EXISTS as new mail therefore
// syncs once per round trip, which StartMail saw as a connection every few seconds and blocked the address
// for on 2026-10-07. Only a size above the last one is an arrival; an EXPUNGE lowers it, so mail arriving
// after a deletion still counts. The client's reader goroutine updates it while the watcher reads it, so it
// is atomic.
type mailboxSize struct{ last atomic.Uint32 }

// set records the size the SELECT reported, the baseline later reports are measured against.
func (m *mailboxSize) set(n uint32) { m.last.Store(n) }

// grew records a reported size and answers whether it is larger than the last one, which is new mail.
func (m *mailboxSize) grew(n uint32) bool { return m.last.Swap(n) < n }

// shrink records one expunged message, never taking the size below zero.
func (m *mailboxSize) shrink() {
	for {
		n := m.last.Load()
		if n == 0 || m.last.CompareAndSwap(n, n-1) {
			return
		}
	}
}

// stopIdle ends the current IDLE (sends DONE) and waits for the server to acknowledge, so the next command
// runs on a settled connection.
func stopIdle(idle *imapclient.IdleCommand) error {
	if err := idle.Close(); err != nil {
		return fmt.Errorf("imap idle: stop: %w", err)
	}
	if err := idle.Wait(); err != nil {
		return fmt.Errorf("imap idle: wait: %w", err)
	}
	return nil
}
