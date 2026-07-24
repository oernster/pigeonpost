package main

import (
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/mailparse"
)

// ComposeRequest is the front-end payload for sending a message. Recipients are comma-free single
// addresses; the front end splits any user input into this list. HoldSeconds is the undo-send window:
// greater than zero queues the send for that long (cancellable via the returned outbox id) and zero or
// less sends immediately. SendAtMs is send-later: a Unix-millisecond instant queues the send held until
// then (cancellable from the Outbox) and takes precedence over the undo window; zero means no schedule.
type ComposeRequest struct {
	AccountID            string                `json:"accountId"`
	From                 string                `json:"from"`
	To                   []string              `json:"to"`
	Cc                   []string              `json:"cc"`
	Bcc                  []string              `json:"bcc"`
	Subject              string                `json:"subject"`
	Body                 string                `json:"body"`
	HTMLBody             string                `json:"htmlBody"`
	AttachmentPaths      []string              `json:"attachmentPaths"`
	AttachmentData       []AttachmentDataEntry `json:"attachmentData"`
	AttachmentMessageIDs []string              `json:"attachmentMessageIds"`
	HoldSeconds          int                   `json:"holdSeconds"`
	SendAtMs             int64                 `json:"sendAtMs"`
}

// AttachmentDataEntry is an attachment whose bytes came over the bridge rather than from a path: a file
// pasted or dropped into the compose window, which the webview holds as name plus content with no
// filesystem path to read from. Content is base64, decoded here rather than typed as []byte so the
// bridge's wire form is explicit on both sides.
type AttachmentDataEntry struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

// rfc822ContentType is the MIME type for a whole email attached to another email.
const rfc822ContentType = "message/rfc822"

const (
	bytesPerMebibyte        = 1 << 20
	maxAttachmentMebibytes  = 25
	maxTotalAttachmentBytes = maxAttachmentMebibytes * bytesPerMebibyte
)

// SendMessage parses the request's addresses and sends the message through the compose use case. With
// a SendAtMs the send is scheduled for that instant; with a positive HoldSeconds it is queued behind an
// undo-send window. Both return the queued item's id (Cancel send and Undo are CancelOutboxItem);
// otherwise it sends immediately and the id is empty.
func (a *App) SendMessage(req ComposeRequest) (string, error) {
	to, err := parseAddresses(req.To)
	if err != nil {
		return "", err
	}
	cc, err := parseAddresses(req.Cc)
	if err != nil {
		return "", err
	}
	bcc, err := parseAddresses(req.Bcc)
	if err != nil {
		return "", err
	}
	attachments, err := a.composeAttachments(req)
	if err != nil {
		return "", err
	}
	draft := application.Draft{
		From:        req.From,
		To:          to,
		Cc:          cc,
		Bcc:         bcc,
		Subject:     req.Subject,
		Body:        req.Body,
		HTMLBody:    req.HTMLBody,
		Attachments: attachments,
	}
	if req.SendAtMs > 0 {
		return a.compose.ScheduleSend(a.ctx, req.AccountID, draft, time.UnixMilli(req.SendAtMs))
	}
	return a.compose.HoldSend(a.ctx, req.AccountID, draft, time.Duration(req.HoldSeconds)*time.Second)
}

// SaveDraft stores an in-progress message in the account's Drafts mailbox. The message may be
// incomplete: unlike SendMessage it does not require recipients.
func (a *App) SaveDraft(req ComposeRequest) error {
	to, err := parseAddresses(req.To)
	if err != nil {
		return err
	}
	cc, err := parseAddresses(req.Cc)
	if err != nil {
		return err
	}
	bcc, err := parseAddresses(req.Bcc)
	if err != nil {
		return err
	}
	attachments, err := a.composeAttachments(req)
	if err != nil {
		return err
	}
	return a.compose.SaveDraft(a.ctx, req.AccountID, application.Draft{
		From:        req.From,
		To:          to,
		Cc:          cc,
		Bcc:         bcc,
		Subject:     req.Subject,
		Body:        req.Body,
		HTMLBody:    req.HTMLBody,
		Attachments: attachments,
	})
}

// OutboxCount returns the number of outgoing operations queued while the server was offline, awaiting
// replay. The front end shows this so the user knows mail is waiting to be sent.
func (a *App) OutboxCount() (int, error) {
	return a.compose.PendingOutbox(a.ctx)
}

