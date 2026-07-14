package main

import (
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// AccountDTO is the JSON-serialisable view of an account sent to the front end. Server settings are
// included so the edit wizard can prefill them; the password is never part of this view.
type AccountDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Protocol    string `json:"protocol"`
	InHost      string `json:"inHost"`
	InPort      int    `json:"inPort"`
	InSecurity  string `json:"inSecurity"`
	OutHost     string `json:"outHost"`
	OutPort     int    `json:"outPort"`
	OutSecurity string `json:"outSecurity"`
	Signature   string `json:"signature"`
	// Auth is the account's authentication method ("password" or "oauth2"). The edit wizard reads it to
	// pick the edit form: a password account collects a password and server settings that are re-verified;
	// an OAuth account edits only its profile fields, because the token is not a password to check against
	// the server.
	Auth string `json:"auth"`
	// Identities are the account's alternate sender addresses (aliases it may send as, beyond its primary
	// address), so the compose window can offer them as From options and the edit wizard can manage them.
	Identities []AddressDTO `json:"identities"`
}

// FolderDTO is the JSON-serialisable view of a folder.
type FolderDTO struct {
	ID        string `json:"id"`
	AccountID string `json:"accountId"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Unread    int    `json:"unread"`
	Total     int    `json:"total"`
}

// BulkResultDTO is the outcome of a batched message action (delete or move): the ids the server
// actually acted on (so the UI drops exactly those), the count that could not be processed and a
// human-readable error when any failed. The facade returns this instead of an error so a partial
// success still reports what went.
type BulkResultDTO struct {
	Ids    []string `json:"ids"`
	Failed int      `json:"failed"`
	Error  string   `json:"error"`
}

// UnreadCountsDTO is the JSON-serialisable view of unread message counts: the total across every
// account and the per-account breakdown keyed by account id.
type UnreadCountsDTO struct {
	Total     int            `json:"total"`
	ByAccount map[string]int `json:"byAccount"`
}

// AddressDTO is the JSON-serialisable view of one email address with its optional display name.
type AddressDTO struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// MessageDTO is the JSON-serialisable view of a message summary.
type MessageDTO struct {
	ID       string `json:"id"`
	FolderID string `json:"folderId"`
	// AccountID is the owning account, stamped only by the unified-mailbox listings so their rows can be
	// labelled per account. It is empty on a per-folder listing, which is already scoped to one account
	// the front end knows.
	AccountID      string       `json:"accountId"`
	Subject        string       `json:"subject"`
	FromName       string       `json:"fromName"`
	FromAddress    string       `json:"fromAddress"`
	To             []AddressDTO `json:"to"`
	Cc             []AddressDTO `json:"cc"`
	Date           string       `json:"date"`
	Size           int          `json:"size"`
	Read           bool         `json:"read"`
	Flagged        bool         `json:"flagged"`
	HasAttachments bool         `json:"hasAttachments"`
	// Answered is set once the message has been replied to (\Answered); Forwarded once it has been forwarded
	// ($Forwarded keyword). They drive the small replied/forwarded glyphs on the row.
	Answered  bool   `json:"answered"`
	Forwarded bool   `json:"forwarded"`
	Snippet   string `json:"snippet"`
	// TagColours are the hex colours of the tags on this message, for the coloured dots shown on the row.
	TagColours []string `json:"tagColours"`
}

// MessagePageDTO is one keyset page of a folder's messages for the reading list's incremental load. The
// cursor is opaque to the frontend: it passes NextCursorDateMs and NextCursorID straight back to fetch
// the following page while HasMore is true and never constructs a cursor itself.
type MessagePageDTO struct {
	Messages         []MessageDTO `json:"messages"`
	HasMore          bool         `json:"hasMore"`
	NextCursorDateMs int64        `json:"nextCursorDateMs"`
	NextCursorID     string       `json:"nextCursorId"`
}

// SearchHitDTO is one search result row: the matched message plus a snippet of the matched text. The
// snippet wraps each matched term between the control characters U+0001 and U+0002; the front end
// splits on those to render highlights, so message content is never interpreted as markup. The snippet
// is empty for a purely structural query (one with no search text).
type SearchHitDTO struct {
	Message MessageDTO `json:"message"`
	Snippet string     `json:"snippet"`
}

// SearchResultDTO carries one search's hits, most relevant first, plus whether the query text failed
// structural parsing and was searched as plain text (so the UI can hint that operators were ignored).
type SearchResultDTO struct {
	Hits     []SearchHitDTO `json:"hits"`
	Degraded bool           `json:"degraded"`
}

// AttachmentDTO is the JSON-serialisable metadata of one received attachment. It carries no bytes: the
// reader lists attachments by name and size, and SaveAttachment resolves the content by index from the
// cached body when the user saves one.
type AttachmentDTO struct {
	Index       int    `json:"index"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
}

// MessageBodyDTO is the JSON-serialisable full body of a message. HasInvite is true when the message
// carried a text/calendar scheduling payload, so the reader offers the meeting actions. Attachments lists
// the files the message carried, for the reader to show and the user to save.
type MessageBodyDTO struct {
	Plain       string          `json:"plain"`
	HTML        string          `json:"html"`
	HasInvite   bool            `json:"hasInvite"`
	Attachments []AttachmentDTO `json:"attachments"`
}

