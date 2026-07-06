package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// fakeAccountStore is a hand-written in-memory AccountStore with error-injection fields.
type fakeAccountStore struct {
	accounts  map[string]domain.Account
	listErr   error
	getErr    error
	saveErr   error
	deleteErr error
	saved     []domain.Account
	deleted   []string
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

func (f *fakeAccountStore) DeleteAccount(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.accounts, id)
	f.deleted = append(f.deleted, id)
	return nil
}

// fakeCredentialStore is a hand-written in-memory CredentialStore with error-injection fields. It
// records which account ids had their secret deleted so rollback can be asserted.
type fakeCredentialStore struct {
	passwords map[string]string
	deleted   []string
	getErr    error
	setErr    error
	deleteErr error
}

func newFakeCredentialStore() *fakeCredentialStore {
	return &fakeCredentialStore{passwords: map[string]string{}}
}

func (f *fakeCredentialStore) Password(_ context.Context, account domain.Account) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.passwords[account.ID()], nil
}

func (f *fakeCredentialStore) SetPassword(_ context.Context, account domain.Account, secret string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.passwords[account.ID()] = secret
	return nil
}

func (f *fakeCredentialStore) DeletePassword(_ context.Context, account domain.Account) error {
	f.deleted = append(f.deleted, account.ID())
	delete(f.passwords, account.ID())
	return f.deleteErr
}

// fakeVerifier is a hand-written AccountVerifier that records the passwords it was asked to verify,
// keyed by account id.
type fakeVerifier struct {
	verified  map[string]string
	verifyErr error
}

func newFakeVerifier() *fakeVerifier {
	return &fakeVerifier{verified: map[string]string{}}
}

func (f *fakeVerifier) Verify(_ context.Context, account domain.Account, password string) error {
	f.verified[account.ID()] = password
	return f.verifyErr
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
	setFlaggedErr    error
	deleteDataErr    error
	getMessageErr    error
	getFolderErr     error
	getBodyErr       error
	saveBodyErr      error
	searchErr        error
	deleteMessageErr error
	unreadByAccount  map[string]int
	unreadErr        error
	bodies           map[string]domain.MessageBody
	searchResults    []domain.MessageSummary
	deletedMessages  []string
	forcedMessage    *domain.MessageSummary
	savedFolderKeys  []string
	savedMessageKeys []string
	deletedData      []string
}

