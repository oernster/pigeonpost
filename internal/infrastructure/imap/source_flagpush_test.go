package imap

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// queuedIntents is how many pending intents the wire tests replay: enough that one connection each is
// unmistakable beside one connection for all.
const queuedIntents = 20

// queuedUIDs answers queuedIntents UIDs, starting at the one the scripted server reports holding.
func queuedUIDs() []string {
	uids := make([]string, queuedIntents)
	for i := range uids {
		uids[i] = strconv.Itoa(heldUID + i)
	}
	return uids
}

// heldUID is the one message the scripted server's FETCH reports, carrying \Seen.
const heldUID = 7

// The replay as it was: each pending intent pushed through the one-message setter. Measured here so the
// difference is on record: one login per intent, which with 14785 intents was a login every few
// seconds for as long as PigeonPost ran.
func TestReplayingThroughTheOneMessageSetterLogsInPerIntent(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands})
	source, account := fakeSource(), fakeAccount(t, host, port)
	for _, uid := range queuedUIDs() {
		if err := source.SetSeen(context.Background(), account, fakeFolder(t), uid, true); err != nil {
			t.Fatalf("SetSeen %s: %v", uid, err)
		}
	}
	if logins := len(commands.matching("LOGIN")); logins != queuedIntents {
		t.Errorf("logins = %d, want %d: one per intent", logins, queuedIntents)
	}
}

// PushFlag carries every intent of a folder on one connection and answers what the server settled: the
// message it reports with the flag as asked, plus every message it no longer holds.
func TestPushFlagUsesOneConnectionAndAnswersWhatSettled(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands})
	settled, err := fakeSource().PushFlag(context.Background(), fakeAccount(t, host, port), fakeFolder(t), queuedUIDs(), domain.FlagSeen, true)
	if err != nil {
		t.Fatalf("PushFlag: %v", err)
	}
	if logins := len(commands.matching("LOGIN")); logins != 1 {
		t.Errorf("logins = %d, want 1 for all %d intents", logins, queuedIntents)
	}
	if !reflect.DeepEqual(settled, queuedUIDs()) {
		t.Errorf("settled = %v, want every UID: the held one agrees and the rest are gone", settled)
	}
}

// A message the server still reports otherwise is not settled, so its intent stays to be retried.
func TestPushFlagLeavesOutWhatTheServerStillReportsOtherwise(t *testing.T) {
	t.Parallel()
	host, port := listenFake(t, script{})
	uids := []string{strconv.Itoa(heldUID), strconv.Itoa(heldUID + 1)}
	// Clearing \Seen: the scripted server still reports the held message as \Seen.
	settled, err := fakeSource().PushFlag(context.Background(), fakeAccount(t, host, port), fakeFolder(t), uids, domain.FlagSeen, false)
	if err != nil {
		t.Fatalf("PushFlag: %v", err)
	}
	if want := uids[1:]; !reflect.DeepEqual(settled, want) {
		t.Errorf("settled = %v, want %v", settled, want)
	}
}

// A flag with no server counterpart connects to nothing; neither does an empty batch.
func TestPushFlagWithNothingToPushDoesNotConnect(t *testing.T) {
	t.Parallel()
	commands := &commandLog{}
	host, port := listenFake(t, script{commands: commands})
	account := fakeAccount(t, host, port)
	for name, call := range map[string]func() ([]string, error){
		"no server flag": func() ([]string, error) {
			return fakeSource().PushFlag(context.Background(), account, fakeFolder(t), []string{"7"}, domain.FlagDraft, true)
		},
		"no uids": func() ([]string, error) {
			return fakeSource().PushFlag(context.Background(), account, fakeFolder(t), nil, domain.FlagSeen, true)
		},
	} {
		if settled, err := call(); err != nil || len(settled) != 0 {
			t.Errorf("%s: settled %v, err %v; want nothing", name, settled, err)
		}
	}
	if logins := len(commands.matching("LOGIN")); logins != 0 {
		t.Errorf("logins = %d, want 0", logins)
	}
}
