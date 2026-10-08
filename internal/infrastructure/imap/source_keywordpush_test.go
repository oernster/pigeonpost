package imap

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// pushedKeyword is the tag keyword the wire tests replay. The scripted server's FETCH never reports it.
const pushedKeyword = "$PPtag_work"

// queuedUIDsBesideHeld answers queuedUIDs without the one the scripted server holds: what settles when
// the held message is read back still disagreeing.
func queuedUIDsBesideHeld() []string {
	return queuedUIDs()[1:]
}

// Where the server keeps keywords, every intent of a folder goes on one connection and what the read-back
// confirms is settled: here the gone messages, while the held one still lacks the keyword.
func TestPushKeywordUsesOneConnectionAndSettlesWhereTheServerKeepsIt(t *testing.T) {
	t.Parallel()
	for name, permanent := range map[string][]string{
		"wildcard":    {`\Seen`, `\*`},
		"the keyword": {`\Seen`, pushedKeyword},
	} {
		commands := &commandLog{}
		host, port := listenFake(t, script{commands: commands, permanentFlags: permanent})
		settled, err := fakeSource().PushKeyword(context.Background(), fakeAccount(t, host, port), fakeFolder(t), queuedUIDs(), pushedKeyword, true)
		if err != nil {
			t.Fatalf("%s: PushKeyword: %v", name, err)
		}
		if logins := len(commands.matching("LOGIN")); logins != 1 {
			t.Errorf("%s: logins = %d, want 1 for all %d intents", name, logins, queuedIntents)
		}
		if stores := commands.matching("UID STORE"); len(stores) == 0 || !strings.Contains(stores[0], "+FLAGS.SILENT ("+pushedKeyword+")") {
			t.Errorf("%s: stores = %v, want the keyword added", name, stores)
		}
		if !reflect.DeepEqual(settled, queuedUIDsBesideHeld()) {
			t.Errorf("%s: settled = %v, want %v", name, settled, queuedUIDsBesideHeld())
		}
	}
}

// A server that does not say it keeps the keyword may hold it for the session only, so the read-back
// cannot be trusted: the keyword is still pushed, nothing is settled and no read-back is spent.
func TestPushKeywordSettlesNoAddWhereTheServerMayNotKeepIt(t *testing.T) {
	t.Parallel()
	for name, permanent := range map[string][]string{
		"no PERMANENTFLAGS":    nil,
		"system flags only":    {`\Seen`, `\Flagged`},
		"empty PERMANENTFLAGS": {},
	} {
		commands := &commandLog{}
		host, port := listenFake(t, script{commands: commands, permanentFlags: permanent})
		settled, err := fakeSource().PushKeyword(context.Background(), fakeAccount(t, host, port), fakeFolder(t), queuedUIDs(), pushedKeyword, true)
		if err != nil {
			t.Fatalf("%s: PushKeyword: %v", name, err)
		}
		if len(settled) != 0 {
			t.Errorf("%s: settled = %v, want none", name, settled)
		}
		if stores := commands.matching("UID STORE"); len(stores) == 0 {
			t.Errorf("%s: the keyword was not pushed", name)
		}
		if reads := commands.matching("UID FETCH"); len(reads) != 0 {
			t.Errorf("%s: read back %v, want no read-back", name, reads)
		}
	}
}

// A removal is safe to settle whatever the server keeps: a keyword it could not keep is absent anyway.
func TestPushKeywordSettlesARemovalWhereverTheReadBackAgrees(t *testing.T) {
	t.Parallel()
	host, port := listenFake(t, script{})
	settled, err := fakeSource().PushKeyword(context.Background(), fakeAccount(t, host, port), fakeFolder(t), queuedUIDs(), pushedKeyword, false)
	if err != nil {
		t.Fatalf("PushKeyword: %v", err)
	}
	if !reflect.DeepEqual(settled, queuedUIDs()) {
		t.Errorf("settled = %v, want every UID", settled)
	}
}

// An empty batch connects to nothing.
func TestPushKeywordWithNoUIDsDoesNotConnect(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands})
	settled, err := fakeSource().PushKeyword(context.Background(), fakeAccount(t, host, port), fakeFolder(t), nil, pushedKeyword, true)
	if err != nil || len(settled) != 0 {
		t.Errorf("settled %v, err %v; want nothing", settled, err)
	}
	if logins := len(commands.matching("LOGIN")); logins != 0 {
		t.Errorf("logins = %d, want 0", logins)
	}
}