func newFakeMailStore() *fakeMailStore {
	return &fakeMailStore{
		folders:  map[string][]domain.Folder{},
		messages: map[string][]domain.MessageSummary{},
		bodies:   map[string]domain.MessageBody{},
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

func (f *fakeMailStore) UnreadByAccount(_ context.Context) (map[string]int, error) {
	if f.unreadErr != nil {
		return nil, f.unreadErr
	}
	return f.unreadByAccount, nil
}

func (f *fakeMailStore) SaveMessages(_ context.Context, folderID string, messages []domain.MessageSummary) error {
	if f.saveMessagesErr != nil {
		return f.saveMessagesErr
	}
	f.messages[folderID] = messages
	f.savedMessageKeys = append(f.savedMessageKeys, folderID)
	return nil
}

func (f *fakeMailStore) DeleteAccountData(_ context.Context, accountID string) error {
	if f.deleteDataErr != nil {
		return f.deleteDataErr
	}
	delete(f.folders, accountID)
	f.deletedData = append(f.deletedData, accountID)
	return nil
}

func (f *fakeMailStore) GetMessage(_ context.Context, messageID string) (domain.MessageSummary, error) {
	if f.getMessageErr != nil {
		return domain.MessageSummary{}, f.getMessageErr
	}
	if f.forcedMessage != nil {
		return *f.forcedMessage, nil
	}
	for _, msgs := range f.messages {
		for _, m := range msgs {
			if m.ID() == messageID {
				return m, nil
			}
		}
	}
	return domain.MessageSummary{}, errors.New("message not found")
}

func (f *fakeMailStore) GetFolder(_ context.Context, folderID string) (domain.Folder, error) {
	if f.getFolderErr != nil {
		return domain.Folder{}, f.getFolderErr
	}
	for _, folders := range f.folders {
		for _, folder := range folders {
			if folder.ID() == folderID {
				return folder, nil
			}
		}
	}
	return domain.Folder{}, errors.New("folder not found")
}

func (f *fakeMailStore) GetMessageBody(_ context.Context, messageID string) (domain.MessageBody, error) {
	if f.getBodyErr != nil {
		return domain.MessageBody{}, f.getBodyErr
	}
	body, ok := f.bodies[messageID]
	if !ok {
		return domain.MessageBody{}, ErrBodyNotCached
	}
	return body, nil
}

func (f *fakeMailStore) SaveMessageBody(_ context.Context, body domain.MessageBody) error {
	if f.saveBodyErr != nil {
		return f.saveBodyErr
	}
	f.bodies[body.MessageID()] = body
	return nil
}

func (f *fakeMailStore) SearchMessages(_ context.Context, query string) ([]domain.MessageSummary, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.searchResults, nil
}

func (f *fakeMailStore) DeleteMessage(_ context.Context, messageID string) error {
	if f.deleteMessageErr != nil {
		return f.deleteMessageErr
	}
	f.deletedMessages = append(f.deletedMessages, messageID)
	for folderID, msgs := range f.messages {
		kept := msgs[:0]
		for _, m := range msgs {
			if m.ID() != messageID {
				kept = append(kept, m)
			}
		}
		f.messages[folderID] = kept
	}
	return nil
}

func (f *fakeMailStore) SetSeen(_ context.Context, messageID string, seen bool) error {
	if f.setSeenErr != nil {
		return f.setSeenErr
	}
	return f.toggleFlag(messageID, domain.FlagSeen, seen)
}

func (f *fakeMailStore) SetFlagged(_ context.Context, messageID string, flagged bool) error {
	if f.setFlaggedErr != nil {
		return f.setFlaggedErr
	}
	return f.toggleFlag(messageID, domain.FlagFlagged, flagged)
}

func (f *fakeMailStore) toggleFlag(messageID string, flag domain.Flag, set bool) error {
	for folderID, msgs := range f.messages {
		for i, m := range msgs {
			if m.ID() != messageID {
				continue
			}
			flags := m.Flags()
			if set {
				flags = flags.With(flag)
			} else {
				flags = flags.Without(flag)
			}
			f.messages[folderID][i] = m.WithFlags(flags)
			return nil
		}
	}
	return nil
}

// fakeTagStore is a hand-written in-memory TagStore with error-injection fields.
type fakeTagStore struct {
	tags      map[string]domain.Tag
	byMessage map[string][]string
	listErr   error
	saveErr   error
	deleteErr error
	forMsgErr error
	addErr    error
	removeErr error
}

func newFakeTagStore() *fakeTagStore {
	return &fakeTagStore{tags: map[string]domain.Tag{}, byMessage: map[string][]string{}}
}

func (f *fakeTagStore) ListTags(context.Context) ([]domain.Tag, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]domain.Tag, 0, len(f.tags))
	for _, t := range f.tags {
		out = append(out, t)
	}
	return out, nil
}

func (f *fakeTagStore) SaveTag(_ context.Context, tag domain.Tag) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.tags[tag.ID()] = tag
	return nil
}

func (f *fakeTagStore) DeleteTag(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.tags, id)
	return nil
}