func toMessageBodyDTO(b domain.MessageBody) MessageBodyDTO {
	attachments := b.Attachments()
	dtos := make([]AttachmentDTO, 0, len(attachments))
	for index, attachment := range attachments {
		dtos = append(dtos, AttachmentDTO{
			Index:       index,
			Filename:    attachment.Filename(),
			ContentType: attachment.ContentType(),
			Size:        attachment.Size(),
		})
	}
	return MessageBodyDTO{Plain: b.Plain(), HTML: b.HTML(), HasInvite: b.HasInvite(), Attachments: dtos}
}

// TagDTO is the JSON-serialisable view of a coloured tag.
type TagDTO struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Colour string `json:"colour"`
}

func toTagDTO(t domain.Tag) TagDTO {
	return TagDTO{ID: t.ID(), Name: t.Name(), Colour: t.Colour().Hex()}
}

func toTagDTOs(tags []domain.Tag) []TagDTO {
	out := make([]TagDTO, 0, len(tags))
	for _, t := range tags {
		out = append(out, toTagDTO(t))
	}
	return out
}

// TemplateDTO is the JSON-serialisable view of a message template. The body is HTML.
type TemplateDTO struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func toTemplateDTO(t domain.Template) TemplateDTO {
	return TemplateDTO{ID: t.ID(), Name: t.Name(), Subject: t.Subject(), Body: t.Body()}
}

func toTemplateDTOs(templates []domain.Template) []TemplateDTO {
	out := make([]TemplateDTO, 0, len(templates))
	for _, t := range templates {
		out = append(out, toTemplateDTO(t))
	}
	return out
}

func toAccountDTO(a domain.Account) AccountDTO {
	return AccountDTO{
		ID:          a.ID(),
		DisplayName: a.DisplayName(),
		Email:       a.Address().Address(),
		Protocol:    a.Protocol().String(),
		InHost:      a.Incoming().Host(),
		InPort:      a.Incoming().Port(),
		InSecurity:  a.Incoming().Security().String(),
		OutHost:     a.Outgoing().Host(),
		OutPort:     a.Outgoing().Port(),
		OutSecurity: a.Outgoing().Security().String(),
		Signature:   a.Signature(),
		Auth:        a.Auth().String(),
		Identities:  toAddressDTOs(a.Identities()),
	}
}

func toFolderDTO(f domain.Folder) FolderDTO {
	return FolderDTO{
		ID:        f.ID(),
		AccountID: f.AccountID(),
		Path:      f.Path(),
		Name:      f.Name(),
		Kind:      f.Kind().String(),
		Unread:    f.Unread(),
		Total:     f.Total(),
	}
}

func toMessageDTO(m domain.MessageSummary, tagColours []string) MessageDTO {
	date := ""
	if !m.Date().IsZero() {
		date = m.Date().Format(time.RFC3339)
	}
	if tagColours == nil {
		tagColours = []string{}
	}
	return MessageDTO{
		ID:             m.ID(),
		FolderID:       m.FolderID(),
		Subject:        m.Subject(),
		FromName:       m.From().Display(),
		FromAddress:    m.From().Address(),
		To:             toAddressDTOs(m.To()),
		Cc:             toAddressDTOs(m.Cc()),
		Date:           date,
		Size:           m.Size(),
		Read:           m.IsRead(),
		Flagged:        m.IsFlagged(),
		HasAttachments: m.HasAttachments(),
		Answered:       m.IsAnswered(),
		Forwarded:      m.IsForwarded(),
		Snippet:        m.Snippet(),
		TagColours:     tagColours,
	}
}

func toAddressDTOs(addrs []domain.EmailAddress) []AddressDTO {
	out := make([]AddressDTO, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, AddressDTO{Name: a.Display(), Address: a.Address()})
	}
	return out
}

func toAccountDTOs(accounts []domain.Account) []AccountDTO {
	out := make([]AccountDTO, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, toAccountDTO(a))
	}
	return out
}

func toFolderDTOs(folders []domain.Folder) []FolderDTO {
	out := make([]FolderDTO, 0, len(folders))
	for _, f := range folders {
		out = append(out, toFolderDTO(f))
	}
	return out
}

func toMessageDTOs(messages []domain.MessageSummary, coloursByID map[string][]string) []MessageDTO {
	out := make([]MessageDTO, 0, len(messages))
	for _, m := range messages {
		out = append(out, toMessageDTO(m, coloursByID[m.ID()]))
	}
	return out
}

// messageIDs collects the ids of a set of message summaries, for a batched tag colour lookup.
func messageIDs(messages []domain.MessageSummary) []string {
	ids := make([]string, len(messages))
	for i, m := range messages {
		ids[i] = m.ID()
	}
	return ids
}

// ThreadDTO is the JSON-serialisable view of a conversation: its display subject, message and unread
// counts, and its messages oldest first. The front end shows the latest message as the collapsed row and
// reveals the rest when the thread is expanded.
type ThreadDTO struct {
	Subject     string       `json:"subject"`
	Count       int          `json:"count"`
	UnreadCount int          `json:"unreadCount"`
	Messages    []MessageDTO `json:"messages"`
}

func toThreadDTOs(threads []domain.Thread) []ThreadDTO {
	out := make([]ThreadDTO, 0, len(threads))
	for _, thread := range threads {
		out = append(out, ThreadDTO{
			Subject:     thread.Subject(),
			Count:       thread.Count(),
			UnreadCount: thread.UnreadCount(),
			Messages:    toMessageDTOs(thread.Messages(), nil),
		})
	}
	return out
}
