package main

// Reading facade: the Wails-exposed methods that read a folder's cached messages for the desktop list
// (flat, paged, threaded and searched) and load an open message's blocked remote images. Kept apart from
// app.go so the composition root stays within the module-size limit, mirroring calendarapi.go and
// contactsapi.go.

import (
	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// ListMessages returns the cached message summaries for a folder.
func (a *App) ListMessages(folderID string) ([]MessageDTO, error) {
	messages, err := a.mailbox.Messages(a.ctx, folderID)
	if err != nil {
		return nil, err
	}
	colours, coloursErr := a.tags.ColoursForMessages(a.ctx, messageIDs(messages))
	if coloursErr != nil {
		// Tag colours are decorative; a failure to load them must not break the message list.
		colours = nil
	}
	return toMessageDTOs(messages, colours), nil
}

// ListMessagesPage returns one keyset page of a folder's cached message summaries for the reading list's
// incremental load. The first call passes hasCursor false; while the returned page reports HasMore, the
// next call passes the page's NextCursor* values back to resume strictly after its last row. limit caps
// the rows read, so a huge folder opens without loading every row at once.
func (a *App) ListMessagesPage(folderID string, hasCursor bool, cursorDateMs int64, cursorID string, limit int, ascending bool) (MessagePageDTO, error) {
	messages, err := a.mailbox.MessagesPage(a.ctx, folderID, hasCursor, cursorDateMs, cursorID, limit, ascending)
	if err != nil {
		return MessagePageDTO{}, err
	}
	colours, coloursErr := a.tags.ColoursForMessages(a.ctx, messageIDs(messages))
	if coloursErr != nil {
		// Tag colours are decorative; a failure to load them must not break the message list.
		colours = nil
	}
	page := MessagePageDTO{Messages: toMessageDTOs(messages, colours), HasMore: len(messages) == limit}
	if len(messages) > 0 {
		last := messages[len(messages)-1]
		page.NextCursorDateMs = last.Date().UnixMilli()
		page.NextCursorID = last.ID()
	}
	return page, nil
}

// ListThreads returns the cached messages of a folder grouped into conversations, newest conversation
// first, for the reading list's conversation view.
func (a *App) ListThreads(folderID string) ([]ThreadDTO, error) {
	threads, err := a.mailbox.Threads(a.ctx, folderID)
	if err != nil {
		return nil, err
	}
	return toThreadDTOs(threads), nil
}

// SearchMessages returns cached messages matching the query, most relevant first. folderID and
// accountID scope the search to the UI's selection (empty for all mail). The query text supports the
// operator grammar: quoted phrases, OR, -negation, from:, to:, subject:, filename:, has:attachment,
// is:unread / is:read / is:flagged, in:<folder>, account:<name> and before:/after:/on: ISO dates.
func (a *App) SearchMessages(query, folderID, accountID string) (SearchResultDTO, error) {
	hits, degraded, err := a.mailbox.Search(a.ctx, query, folderID, accountID)
	if err != nil {
		return SearchResultDTO{}, err
	}
	summaries := make([]domain.MessageSummary, 0, len(hits))
	for _, hit := range hits {
		summaries = append(summaries, hit.Summary)
	}
	colours, coloursErr := a.tags.ColoursForMessages(a.ctx, messageIDs(summaries))
	if coloursErr != nil {
		// Tag colours are decorative; a failure to load them must not break the search list.
		colours = nil
	}
	result := SearchResultDTO{Hits: make([]SearchHitDTO, 0, len(hits)), Degraded: degraded}
	for i, hit := range hits {
		result.Hits = append(result.Hits, SearchHitDTO{
			Message: toMessageDTO(summaries[i], colours[hit.Summary.ID()]),
			Snippet: hit.Snippet,
		})
	}
	return result, nil
}

// ListUnifiedMessages returns every account's cached inbox messages merged into one newest-first list,
// each row stamped with its owning account, for the unified mailbox's conversation view (which needs the
// whole combined set to thread it).
func (a *App) ListUnifiedMessages() ([]MessageDTO, error) {
	merged, err := a.unified.Messages(a.ctx)
	if err != nil {
		return nil, err
	}
	return toUnifiedMessageDTOs(merged, a.unifiedColours(merged)), nil
}

// ListUnifiedMessagesPage returns one keyset page of the combined inbox list for the unified mailbox's
// flat view, with the same cursor mechanics as ListMessagesPage: the first call passes hasCursor false
// and each later call passes the previous page's NextCursor* values back.
func (a *App) ListUnifiedMessagesPage(hasCursor bool, cursorDateMs int64, cursorID string, limit int, ascending bool) (MessagePageDTO, error) {
	merged, err := a.unified.MessagesPage(a.ctx, hasCursor, cursorDateMs, cursorID, limit, ascending)
	if err != nil {
		return MessagePageDTO{}, err
	}
	page := MessagePageDTO{Messages: toUnifiedMessageDTOs(merged, a.unifiedColours(merged)), HasMore: len(merged) == limit}
	if len(merged) > 0 {
		last := merged[len(merged)-1].Summary
		page.NextCursorDateMs = last.Date().UnixMilli()
		page.NextCursorID = last.ID()
	}
	return page, nil
}

// unifiedColours batch-loads the tag colours for a merged listing's rows. Tag colours are decorative;
// a failure to load them must not break the list, so it degrades to none.
func (a *App) unifiedColours(merged []application.UnifiedMessage) map[string][]string {
	ids := make([]string, len(merged))
	for i, m := range merged {
		ids[i] = m.Summary.ID()
	}
	colours, err := a.tags.ColoursForMessages(a.ctx, ids)
	if err != nil {
		return nil
	}
	return colours
}

// toUnifiedMessageDTOs maps merged unified rows to DTOs, stamping each with its owning account so the
// front end can label the row.
func toUnifiedMessageDTOs(merged []application.UnifiedMessage, coloursByID map[string][]string) []MessageDTO {
	out := make([]MessageDTO, 0, len(merged))
	for _, m := range merged {
		dto := toMessageDTO(m.Summary, coloursByID[m.Summary.ID()])
		dto.AccountID = m.AccountID
		out = append(out, dto)
	}
	return out
}

// LoadRemoteImages returns the open message's HTML with its blocked remote images fetched server-side and
// inlined as data: URIs, so the reader can display images a browser cannot load cross-origin (a sender's
// Cross-Origin-Resource-Policy, CORS or hotlink protection). An image that cannot be fetched is left parked,
// so the returned HTML is always usable.
func (a *App) LoadRemoteImages(html string) (string, error) {
	return a.remoteImages.LoadImages(a.ctx, html)
}