func (f *fakeTagStore) TagsForMessage(_ context.Context, messageID string) ([]domain.Tag, error) {
	if f.forMsgErr != nil {
		return nil, f.forMsgErr
	}
	var out []domain.Tag
	for _, id := range f.byMessage[messageID] {
		if t, ok := f.tags[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeTagStore) AddMessageTag(_ context.Context, messageID, tagID string) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.byMessage[messageID] = append(f.byMessage[messageID], tagID)
	return nil
}

func (f *fakeTagStore) RemoveMessageTag(_ context.Context, messageID, tagID string) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	kept := f.byMessage[messageID][:0]
	for _, id := range f.byMessage[messageID] {
		if id != tagID {
			kept = append(kept, id)
		}
	}
	f.byMessage[messageID] = kept
	return nil
}

// fakeMailSource is a hand-written MailSource with error-injection fields.
type fakeMailSource struct {
	folders          []domain.Folder
	messagesByFolder map[string][]domain.MessageSummary
	fetchFoldersErr  error
	fetchMessagesErr error
	fetchBodyErr     error
	bodyPlain        string
	bodyHTML         string
	bodyInvite       []byte
	bodyAttachments  []domain.Attachment
	fetchRawErr      error
	raw              []byte
}

func (f *fakeMailSource) FetchBody(context.Context, domain.Account, domain.Folder, string) (string, string, []byte, []domain.Attachment, error) {
	if f.fetchBodyErr != nil {
		return "", "", nil, nil, f.fetchBodyErr
	}
	return f.bodyPlain, f.bodyHTML, f.bodyInvite, f.bodyAttachments, nil
}

func (f *fakeMailSource) FetchRaw(context.Context, domain.Account, domain.Folder, string) ([]byte, error) {
	if f.fetchRawErr != nil {
		return nil, f.fetchRawErr
	}
	return f.raw, nil
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

// fakeMailActions is a hand-written MailActions that records the operations it was asked to perform.
type fakeMailActions struct {
	setSeenErr       error
	flaggedErr       error
	deleteErr        error
	moveErr          error
	copyErr          error
	seenCalls        []bool
	flaggedCalls     []bool
	deleteTrashPaths []string
	moveDestPaths    []string
	copyDestPaths    []string
}

func (f *fakeMailActions) SetSeen(_ context.Context, _ domain.Account, _ domain.Folder, _ string, seen bool) error {
	if f.setSeenErr != nil {
		return f.setSeenErr
	}
	f.seenCalls = append(f.seenCalls, seen)
	return nil
}

func (f *fakeMailActions) Delete(_ context.Context, _ domain.Account, _ domain.Folder, _ string, trashPath string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleteTrashPaths = append(f.deleteTrashPaths, trashPath)
	return nil
}

func (f *fakeMailActions) SetFlagged(_ context.Context, _ domain.Account, _ domain.Folder, _ string, flagged bool) error {
	if f.flaggedErr != nil {
		return f.flaggedErr
	}
	f.flaggedCalls = append(f.flaggedCalls, flagged)
	return nil
}

func (f *fakeMailActions) Move(_ context.Context, _ domain.Account, _ domain.Folder, _ string, destPath string) error {
	if f.moveErr != nil {
		return f.moveErr
	}
	f.moveDestPaths = append(f.moveDestPaths, destPath)
	return nil
}

func (f *fakeMailActions) Copy(_ context.Context, _ domain.Account, _ domain.Folder, _ string, destPath string) error {
	if f.copyErr != nil {
		return f.copyErr
	}
	f.copyDestPaths = append(f.copyDestPaths, destPath)
	return nil
}

// fakeFolderActions is a hand-written FolderActions recording the folder operations it was asked for.
type fakeFolderActions struct {
	createErr error
	renameErr error
	deleteErr error
	created   []string
	renamed   [][2]string
	deleted   []string
}

func (f *fakeFolderActions) CreateFolder(_ context.Context, _ domain.Account, path string) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, path)
	return nil
}

func (f *fakeFolderActions) RenameFolder(_ context.Context, _ domain.Account, oldPath, newPath string) error {
	if f.renameErr != nil {
		return f.renameErr
	}
	f.renamed = append(f.renamed, [2]string{oldPath, newPath})
	return nil
}

