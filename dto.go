package main

import (
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// AccountDTO is the JSON-serialisable view of an account sent to the front end.
type AccountDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Protocol    string `json:"protocol"`
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

// MessageDTO is the JSON-serialisable view of a message summary.
type MessageDTO struct {
	ID             string `json:"id"`
	FolderID       string `json:"folderId"`
	Subject        string `json:"subject"`
	FromName       string `json:"fromName"`
	FromAddress    string `json:"fromAddress"`
	Date           string `json:"date"`
	Size           int    `json:"size"`
	Read           bool   `json:"read"`
	HasAttachments bool   `json:"hasAttachments"`
	Snippet        string `json:"snippet"`
}

func toAccountDTO(a domain.Account) AccountDTO {
	return AccountDTO{
		ID:          a.ID(),
		DisplayName: a.DisplayName(),
		Email:       a.Address().Address(),
		Protocol:    a.Protocol().String(),
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
		Date:           date,
		Size:           m.Size(),
		Read:           m.IsRead(),
		HasAttachments: m.HasAttachments(),
		Snippet:        m.Snippet(),
	}
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
