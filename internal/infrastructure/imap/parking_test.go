package imap

import (
	"context"
	"testing"
	"time"
)

// shortLimit stands in for the idle and check limits where a test needs one to pass while it watches.
const shortLimit = 50 * time.Millisecond

// waitLimit bounds how long a test waits for something the parking does on a timer.
const waitLimit = 2 * time.Second

// eventually reports whether cond holds within waitLimit, checking it every tenth of shortLimit.
func eventually(cond func() bool) bool {
	deadline := time.Now().Add(waitLimit)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(shortLimit / 10)
	}
	return cond()
}

// parkedCount answers how many connections the source has parked.
func parkedCount(source *Source) int {
	source.parking.mu.Lock()
	defer source.parking.mu.Unlock()
	return len(source.parking.slots)
}

// markSeen runs one user action without a session, the shape every click has.
func markSeen(t *testing.T, source *Source, host string, port int) {
	t.Helper()
	if err := source.SetSeen(context.Background(), fakeAccount(t, host, port), fakeFolder(t), "7", true); err != nil {
		t.Fatalf("SetSeen: %v", err)
	}
}

// A parked connection that answers its NOOP is reused: two actions, one login.
func TestParkedConnectionIsReusedOnceItAnswers(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands})
	source := fakeSource()
	markSeen(t, source, host, port)
	markSeen(t, source, host, port)
	if logins := len(commands.matching("LOGIN")); logins != 1 {
		t.Errorf("logins = %d, want 1", logins)
	}
	if noops := len(commands.matching("NOOP")); noops != 1 {
		t.Errorf("noops = %d, want 1: the parked connection is checked before reuse", noops)
	}
	if parkedCount(source) != 1 {
		t.Errorf("parked = %d, want 1 after the second action", parkedCount(source))
	}
}

// A parked connection the server has dropped is closed and replaced; the action still succeeds.
func TestDroppedParkedConnectionIsReplaced(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands})
	source := fakeSource()
	markSeen(t, source, host, port)
	_ = source.parking.slots[parkKeyFor(fakeAccount(t, host, port))].client.Close()
	markSeen(t, source, host, port)
	if logins := len(commands.matching("LOGIN")); logins != 2 {
		t.Errorf("logins = %d, want 2: the dropped connection is replaced", logins)
	}
}

// A half-open connection takes the NOOP and never answers; the check gives up at its limit rather than
// holding the user's action, which then succeeds on a fresh connection.
func TestSilentParkedConnectionIsReplacedAtTheCheckLimit(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands, silentNoop: true})
	source := fakeSource()
	source.parking.checkLimit = shortLimit
	markSeen(t, source, host, port)
	markSeen(t, source, host, port)
	if logins := len(commands.matching("LOGIN")); logins != 2 {
		t.Errorf("logins = %d, want 2: the silent connection is replaced", logins)
	}
}

// A parked connection nobody takes is logged out at the idle limit; the next action logs in again.
func TestParkedConnectionIsLoggedOutAtTheIdleLimit(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands})
	source := fakeSource()
	source.parking.idleLimit = shortLimit
	markSeen(t, source, host, port)
	if !eventually(func() bool { return len(commands.matching("LOGOUT")) == 1 && parkedCount(source) == 0 }) {
		t.Fatalf("logouts = %d, parked = %d; want the idle connection logged out", len(commands.matching("LOGOUT")), parkedCount(source))
	}
	markSeen(t, source, host, port)
	if logins := len(commands.matching("LOGIN")); logins != 2 {
		t.Errorf("logins = %d, want 2", logins)
	}
}

// An account edited to another server never reuses its connection to the old one.
func TestEditedAccountDoesNotReuseTheOldServer(t *testing.T) {
	t.Parallel()
	oldCommands, newCommands := &commandLog{}, &commandLog{}
	oldHost, oldPort := listenFake(t, script{commands: oldCommands})
	newHost, newPort := listenFake(t, script{commands: newCommands})
	source := fakeSource()
	markSeen(t, source, oldHost, oldPort)
	markSeen(t, source, newHost, newPort)
	if logins := len(newCommands.matching("LOGIN")); logins != 1 {
		t.Errorf("logins on the new server = %d, want 1", logins)
	}
	if noops := len(oldCommands.matching("NOOP")); noops != 0 {
		t.Errorf("the old server's connection was offered for the edited account")
	}
}

// Two connections in use at once both come back; one is parked and the other logged out, so the idle
// count never passes one.
func TestASecondConcurrentConnectionIsLoggedOut(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands})
	source, account, ctx := fakeSource(), fakeAccount(t, host, port), context.Background()
	first, err := source.connect(ctx, account)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	second, err := source.connect(ctx, account)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	source.release(ctx, first)
	source.release(ctx, second)
	if logouts := len(commands.matching("LOGOUT")); logouts != 1 || parkedCount(source) != 1 {
		t.Errorf("logouts = %d, parked = %d; want 1 and 1", logouts, parkedCount(source))
	}
}

// A connection the lot never lent is logged out rather than parked; one already closed is dropped.
func TestUnlentAndClosedConnectionsAreNotParked(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands})
	source, account := fakeSource(), fakeAccount(t, host, port)
	unlent, err := source.authWith(account, "secret")
	if err != nil {
		t.Fatalf("authWith: %v", err)
	}
	source.parking.giveBack(unlent)
	if logouts := len(commands.matching("LOGOUT")); logouts != 1 {
		t.Errorf("logouts = %d, want 1 for the unlent connection", logouts)
	}
	closed, err := source.connect(context.Background(), account)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	_ = closed.Close()
	if !eventually(func() bool { return isClosed(closed) }) {
		t.Fatal("the closed client never reported closed")
	}
	source.parking.giveBack(closed)
	if parkedCount(source) != 0 {
		t.Errorf("a closed connection was parked")
	}
}

// The idle timer of a slot an action has already taken leaves the slot that replaced it alone.
func TestAStaleIdleTimerLeavesTheCurrentSlotAlone(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands})
	source := fakeSource()
	markSeen(t, source, host, port)
	source.parking.expire(parkKeyFor(fakeAccount(t, host, port)), &parked{})
	if logouts := len(commands.matching("LOGOUT")); logouts != 0 || parkedCount(source) != 1 {
		t.Errorf("logouts = %d, parked = %d; want the current slot untouched", logouts, parkedCount(source))
	}
}

// within answers a panic and a command that outlives its limit as errors rather than ending the run or
// waiting on.
func TestWithinAnswersAPanicAndATimeoutAsErrors(t *testing.T) {
	t.Parallel()
	if err := within(waitLimit, func() error { panic("boom") }); err == nil {
		t.Error("a panicking command answered no error")
	}
	stuck := make(chan struct{})
	defer close(stuck)
	if err := within(shortLimit, func() error { <-stuck; return nil }); err == nil {
		t.Error("a command past its limit answered no error")
	}
}
