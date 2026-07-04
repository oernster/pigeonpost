package mailrouter

import (
	"context"
	"reflect"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// recorder is a fake protocol source that records the methods it was asked to perform, so a test can
// assert which adapter the router selected.
type recorder struct {
	calls []string
}

func (r *recorder) FetchFolders(context.Context, domain.Account) ([]domain.Folder, error) {
	r.calls = append(r.calls, "folders")
	return nil, nil
}

func (r *recorder) FetchMessages(context.Context, domain.Account, domain.Folder) ([]domain.MessageSummary, error) {
	r.calls = append(r.calls, "messages")
	return nil, nil
}

func (r *recorder) FetchBody(context.Context, domain.Account, domain.Folder, string) (string, string, error) {
	r.calls = append(r.calls, "body")
	return "", "", nil
}

func (r *recorder) FetchRaw(context.Context, domain.Account, domain.Folder, string) ([]byte, error) {
	r.calls = append(r.calls, "raw")
	return nil, nil
}

func (r *recorder) Verify(context.Context, domain.Account, string) error {
	r.calls = append(r.calls, "verify")
	return nil
}

func (r *recorder) SetSeen(context.Context, domain.Account, domain.Folder, string, bool) error {
	r.calls = append(r.calls, "seen")
	return nil
}

func (r *recorder) SetFlagged(context.Context, domain.Account, domain.Folder, string, bool) error {
	r.calls = append(r.calls, "flagged")
	return nil
}

func (r *recorder) Delete(context.Context, domain.Account, domain.Folder, string, string) error {
	r.calls = append(r.calls, "delete")
	return nil
}

func (r *recorder) Move(context.Context, domain.Account, domain.Folder, string, string) error {
	r.calls = append(r.calls, "move")
	return nil
}

func (r *recorder) Copy(context.Context, domain.Account, domain.Folder, string, string) error {
	r.calls = append(r.calls, "copy")
	return nil
}

func testAccount(t *testing.T, protocol domain.Protocol) domain.Account {
	t.Helper()
	addr, err := domain.NewEmailAddress("", "user@example.com")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	in, err := domain.NewServerConfig("in.example.com", 995, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("incoming: %v", err)
	}
	out, err := domain.NewServerConfig("out.example.com", 465, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("outgoing: %v", err)
	}
	account, err := domain.NewAccount("user@example.com", "User", addr, protocol, in, out, domain.AuthPassword)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	return account
}

// exercise calls every router method once for the given account.
func exercise(t *testing.T, router *Router, account domain.Account) {
	t.Helper()
	ctx := context.Background()
	var folder domain.Folder
	if _, err := router.FetchFolders(ctx, account); err != nil {
		t.Fatalf("FetchFolders: %v", err)
	}
	if _, err := router.FetchMessages(ctx, account, folder); err != nil {
		t.Fatalf("FetchMessages: %v", err)
	}
	if _, _, err := router.FetchBody(ctx, account, folder, "1"); err != nil {
		t.Fatalf("FetchBody: %v", err)
	}
	if _, err := router.FetchRaw(ctx, account, folder, "1"); err != nil {
		t.Fatalf("FetchRaw: %v", err)
	}
	if err := router.Verify(ctx, account, "pw"); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if err := router.SetSeen(ctx, account, folder, "1", true); err != nil {
		t.Fatalf("SetSeen: %v", err)
	}
	if err := router.SetFlagged(ctx, account, folder, "1", true); err != nil {
		t.Fatalf("SetFlagged: %v", err)
	}
	if err := router.Delete(ctx, account, folder, "1", ""); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := router.Move(ctx, account, folder, "1", "dest"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if err := router.Copy(ctx, account, folder, "1", "dest"); err != nil {
		t.Fatalf("Copy: %v", err)
	}
}

func TestRouterRoutesImapToImapAdapter(t *testing.T) {
	imapRec, pop3Rec := &recorder{}, &recorder{}
	router := NewRouter(imapRec, pop3Rec)

	exercise(t, router, testAccount(t, domain.ProtocolIMAP))

	want := []string{"folders", "messages", "body", "raw", "verify", "seen", "flagged", "delete", "move", "copy"}
	if !reflect.DeepEqual(imapRec.calls, want) {
		t.Errorf("imap adapter calls = %v, want %v", imapRec.calls, want)
	}
	if len(pop3Rec.calls) != 0 {
		t.Errorf("pop3 adapter was called for an IMAP account: %v", pop3Rec.calls)
	}
}

func TestRouterRoutesPop3ToPop3Adapter(t *testing.T) {
	imapRec, pop3Rec := &recorder{}, &recorder{}
	router := NewRouter(imapRec, pop3Rec)

	exercise(t, router, testAccount(t, domain.ProtocolPOP3))

	want := []string{"folders", "messages", "body", "raw", "verify", "seen", "flagged", "delete", "move", "copy"}
	if !reflect.DeepEqual(pop3Rec.calls, want) {
		t.Errorf("pop3 adapter calls = %v, want %v", pop3Rec.calls, want)
	}
	if len(imapRec.calls) != 0 {
		t.Errorf("imap adapter was called for a POP3 account: %v", imapRec.calls)
	}
}
