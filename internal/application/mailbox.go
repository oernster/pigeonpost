package application

import (
	"context"
	"fmt"
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

// UnreadTotals carries the per-account unread message counts and their sum across all accounts.
type UnreadTotals struct {
	Total     int
	ByAccount map[string]int
}

// UnreadCounts returns the unread message count for each account and the total across all accounts,
// computed from the local cache. The per-account map never contains a nil value; an account with no
// unread messages is simply absent.
func (s *MailboxService) UnreadCounts(ctx context.Context) (UnreadTotals, error) {
	byAccount, err := s.mail.UnreadByAccount(ctx, s.clock.Now())
	if err != nil {
		return UnreadTotals{}, fmt.Errorf("unread counts: %w", err)
	}
	total := 0
	for _, n := range byAccount {
		total += n
	}
	return UnreadTotals{Total: total, ByAccount: byAccount}, nil
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
