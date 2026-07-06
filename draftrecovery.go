package main

import "github.com/oernster/pigeonpost/internal/application"

// DraftRecoveryRequest is the front-end payload for a local compose snapshot. The recipient fields are
// the raw text as typed, matching the compose window's inputs rather than parsed address lists.
type DraftRecoveryRequest struct {
	AccountID string `json:"accountId"`
	To        string `json:"to"`
	Cc        string `json:"cc"`
	Bcc       string `json:"bcc"`
	Subject   string `json:"subject"`
	BodyHTML  string `json:"bodyHtml"`
}

// DraftRecoveryDTO is the stored compose snapshot returned to the front end. Present is false when no
// snapshot is held, in which case the other fields are empty and the front end offers nothing to restore.
type DraftRecoveryDTO struct {
	Present   bool   `json:"present"`
	AccountID string `json:"accountId"`
	To        string `json:"to"`
	Cc        string `json:"cc"`
	Bcc       string `json:"bcc"`
	Subject   string `json:"subject"`
	BodyHTML  string `json:"bodyHtml"`
	SavedMs   int64  `json:"savedMs"`
}

// SaveDraftRecovery stores a local snapshot of the in-progress compose window for crash and
// accidental-close recovery. It never saves to the server; that is what SaveDraft does.
func (a *App) SaveDraftRecovery(req DraftRecoveryRequest) error {
	return a.compose.SaveDraftRecovery(a.ctx, application.DraftSnapshot{
		AccountID: req.AccountID,
		To:        req.To,
		Cc:        req.Cc,
		Bcc:       req.Bcc,
		Subject:   req.Subject,
		BodyHTML:  req.BodyHTML,
	})
}

// DraftRecovery returns the locally stored compose snapshot, if any, so the front end can offer to
// restore an unsent message.
func (a *App) DraftRecovery() (DraftRecoveryDTO, error) {
	recovery, ok, err := a.compose.DraftRecovery(a.ctx)
	if err != nil {
		return DraftRecoveryDTO{}, err
	}
	if !ok {
		return DraftRecoveryDTO{Present: false}, nil
	}
	return DraftRecoveryDTO{
		Present:   true,
		AccountID: recovery.AccountID(),
		To:        recovery.To(),
		Cc:        recovery.Cc(),
		Bcc:       recovery.Bcc(),
		Subject:   recovery.Subject(),
		BodyHTML:  recovery.BodyHTML(),
		SavedMs:   recovery.SavedAt().UnixMilli(),
	}, nil
}

// ClearDraftRecovery discards the local compose snapshot, called after a send, a server save, or the
// user declining to restore it.
func (a *App) ClearDraftRecovery() error {
	return a.compose.ClearDraftRecovery(a.ctx)
}
