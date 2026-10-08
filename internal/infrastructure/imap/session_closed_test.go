package imap

import (
	"context"
	"testing"
)

// An unreadable body structure makes the client close its connection. Under a sync session that was the
// session's shared connection. The fallback fetch must get a working connection rather than the closed
// one; so must every folder after it in the same sync.
func TestSessionReplacesAConnectionTheClientClosed(t *testing.T) {
	t.Parallel()
	host, port := listenFake(t, script{bodyStructure: unreadableBodyStructure})
	source, account := fakeSource(), fakeAccount(t, host, port)
	ctx, end := source.BeginSession(context.Background(), account)
	defer end()
	for i := range 2 {
		messages, err := source.FetchMessages(ctx, account, fakeFolder(t))
		if err != nil {
			t.Fatalf("folder %d: FetchMessages under a session: %v", i+1, err)
		}
		if len(messages) != 1 {
			t.Fatalf("folder %d: summaries = %d, want 1", i+1, len(messages))
		}
	}
}
