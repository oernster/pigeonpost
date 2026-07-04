package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

func openRawStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "raw.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

const rawAccountInsert = `INSERT INTO account
	(id, display_name, email, protocol, in_host, in_port, in_security, out_host, out_port, out_security, auth)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

// TestListAccountsRebuildErrors inserts rows that pass the SQL schema but fail domain reconstruction,
// exercising every rebuild-error branch in scanAccount.
func TestListAccountsRebuildErrors(t *testing.T) {
	ctx := context.Background()
	cases := map[string][]any{
		"bad email":    {"a", "Name", "notanemail", 0, "h", 993, 0, "h", 465, 0, 0},
		"bad incoming": {"a", "Name", "u@x.com", 0, "", 993, 0, "h", 465, 0, 0},
		"bad outgoing": {"a", "Name", "u@x.com", 0, "h", 993, 0, "", 465, 0, 0},
		"bad account":  {"a", "", "u@x.com", 0, "h", 993, 0, "h", 465, 0, 0},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			store := openRawStore(t)
			if _, err := store.db.ExecContext(ctx, rawAccountInsert, args...); err != nil {
				t.Fatalf("insert: %v", err)
			}
			if _, err := store.ListAccounts(ctx); err == nil {
				t.Error("expected a rebuild error from ListAccounts")
			}
			if _, err := store.GetAccount(ctx, "a"); err == nil {
				t.Error("expected a rebuild error from GetAccount")
			}
		})
	}
}

func TestListFoldersRebuildError(t *testing.T) {
	ctx := context.Background()
	store := openRawStore(t)
	// unread greater than total is invalid per the domain.
	if _, err := store.db.ExecContext(ctx,
		`INSERT INTO folder (id, account_id, path, kind, unread, total) VALUES ('f','a','INBOX',0,5,2);`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := store.ListFolders(ctx, "a"); err == nil {
		t.Error("expected a folder rebuild error")
	}
}

const rawMessageInsert = `INSERT INTO message
	(id, folder_id, uid, message_id, from_display, from_address, subject, date_ms, size, flags, has_attachments, snippet)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

func TestListMessagesRebuildErrors(t *testing.T) {
	ctx := context.Background()
	t.Run("empty uid", func(t *testing.T) {
		store := openRawStore(t)
		if _, err := store.db.ExecContext(ctx, rawMessageInsert, "m", "f", "", "", "", "", "s", 0, 10, 0, 0, ""); err != nil {
			t.Fatalf("insert: %v", err)
		}
		if _, err := store.ListMessages(ctx, "f"); err == nil {
			t.Error("expected a uid rebuild error")
		}
	})
	t.Run("bad sender", func(t *testing.T) {
		store := openRawStore(t)
		if _, err := store.db.ExecContext(ctx, rawMessageInsert, "m", "f", 1, "", "", "notvalid", "s", 0, 10, 0, 0, ""); err != nil {
			t.Fatalf("insert: %v", err)
		}
		if _, err := store.ListMessages(ctx, "f"); err == nil {
			t.Error("expected a sender rebuild error")
		}
	})
}

// TestSaveFoldersInsertError forces a duplicate primary key mid-transaction, covering the insert
// error branch and the transaction rollback path.
func TestSaveFoldersInsertError(t *testing.T) {
	ctx := context.Background()
	store := openRawStore(t)
	folder, _ := domain.NewFolder("dup", "a", "INBOX", domain.FolderInbox, 0, 0)
	if err := store.SaveFolders(ctx, "a", []domain.Folder{folder, folder}); err == nil {
		t.Error("expected a duplicate-id insert error")
	}
}

func TestSaveMessagesInsertError(t *testing.T) {
	ctx := context.Background()
	store := openRawStore(t)
	msg := buildMessage(t, "dup", time.Unix(0, 0).UTC(), false)
	if err := store.SaveMessages(ctx, "f", []domain.MessageSummary{msg, msg}); err == nil {
		t.Error("expected a duplicate-id insert error")
	}
}

// TestClosedStoreErrors exercises the query/exec/transaction error branches by operating on a store
// whose database handle has been closed.
func TestClosedStoreErrors(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "closed.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	account := buildAccount(t, "a1")
	folder, _ := domain.NewFolder("f", "a1", "INBOX", domain.FolderInbox, 0, 0)
	msg := buildMessage(t, "m1", time.Unix(0, 0).UTC(), true)
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if _, err := store.ListAccounts(ctx); err == nil {
		t.Error("ListAccounts should fail on a closed store")
	}
	if _, err := store.GetAccount(ctx, "a1"); err == nil {
		t.Error("GetAccount should fail on a closed store")
	}
	if err := store.SaveAccount(ctx, account); err == nil {
		t.Error("SaveAccount should fail on a closed store")
	}
	if _, err := store.ListFolders(ctx, "a1"); err == nil {
		t.Error("ListFolders should fail on a closed store")
	}
	if err := store.SaveFolders(ctx, "a1", []domain.Folder{folder}); err == nil {
		t.Error("SaveFolders should fail on a closed store")
	}
	if _, err := store.ListMessages(ctx, "f"); err == nil {
		t.Error("ListMessages should fail on a closed store")
	}
	if err := store.SaveMessages(ctx, "f", []domain.MessageSummary{msg}); err == nil {
		t.Error("SaveMessages should fail on a closed store")
	}
	if err := store.SetSeen(ctx, "m1", true); err == nil {
		t.Error("SetSeen should fail on a closed store")
	}
}