// ReplayOutbox attempts to deliver every queued outgoing operation, oldest first, and returns how many
// succeeded. It is called after a successful sync, when connectivity has returned.
func (a *App) ReplayOutbox() (int, error) {
	return a.compose.ReplayOutbox(a.ctx)
}

// composeAttachments gathers all attachments for an outgoing message: files chosen from disk, files
// pasted or dropped in as bytes, then any existing emails referenced by id, kept in that order. The
// size cap covers the images embedded in the HTML body too, since they ship as message parts just as
// attachments do.
func (a *App) composeAttachments(req ComposeRequest) ([]domain.Attachment, error) {
	files, err := readAttachments(req.AttachmentPaths)
	if err != nil {
		return nil, err
	}
	pasted, err := dataAttachments(req.AttachmentData)
	if err != nil {
		return nil, err
	}
	messages, err := a.messageAttachments(req.AttachmentMessageIDs)
	if err != nil {
		return nil, err
	}
	all := append(append(files, pasted...), messages...)
	total := mailparse.DataImageBytes(req.HTMLBody)
	for _, attachment := range all {
		total += attachment.Size()
	}
	if total > maxTotalAttachmentBytes {
		return nil, fmt.Errorf("attachments and embedded images total %d MB, over the %d MB limit",
			total/bytesPerMebibyte, maxAttachmentMebibytes)
	}
	return all, nil
}

// dataAttachments decodes each pasted or dropped file into a domain Attachment (the domain defaults
// an empty content type).
func dataAttachments(entries []AttachmentDataEntry) ([]domain.Attachment, error) {
	out := make([]domain.Attachment, 0, len(entries))
	for _, entry := range entries {
		content, err := base64.StdEncoding.DecodeString(entry.Content)
		if err != nil {
			return nil, fmt.Errorf("decode pasted attachment %q: %w", entry.Name, err)
		}
		attachment, err := domain.NewAttachment(entry.Name, entry.ContentType, content)
		if err != nil {
			return nil, fmt.Errorf("build pasted attachment %q: %w", entry.Name, err)
		}
		out = append(out, attachment)
	}
	return out, nil
}

// messageAttachments fetches each referenced message's raw bytes and wraps it as a message/rfc822
// attachment named from its subject, for attaching an existing email to a new one.
func (a *App) messageAttachments(ids []string) ([]domain.Attachment, error) {
	out := make([]domain.Attachment, 0, len(ids))
	for _, id := range ids {
		raw, err := a.body.RawMessage(a.ctx, id)
		if err != nil {
			return nil, err
		}
		attachment, err := domain.NewAttachment(emlAttachmentName(raw.Subject), rfc822ContentType, raw.Raw)
		if err != nil {
			return nil, err
		}
		out = append(out, attachment)
	}
	return out, nil
}

// emlAttachmentName builds a safe .eml filename from a message subject, replacing characters a
// filesystem rejects and falling back to a default when the subject is empty.
func emlAttachmentName(subject string) string {
	replace := func(r rune) rune {
		switch r {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		if r < ' ' {
			return '-'
		}
		return r
	}
	cleaned := strings.TrimSpace(strings.Map(replace, subject))
	if cleaned == "" {
		cleaned = "message"
	}
	return cleaned + ".eml"
}

// readAttachments loads each chosen file into a domain Attachment, taking the display name from the
// file's base name and the content type from its extension (the domain defaults it when unknown).
func readAttachments(paths []string) ([]domain.Attachment, error) {
	out := make([]domain.Attachment, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read attachment %q: %w", path, err)
		}
		contentType := mime.TypeByExtension(filepath.Ext(path))
		attachment, err := domain.NewAttachment(filepath.Base(path), contentType, content)
		if err != nil {
			return nil, fmt.Errorf("build attachment %q: %w", path, err)
		}
		out = append(out, attachment)
	}
	return out, nil
}

func parseAddresses(values []string) ([]domain.EmailAddress, error) {
	out := make([]domain.EmailAddress, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		addr, err := domain.NewEmailAddress("", trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid address %q: %w", trimmed, err)
		}
		out = append(out, addr)
	}
	return out, nil
}
