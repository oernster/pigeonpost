package imap

import (
	"context"
	"testing"
)

// syncedFolders stands in for an account's folder count in the full-sync tests; StartMail had 49.
const syncedFolders = 10

// runFullSync makes the calls a full account sync makes (the folder list, then a fetch of each folder)
// with or without a session, then answers how many logins the server saw.
func runFullSync(t *testing.T, session bool) int {
	t.Helper()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands, bodyStructure: plainStructure})
	source, account := fakeSource(), fakeAccount(t, host, port)
	ctx := context.Background()
	if session {
		var end func()
		ctx, end = source.BeginSession(ctx, account)
		defer end()
	}
	if _, err := source.FetchFolders(ctx, account); err != nil {
		t.Fatalf("FetchFolders: %v", err)
	}
	for range syncedFolders {
		if _, err := source.FetchMessages(ctx, account, fakeFolder(t)); err != nil {
			t.Fatalf("FetchMessages: %v", err)
		}
	}
	return len(commands.matching("LOGIN"))
}

// Without a session every call used to log in on its own: the 51 logins in a minute one press of Sync
// made against StartMail's 49 folders. Calls made one after another now reuse the parked connection.
func TestFullSyncWithoutASessionReusesTheParkedConnection(t *testing.T) {
	t.Parallel()
	if logins := runFullSync(t, false); logins != 1 {
		t.Errorf("logins = %d, want 1: each call after the first takes the parked connection", logins)
	}
}

// Under a session the whole sync shares one login.
func TestFullSyncUnderASessionLogsInOnce(t *testing.T) {
	t.Parallel()
	if logins := runFullSync(t, true); logins != 1 {
		t.Errorf("logins = %d, want 1 for the whole sync", logins)
	}
}

// Ending the session parks its connection, so the user's next action reuses it rather than logging in. A
// session for one account never lends its connection to another; ending one that never connected does
// nothing.
func TestSessionEndsAndKeepsToItsAccount(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands})
	source, account := fakeSource(), fakeAccount(t, host, port)

	_, unused := source.BeginSession(context.Background(), account)
	unused()

	ctx, end := source.BeginSession(context.Background(), account)
	if _, err := source.FetchFolders(ctx, account); err != nil {
		t.Fatalf("FetchFolders: %v", err)
	}
	if sessionFor(ctx, fakeAccountWithID(t, host, port, "another")) != nil {
		t.Error("the session was offered to another account")
	}
	end()
	if logouts := len(commands.matching("LOGOUT")); logouts != 0 {
		t.Errorf("logouts = %d, want 0: the session's connection is parked", logouts)
	}
	if err := source.SetSeen(context.Background(), account, fakeFolder(t), "7", true); err != nil {
		t.Fatalf("SetSeen: %v", err)
	}
	if logins := len(commands.matching("LOGIN")); logins != 1 {
		t.Errorf("logins = %d, want 1: the action after the session reuses its connection", logins)
	}
}
