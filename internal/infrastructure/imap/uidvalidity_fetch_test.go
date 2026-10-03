package imap

import (
	"context"
	"testing"

	"github.com/oernster/pigeonpost/internal/application"
)

// The adapter must satisfy the MailSource port, UIDVALIDITY fetch included, with the exact signature
// the router forwards to; a drifted one is a build failure here.
var _ application.MailSource = (*Source)(nil)

// servedValidity is a UIDVALIDITY distinct from the fake's default, so a test cannot pass by reading a
// constant rather than the server's answer.
const servedValidity uint32 = 4242

// plainStructure is a single text part the client reads without trouble, for the ordinary path.
const plainStructure = `("TEXT" "PLAIN" ("CHARSET" "UTF-8") NIL NIL "7BIT" 10 1 NIL NIL NIL NIL)`

// TestFetchMessagesValidityReportsTheSelectedValue proves the UIDVALIDITY returned is the one the server
// gave on the SELECT for this fetch, on the ordinary path and on the fallback that refetches without the
// body structure (whose second session is the one whose UIDs are kept).
func TestFetchMessagesValidityReportsTheSelectedValue(t *testing.T) {
	t.Parallel()
	cases := map[string]script{
		"ordinary": {uidValidity: servedValidity, bodyStructure: plainStructure},
		"fallback": {uidValidity: servedValidity, bodyStructure: unreadableBodyStructure},
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			host, port := listenFake(t, s)
			messages, validity, err := fakeSource().FetchMessagesValidity(context.Background(),
				fakeAccount(t, host, port), fakeFolder(t))
			if err != nil {
				t.Fatalf("FetchMessagesValidity: %v", err)
			}
			if validity != servedValidity {
				t.Errorf("UIDVALIDITY = %d, want %d", validity, servedValidity)
			}
			if len(messages) != 1 {
				t.Errorf("summaries: want 1, got %d", len(messages))
			}
		})
	}
}

// TestFetchMessagesValidityReportsAnEmptyFolder pins that an empty folder still reports its value: mail
// later arriving into it is judged against what is recorded now.
func TestFetchMessagesValidityReportsAnEmptyFolder(t *testing.T) {
	t.Parallel()
	host, port := listenFake(t, script{uidValidity: servedValidity, empty: true})
	messages, validity, err := fakeSource().FetchMessagesValidity(context.Background(),
		fakeAccount(t, host, port), fakeFolder(t))
	if err != nil {
		t.Fatalf("FetchMessagesValidity: %v", err)
	}
	if validity != servedValidity || len(messages) != 0 {
		t.Errorf("got %d message(s) under UIDVALIDITY %d, want none under %d", len(messages), validity, servedValidity)
	}
}
