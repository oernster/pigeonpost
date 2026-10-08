package imap

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/oernster/pigeonpost/internal/domain"
)

// parkedIdleLimit is how long a parked connection waits for its next operation before it is logged out.
// RFC 3501 has a server keep an idle connection for at least 30 minutes, so a connection parked for less
// than that is still open when it is taken; five minutes covers a reader working through a folder,
// opening and marking message after message, without holding a connection open all day.
const parkedIdleLimit = 5 * time.Minute

// parkedCheckLimit bounds the NOOP that proves a parked connection still answers; it bounds the LOGOUT
// that retires one too. A connection left half-open by a sleep or a network change takes a command and never
// replies, so without a bound the user's click would wait forever. It is the time a fresh connection is
// allowed to take: a parked one that cannot answer within it is worth less than dialling again.
const parkedCheckLimit = dialTimeout

// parkKey says which connections are interchangeable: the same account signed in to the same server as
// the same user by the same method. An account edited to point at another server never reuses a
// connection to the old one.
type parkKey struct {
	accountID string
	incoming  domain.ServerConfig
	user      string
	auth      domain.AuthMethod
}

func parkKeyFor(account domain.Account) parkKey {
	return parkKey{accountID: account.ID(), incoming: account.Incoming(), user: account.Address().Address(), auth: account.Auth()}
}

// parked is one idle connection and the timer that retires it at the idle limit.
type parked struct {
	client *imapclient.Client
	expiry *time.Timer
}

// parking keeps at most one idle, signed-in connection per account between operations, so a user's
// actions (opening a message, marking it, moving it) share a login rather than each making its own.
// Before that every action logged in and out again; heavy clicking reached 13 logins a minute against
// StartMail on 2026-10-07, close to the rate that had the address blocked. A second operation running
// while the first holds the connection dials its own; when both finish, one is parked and the other
// logged out, so the idle count never grows past one.
//
// Every connection connect hands out is lent: the lot remembers whose it is, so release can park it
// without each operation passing its account back.
type parking struct {
	mu         sync.Mutex
	slots      map[parkKey]*parked
	lent       map[*imapclient.Client]parkKey
	idleLimit  time.Duration
	checkLimit time.Duration
}

func newParking() *parking {
	return &parking{
		slots:      map[parkKey]*parked{},
		lent:       map[*imapclient.Client]parkKey{},
		idleLimit:  parkedIdleLimit,
		checkLimit: parkedCheckLimit,
	}
}

// take answers the account's parked connection, lent out, once it has answered a NOOP; nil when none is
// parked or the parked one no longer answers, in which case it is closed and the caller dials afresh.
func (p *parking) take(account domain.Account) *imapclient.Client {
	key := parkKeyFor(account)
	p.mu.Lock()
	slot, ok := p.slots[key]
	if ok {
		delete(p.slots, key)
		slot.expiry.Stop()
	}
	p.mu.Unlock()
	if !ok {
		return nil
	}
	if err := within(p.checkLimit, func() error { return slot.client.Noop().Wait() }); err != nil {
		log.Printf("imap: parked connection for %s no longer answers, dialling afresh: %v", key.accountID, err)
		_ = slot.client.Close()
		return nil
	}
	p.lend(account, slot.client)
	return slot.client
}

// lend records that client, freshly dialled or taken, is in use for account.
func (p *parking) lend(account domain.Account, client *imapclient.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lent[client] = parkKeyFor(account)
}

// giveBack ends an operation's use of client. A lent connection is parked for the account's next
// operation unless one is already parked there, in which case it is retired; a connection the lot never
// lent is retired.
// One the client library has already closed (it does so on a reply it cannot decode) is dropped.
func (p *parking) giveBack(client *imapclient.Client) {
	p.mu.Lock()
	key, ok := p.lent[client]
	delete(p.lent, client)
	_, occupied := p.slots[key]
	if isClosed(client) {
		p.mu.Unlock()
		return
	}
	if !ok || occupied {
		p.mu.Unlock()
		p.retire(client)
		return
	}
	slot := &parked{client: client}
	slot.expiry = time.AfterFunc(p.idleLimit, func() { p.expire(key, slot) })
	p.slots[key] = slot
	p.mu.Unlock()
}

// expire retires slot when its idle limit passes, unless an operation has taken it first.
func (p *parking) expire(key parkKey, slot *parked) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("imap: retiring the parked connection for %s panicked: %v", key.accountID, r)
		}
	}()
	p.mu.Lock()
	current, still := p.slots[key]
	if still && current == slot {
		delete(p.slots, key)
	}
	p.mu.Unlock()
	if still && current == slot {
		p.retire(slot.client)
	}
}

// retire logs a connection out, closing it outright when the server does not answer the LOGOUT in time.
func (p *parking) retire(client *imapclient.Client) {
	if err := within(p.checkLimit, func() error { return client.Logout().Wait() }); err != nil {
		_ = client.Close()
	}
}

// isClosed reports whether the client's connection has already been closed.
func isClosed(client *imapclient.Client) bool {
	select {
	case <-client.Closed():
		return true
	default:
		return false
	}
}

// within runs command and answers its error; a timeout error when it has not finished inside limit.
// A command still running at the limit is abandoned to its own goroutine; the caller closes the
// connection, which ends it. A panic in the command is answered as an error rather than ending the run.
func within(limit time.Duration, command func() error) error {
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("imap: command panicked: %v", r)
			}
		}()
		done <- command()
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(limit):
		return fmt.Errorf("imap: no answer within %s", limit)
	}
}