func (f *fakeFolderActions) DeleteFolder(_ context.Context, _ domain.Account, path string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, path)
	return nil
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

// fakeDraftSaver is a hand-written DraftSaver that records the drafts it was asked to append and the
// mailbox path each was appended to.
type fakeDraftSaver struct {
	saveErr error
	saved   []domain.OutgoingMessage
	paths   []string
}

func (f *fakeDraftSaver) SaveDraft(_ context.Context, _ domain.Account, draftsPath string, msg domain.OutgoingMessage) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, msg)
	f.paths = append(f.paths, draftsPath)
	return nil
}

// fakeOutboxStore is a hand-written in-memory OutboxStore with error-injection fields.
type fakeOutboxStore struct {
	items      []domain.OutboxItem
	enqueueErr error
	listErr    error
	deleteErr  error
	markErr    error
	deleted    []string
	failed     map[string]string
}

func (f *fakeOutboxStore) EnqueueOutbox(_ context.Context, item domain.OutboxItem) error {
	if f.enqueueErr != nil {
		return f.enqueueErr
	}
	f.items = append(f.items, item)
	return nil
}

func (f *fakeOutboxStore) ListOutbox(context.Context) ([]domain.OutboxItem, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.items, nil
}

func (f *fakeOutboxStore) DeleteOutbox(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, id)
	kept := f.items[:0]
	for _, item := range f.items {
		if item.ID() != id {
			kept = append(kept, item)
		}
	}
	f.items = kept
	return nil
}

func (f *fakeOutboxStore) MarkOutboxFailed(_ context.Context, id, reason string) error {
	if f.markErr != nil {
		return f.markErr
	}
	if f.failed == nil {
		f.failed = map[string]string{}
	}
	f.failed[id] = reason
	for i, item := range f.items {
		if item.ID() == id {
			f.items[i] = item.WithFailure(reason)
		}
	}
	return nil
}

// fakeDraftRecoveryStore is a hand-written in-memory DraftRecoveryStore with error-injection fields. It
// holds a single snapshot, present when saved reports true, mirroring the one-slot store contract.
type fakeDraftRecoveryStore struct {
	snapshot domain.DraftRecovery
	present  bool
	saveErr  error
	getErr   error
	clearErr error
	cleared  bool
}

func (f *fakeDraftRecoveryStore) SaveDraftRecovery(_ context.Context, recovery domain.DraftRecovery) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.snapshot = recovery
	f.present = true
	return nil
}

func (f *fakeDraftRecoveryStore) GetDraftRecovery(context.Context) (domain.DraftRecovery, bool, error) {
	if f.getErr != nil {
		return domain.DraftRecovery{}, false, f.getErr
	}
	return f.snapshot, f.present, nil
}

func (f *fakeDraftRecoveryStore) ClearDraftRecovery(context.Context) error {
	if f.clearErr != nil {
		return f.clearErr
	}
	f.cleared = true
	f.present = false
	f.snapshot = domain.DraftRecovery{}
	return nil
}

// fakeRuleStore is a hand-written in-memory RuleStore with error-injection fields.
type fakeRuleStore struct {
	rules     []domain.Rule
	listErr   error
	saveErr   error
	deleteErr error
	saved     []domain.Rule
	deleted   []string
}

func (f *fakeRuleStore) ListRules(context.Context) ([]domain.Rule, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.rules, nil
}

func (f *fakeRuleStore) SaveRule(_ context.Context, rule domain.Rule) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, rule)
	f.rules = append(f.rules, rule)
	return nil
}

func (f *fakeRuleStore) DeleteRule(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, id)
	return nil
}

// fakeClock is a hand-written domain.Clock returning a fixed instant.
type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

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
		ID: id, FolderID: folderID, UID: "1", Size: 10, Flags: domain.NewFlags(0),
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	return msg
}
