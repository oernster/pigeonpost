package imap

import (
	"context"
	"fmt"
	"time"

	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/oernster/pigeonpost/internal/domain"
)

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
}

// NewWatcher constructs the watcher with the password provider used to authenticate each IDLE connection.
func NewWatcher(passwords PasswordProvider) *Watcher {
	return &Watcher{passwords: passwords}
}

// Watch holds an IDLE connection to the account's inbox until ctx is cancelled, calling onChange whenever
// the server reports the mailbox has changed (a message arrived), and once at the start of each session so
// anything that arrived while reconnecting is caught. It reconnects with capped exponential backoff after
// any error and reissues the IDLE before the server would time it out. It is only for IMAP accounts; POP3
// has no IDLE and stays on the caller's poll.
func (w *Watcher) Watch(ctx context.Context, account domain.Account, onChange func()) {
	backoff := initialBackoff
	for ctx.Err() == nil {
		err := w.session(ctx, account, onChange)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			backoff = initialBackoff
			continue
		}
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
// mailbox change, on the refresh timer or on cancellation. It returns nil only when ctx is cancelled, and
// an error otherwise so Watch reconnects.
func (w *Watcher) session(ctx context.Context, account domain.Account, onChange func()) error {
	password, err := w.passwords.Password(ctx, account)
	if err != nil {
		return fmt.Errorf("imap idle: password for %q: %w", account.ID(), err)
	}
	changed := make(chan struct{}, 1)
	signal := func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	}
	options := &imapclient.Options{
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Mailbox: func(data *imapclient.UnilateralDataMailbox) {
				if data.NumMessages != nil {
					signal()
				}
			},
		},
	}
	client, err := dial(account.Incoming(), options)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Login(account.Address().Address(), password).Wait(); err != nil {
		return fmt.Errorf("imap idle: login %q: %w", account.ID(), err)
	}
	if _, err := client.Select("INBOX", nil).Wait(); err != nil {
		return fmt.Errorf("imap idle: select inbox: %w", err)
	}
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
