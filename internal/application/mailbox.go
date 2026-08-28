package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// MailboxService is the use-case boundary for reading cached folders and messages. It reads from the
// local store only, so it never blocks on the network. Its listings are the visible views: a message
// hidden by an unexpired snooze is excluded until it comes due, which is why the clock is injected.
type MailboxService struct {
	mail  MailStore
	loc   *time.Location
	clock domain.Clock
}

// NewMailboxService constructs the service with its injected store, the location the search-query
// date operators (before:/after:/on:) are interpreted in (normally the user's local time zone) and the
// clock that decides which snoozed messages are currently hidden.
func NewMailboxService(mail MailStore, loc *time.Location, clock domain.Clock) *MailboxService {
	return &MailboxService{mail: mail, loc: loc, clock: clock}
}

// Folders returns the cached folders for an account.
func (s *MailboxService) Folders(ctx context.Context, accountID string) ([]domain.Folder, error) {
	folders, err := s.mail.ListFolders(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list folders for account %q: %w", accountID, err)
	}
	return folders, nil
}

// Messages returns the cached message summaries visible in a folder: a snoozed message stays hidden
// until it comes due.
func (s *MailboxService) Messages(ctx context.Context, folderID string) ([]domain.MessageSummary, error) {
	messages, err := s.mail.ListMessagesVisible(ctx, folderID, s.clock.Now())
	if err != nil {
		return nil, fmt.Errorf("list messages for folder %q: %w", folderID, err)
	}
	return messages, nil
}

// MessagesPage returns one keyset page of a folder's cached message summaries. The first page passes
// hasCursor false; each later page passes the previous page's last row (date and id) so the reading list
// can load a large folder incrementally rather than all at once.
func (s *MailboxService) MessagesPage(ctx context.Context, folderID string, hasCursor bool, cursorDateMs int64, cursorID string, limit int, ascending bool) ([]domain.MessageSummary, error) {
	messages, err := s.mail.ListMessagesPageVisible(ctx, folderID, hasCursor, cursorDateMs, cursorID, limit, ascending, s.clock.Now())
	if err != nil {
		return nil, fmt.Errorf("page messages for folder %q: %w", folderID, err)
	}
	return messages, nil
}

// Threads returns the cached messages of a folder grouped into conversations, newest conversation first.
// Grouping is done in the domain from the same summaries Messages returns, so a threaded and a flat view
// read the identical cache.
func (s *MailboxService) Threads(ctx context.Context, folderID string) ([]domain.Thread, error) {
	messages, err := s.mail.ListMessagesVisible(ctx, folderID, s.clock.Now())
	if err != nil {
		return nil, fmt.Errorf("list messages for folder %q: %w", folderID, err)
	}
	return domain.GroupThreads(messages), nil
}

// conversationLimit caps how many of an account's messages one conversation lookup considers. A thread
// is a handful of messages; the cap is there so a pathological subject (an empty one, which every
// blank-subject message shares) cannot pull an entire mailbox into memory.
const conversationLimit = 200

// ConversationEntry is one message of a conversation, carried with the folder it lives in so the reader
// can say where each message sits: the question a cross-folder thread has to answer is which of these
// you sent and which you received.
type ConversationEntry struct {
	Summary    domain.MessageSummary
	FolderName string
	FolderKind domain.FolderKind
}

// Conversation returns every cached message that threads with the given one, across all of its
// account's folders, oldest first. This is what makes a conversation navigable rather than merely
// visible: the list's threading groups a folder's own messages, so a reply the user sent (which lives
// in Sent) or an earlier message they filed elsewhere never appears beside the message that answers it.
//
// Membership is domain.ThreadKey equality, the same rule the list's grouping uses, so a message never
// threads one way in the list and another way here. The store's suffix match narrows the candidates;
// this comparison decides. A message whose folder has since been removed from the cache is skipped
// rather than shown under a folder that cannot be named.
func (s *MailboxService) Conversation(ctx context.Context, messageID string) ([]ConversationEntry, error) {
	message, err := s.mail.GetMessage(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("locate message %q: %w", messageID, err)
	}
	folder, err := s.mail.GetFolder(ctx, message.FolderID())
	if err != nil {
		return nil, fmt.Errorf("locate folder %q: %w", message.FolderID(), err)
	}
	folders, err := s.mail.ListFolders(ctx, folder.AccountID())
	if err != nil {
		return nil, fmt.Errorf("list folders for account %q: %w", folder.AccountID(), err)
	}
	byID := make(map[string]domain.Folder, len(folders))
	for _, f := range folders {
		byID[f.ID()] = f
	}

	key := domain.ThreadKey(message.Subject())
	candidates, err := s.mail.ThreadMessages(ctx, folder.AccountID(), key, conversationLimit)
	if err != nil {
		return nil, fmt.Errorf("list conversation for %q: %w", messageID, err)
	}
	entries := make([]ConversationEntry, 0, len(candidates))
	for _, c := range candidates {
		if domain.ThreadKey(c.Subject()) != key {
			continue
		}
		home, ok := byID[c.FolderID()]
		if !ok {
			continue
		}
		entries = append(entries, ConversationEntry{Summary: c, FolderName: home.Name(), FolderKind: home.Kind()})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Summary.Date().Before(entries[j].Summary.Date())
	})
	return entries, nil
}

// UnreadTotals carries the per-account unread message counts, their sum across all accounts and each
// account's newest unread message date in Unix milliseconds (absent, like the count, when an account
// has no unread mail).
type UnreadTotals struct {
	Total           int
	ByAccount       map[string]int
	NewestByAccount map[string]int64
}

// UnreadCounts returns the unread message count for each account, the total across all accounts and
// the newest unread date per account, computed from the local cache. The per-account maps never
// contain a nil value; an account with no unread messages is simply absent.
func (s *MailboxService) UnreadCounts(ctx context.Context) (UnreadTotals, error) {
	now := s.clock.Now()
	byAccount, err := s.mail.UnreadByAccount(ctx, now)
	if err != nil {
		return UnreadTotals{}, fmt.Errorf("unread counts: %w", err)
	}
	newest, err := s.mail.NewestUnreadByAccount(ctx, now)
	if err != nil {
		return UnreadTotals{}, fmt.Errorf("newest unread: %w", err)
	}
	total := 0
	for _, n := range byAccount {
		total += n
	}
	return UnreadTotals{Total: total, ByAccount: byAccount, NewestByAccount: newest}, nil
}

// searchResultLimit caps how many hits one search returns, so a two-letter query over a huge cache
// cannot flood the UI; relevance ordering means the cap drops only the weakest matches.
const searchResultLimit = 500

// Search parses raw user input through the query grammar and returns the matching cached messages,
// most relevant first. folderID and accountID are the UI's scope selection (empty for all mail); the
// in: and account: operators inside the query compose with them. Parsing never fails: structurally
// unparseable input degrades to plain free text, reported through the degraded flag so the UI can hint
// that operators were ignored. An empty or blank query returns no results.
func (s *MailboxService) Search(ctx context.Context, raw, folderID, accountID string) ([]SearchHit, bool, error) {
	query := domain.ParseSearchQuery(raw, s.loc)
	if query.IsEmpty() {
		return nil, query.IsDegraded(), nil
	}
	query = query.WithFolderScope(folderID).WithAccountScope(accountID)
	hits, err := s.mail.SearchMessages(ctx, query, searchResultLimit)
	if err != nil {
		return nil, false, fmt.Errorf("search messages for %q: %w", raw, err)
	}
	return hits, query.IsDegraded(), nil
}
