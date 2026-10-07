package imap

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// watchWindow is how long a test lets the watcher run before stopping it. It is long enough for a
// watcher caught in a loop to go round it many times; a correct one goes round none.
const watchWindow = 2 * time.Second

// watchFor runs the real watcher against a scripted server for watchWindow and answers how many times it
// asked for a mail check and how many times it logged in, which is one per connection it opened.
func watchFor(t *testing.T, s script) (checks int32, logins int) {
	t.Helper()
	commands := &commandLog{}
	s.commands = commands
	s.extraCaps = "IDLE"
	host, port := listenFake(t, s)
	account := fakeAccount(t, host, port)
	watcher := NewWatcher(staticPassword{secret: "secret"}, staticToken{token: "token"})

	var calls atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), watchWindow)
	defer cancel()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		watcher.Watch(ctx, account, func() { calls.Add(1) })
	}()
	<-finished
	return calls.Load(), len(commands.matching("LOGIN"))
}

// A server may report the mailbox size on every IDLE, changed or not. The watcher must not read an
// unchanged size as new mail: each check it asks for is a full sync of every account, so a check per
// IDLE becomes a sync every round trip and the server sees a connection every few seconds. StartMail
// blocked the user's address for exactly that rate on 2026-10-07.
func TestWatchIgnoresAnUnchangedMailboxSize(t *testing.T) {
	t.Parallel()
	checks, logins := watchFor(t, script{idleUpdate: "* 1 EXISTS"})
	t.Logf("unchanged size on every IDLE: %d mail checks, %d logins in %s", checks, logins, watchWindow)
	// The one check is the catch-up the watcher asks for when its session starts.
	if checks != 1 {
		t.Errorf("mail checks = %d, want 1: an unchanged size was read as new mail", checks)
	}
	if logins != 1 {
		t.Errorf("logins = %d, want 1: the watcher reconnected", logins)
	}
}

// The size follows what the server reports: a repeat is not an arrival; an expunge lowers the baseline so
// a message arriving after a deletion is one; the size never falls below zero.
func TestMailboxSizeReadsOnlyGrowthAsArrival(t *testing.T) {
	t.Parallel()
	var size mailboxSize
	size.set(3)
	if size.grew(3) {
		t.Error("a repeated size read as an arrival")
	}
	size.shrink()
	if !size.grew(3) {
		t.Error("an arrival after a deletion was missed")
	}
	if size.grew(3) {
		t.Error("the size repeated after an arrival read as another")
	}
	var empty mailboxSize
	empty.shrink()
	if !empty.grew(1) {
		t.Error("an expunge on an empty inbox took the size below zero")
	}
}

// A size larger than the one last seen is new mail, so it must still ask for a check, once.
func TestWatchChecksWhenTheMailboxGrows(t *testing.T) {
	t.Parallel()
	checks, logins := watchFor(t, script{idleUpdate: "* 2 EXISTS"})
	t.Logf("grown size on every IDLE: %d mail checks, %d logins in %s", checks, logins, watchWindow)
	// The catch-up at session start, then the arrival; the same size repeated afterwards is not new.
	const wantChecks = 2
	if checks != wantChecks {
		t.Errorf("mail checks = %d, want %d", checks, wantChecks)
	}
	if logins != 1 {
		t.Errorf("logins = %d, want 1: the watcher reconnected", logins)
	}
}
