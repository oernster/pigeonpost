package mailrouter

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// lister is a recorder that can also list, standing in for the IMAP adapter.
type lister struct {
	recorder
}

func (l *lister) FetchListing(context.Context, domain.Account, domain.Folder) ([]domain.MessageState, uint32, error) {
	l.calls = append(l.calls, "listing")
	return nil, 1, nil
}

func (l *lister) FetchMessagesByUID(context.Context, domain.Account, domain.Folder, []string) ([]domain.MessageSummary, uint32, error) {
	l.calls = append(l.calls, "by-uid")
	return nil, 1, nil
}

// An adapter that can list is asked. One that cannot (POP3) reports no UIDVALIDITY, so the sync fetches
// in full; it also refuses a by-UID fetch it is never meant to be asked for.
func TestRouterListsOnlyWhereTheAdapterCan(t *testing.T) {
	imapAdapter, pop3Adapter := &lister{}, &recorder{}
	router := NewRouter(imapAdapter, pop3Adapter)
	var folder domain.Folder
	imapAccount, pop3Account := testAccount(t, domain.ProtocolIMAP), testAccount(t, domain.ProtocolPOP3)

	if _, validity, err := router.FetchListing(context.Background(), imapAccount, folder); err != nil || validity != 1 {
		t.Errorf("IMAP listing: validity %d err %v", validity, err)
	}
	if _, _, err := router.FetchMessagesByUID(context.Background(), imapAccount, folder, []string{"1"}); err != nil {
		t.Errorf("IMAP by-UID: %v", err)
	}
	if len(imapAdapter.calls) != 2 {
		t.Errorf("IMAP adapter calls = %v, want listing and by-uid", imapAdapter.calls)
	}

	if listing, validity, err := router.FetchListing(context.Background(), pop3Account, folder); err != nil || listing != nil || validity != noUIDValidity {
		t.Errorf("POP3 listing: %v under %d, err %v; want none reported", listing, validity, err)
	}
	if _, _, err := router.FetchMessagesByUID(context.Background(), pop3Account, folder, []string{"1"}); !errors.Is(err, errNoListing) {
		t.Errorf("POP3 by-UID error = %v, want errNoListing", err)
	}
	if len(pop3Adapter.calls) != 0 {
		t.Errorf("POP3 adapter was called: %v", pop3Adapter.calls)
	}
}
