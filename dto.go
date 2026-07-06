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
	ID             string       `json:"id"`
	FolderID       string       `json:"folderId"`
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
	Snippet        string       `json:"snippet"`
}

// MessageBodyDTO is the JSON-serialisable full body of a message. HasInvite is true when the message
// carried a text/calendar scheduling payload, so the reader offers the meeting actions.
type MessageBodyDTO struct {
	Plain     string `json:"plain"`
	HTML      string `json:"html"`
	HasInvite bool   `json:"hasInvite"`
}

func toMessageBodyDTO(b domain.MessageBody) MessageBodyDTO {
	return MessageBodyDTO{Plain: b.Plain(), HTML: b.HTML(), HasInvite: b.HasInvite()}
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

func toMessageDTO(m domain.MessageSummary) MessageDTO {
	date := ""
	if !m.Date().IsZero() {
		date = m.Date().Format(time.RFC3339)
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
		Snippet:        m.Snippet(),
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

func toMessageDTOs(messages []domain.MessageSummary) []MessageDTO {
	out := make([]MessageDTO, 0, len(messages))
	for _, m := range messages {
		out = append(out, toMessageDTO(m))
	}
	return out
}
