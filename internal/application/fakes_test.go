package application

import (
	"context"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// fakeAccountStore is a hand-written in-memory AccountStore with error-injection fields.
type fakeAccountStore struct {
	accounts map[string]domain.Account
	listErr  error
	getErr   error
	saveErr  error
	saved    []domain.Account
}

func newFakeAccountStore() *fakeAccountStore {
	return &fakeAccountStore{accounts: map[string]domain.Account{}}
}

func (f *fakeAccountStore) ListAccounts(context.Context) ([]domain.Account, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]domain.Account, 0, len(f.accounts))
	for _, a := range f.accounts {
		out = append(out, a)
	}
	return out, nil
}

func (f *fakeAccountStore) GetAccount(_ context.Context, id string) (domain.Account, error) {
	if f.getErr != nil {
		return domain.Account{}, f.getErr
	}
	a, ok := f.accounts[id]
	if !ok {
		return domain.Account{}, ErrAccountNotFound
	}
	return a, nil
}

func (f *fakeAccountStore) SaveAccount(_ context.Context, account domain.Account) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.accounts[account.ID()] = account
	f.saved = append(f.saved, account)
	return nil
}

// fakeMailStore is a hand-written in-memory MailStore with error-injection fields.
type fakeMailStore struct {
	folders          map[string][]domain.Folder
	messages         map[string][]domain.MessageSummary
	listFoldersErr   error
	saveFoldersErr   error
	listMessagesErr  error
	saveMessagesErr  error
	setSeenErr       error
	savedFolderKeys  []string
	savedMessageKeys []string
}

func newFakeMailStore() *fakeMailStore {
	return &fakeMailStore{
		folders:  map[string][]domain.Folder{},
		messages: map[string][]domain.MessageSummary{},
	}
}

func (f *fakeMailStore) ListFolders(_ context.Context, accountID string) ([]domain.Folder, error) {
	if f.listFoldersErr != nil {
		return nil, f.listFoldersErr
	}
	return f.folders[accountID], nil
}

func (f *fakeMailStore) SaveFolders(_ context.Context, accountID string, folders []domain.Folder) error {
	if f.saveFoldersErr != nil {
		return f.saveFoldersErr
	}
	f.folders[accountID] = folders
	f.savedFolderKeys = append(f.savedFolderKeys, accountID)
	return nil
}

func (f *fakeMailStore) ListMessages(_ context.Context, folderID string) ([]domain.MessageSummary, error) {
	if f.listMessagesErr != nil {
		return nil, f.listMessagesErr
	}
	return f.messages[folderID], nil
}

func (f *fakeMailStore) SaveMessages(_ context.Context, folderID string, messages []domain.MessageSummary) error {
	if f.saveMessagesErr != nil {
		return f.saveMessagesErr
	}
	f.messages[folderID] = messages
	f.savedMessageKeys = append(f.savedMessageKeys, folderID)
	return nil
}

func (f *fakeMailStore) SetSeen(_ context.Context, messageID string, seen bool) error {
	if f.setSeenErr != nil {
		return f.setSeenErr
	}
	for folderID, msgs := range f.messages {
		for i, m := range msgs {
			if m.ID() != messageID {
				continue
			}
			flags := m.Flags()
			if seen {
				flags = flags.With(domain.FlagSeen)
			} else {
				flags = flags.Without(domain.FlagSeen)
			}
			f.messages[folderID][i] = m.WithFlags(flags)
			return nil
		}
	}
	return nil
}

// fakeMailSource is a hand-written MailSource with error-injection fields.
type fakeMailSource struct {
	folders          []domain.Folder
	messagesByFolder map[string][]domain.MessageSummary
	fetchFoldersErr  error
	fetchMessagesErr error
}

func (f *fakeMailSource) FetchFolders(context.Context, domain.Account) ([]domain.Folder, error) {
	if f.fetchFoldersErr != nil {
		return nil, f.fetchFoldersErr
	}
	return f.folders, nil
}

func (f *fakeMailSource) FetchMessages(_ context.Context, _ domain.Account, folder domain.Folder) ([]domain.MessageSummary, error) {
	if f.fetchMessagesErr != nil {
		return nil, f.fetchMessagesErr
	}
	return f.messagesByFolder[folder.ID()], nil
}

// fakeMailTransport is a hand-written MailTransport that records sent messages.
type fakeMailTransport struct {
	sendErr error
	sent    []domain.OutgoingMessage
}

func (f *fakeMailTransport) Send(_ context.Context, _ domain.Account, msg domain.OutgoingMessage) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	f.sent = append(f.sent, msg)
	return nil
}

// --- test builders ---

func testAccount(t *testing.T, id string) domain.Account {
	t.Helper()
	addr, err := domain.NewEmailAddress("", "user@example.com")
	if err != nil {
		t.Fatalf("build address: %v", err)
	}
	sc, err := domain.NewServerConfig("host.example.com", 993, domain.SecurityTLS)
	if err != nil {
		t.Fatalf("build server config: %v", err)
	}
	account, err := domain.NewAccount(id, "Test", addr, domain.ProtocolIMAP, sc, sc, domain.AuthPassword)
	if err != nil {
		t.Fatalf("build account: %v", err)
	}
	return account
}

func testFolder(t *testing.T, id, accountID, path string) domain.Folder {
	t.Helper()
	folder, err := domain.NewFolder(id, accountID, path, domain.FolderInbox, 0, 0)
	if err != nil {
		t.Fatalf("build folder: %v", err)
	}
	return folder
}

func testMessage(t *testing.T, id, folderID string) domain.MessageSummary {
	t.Helper()
	msg, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: id, FolderID: folderID, UID: 1, Size: 10, Flags: domain.NewFlags(0),
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	return msg
}
