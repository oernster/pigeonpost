package mailrouter

import (
	"context"
	"reflect"
	"testing"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// The router is the sync's MailSource, UIDVALIDITY fetch included; a drifted signature is a build
// failure here rather than one reported from wherever the router is next wired.
var _ application.MailSource = (*Router)(nil)

// reportedValidity is the UIDVALIDITY the validity-reporting fake adapter answers with.
const reportedValidity uint32 = 77

// validityRecorder is a recorder whose adapter also reports a folder's UIDVALIDITY, as the IMAP adapter does.
type validityRecorder struct {
	*recorder
}

func (r validityRecorder) FetchMessagesValidity(context.Context, domain.Account, domain.Folder) ([]domain.MessageSummary, uint32, error) {
	r.calls = append(r.calls, "messages-validity")
	return nil, reportedValidity, nil
}

// TestRouterReportsUIDValidityWhereTheAdapterHasOne pins both arms. An adapter that reports UIDVALIDITY
// (IMAP) is asked for it; one that has none (POP3) is fetched as before with no value reported, so the
// sync keeps its earlier behaviour for it.
func TestRouterReportsUIDValidityWhereTheAdapterHasOne(t *testing.T) {
	ctx := context.Background()
	imapRec, pop3Rec := validityRecorder{&recorder{}}, &recorder{}
	router := NewRouter(imapRec, pop3Rec)
	folder, err := domain.NewFolder("f1", "a1", "INBOX", domain.FolderInbox, 0, 0)
	if err != nil {
		t.Fatalf("folder: %v", err)
	}

	if _, validity, err := router.FetchMessagesValidity(ctx, testAccount(t, domain.ProtocolIMAP), folder); err != nil || validity != reportedValidity {
		t.Errorf("IMAP: UIDVALIDITY %d, err %v; want %d", validity, err, reportedValidity)
	}
	if _, validity, err := router.FetchMessagesValidity(ctx, testAccount(t, domain.ProtocolPOP3), folder); err != nil || validity != 0 {
		t.Errorf("POP3: UIDVALIDITY %d, err %v; want none", validity, err)
	}
	if want := []string{"messages-validity"}; !reflect.DeepEqual(imapRec.calls, want) {
		t.Errorf("imap adapter calls = %v, want %v", imapRec.calls, want)
	}
	if want := []string{"messages"}; !reflect.DeepEqual(pop3Rec.calls, want) {
		t.Errorf("pop3 adapter calls = %v, want %v", pop3Rec.calls, want)
	}
}
