package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/emersion/go-message/mail"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/mailparse"
)

// messageFileMode is the permission for an exported .eml file: readable and writable by the owner,
// readable by others, matching a normal user document.
const messageFileMode os.FileMode = 0o644

// SaveMessageAs fetches a message's raw RFC822 bytes and writes them to a file the user chooses through
// a native save dialog, producing a standard .eml file that other mail clients can open. A cancelled
// dialog (empty path) is a no-op that returns no error.
func (a *App) SaveMessageAs(messageID, suggestedName string) error {
	raw, err := a.body.Raw(a.ctx, messageID)
	if err != nil {
		return err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: suggestedName,
		Title:           "Save message as",
		Filters: []runtime.FileFilter{
			{DisplayName: "Email files (*.eml)", Pattern: "*.eml"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return fmt.Errorf("save message dialog: %w", err)
	}
	if path == "" {
		return nil
	}
	if err := os.WriteFile(path, raw, messageFileMode); err != nil {
		return fmt.Errorf("write message file %q: %w", path, err)
	}
	return nil
}

// SaveAttachment writes one of a received message's attachments to a file the user chooses through a
// native save dialog. The attachment is identified by its index in the message body's attachment list;
// its bytes come from the locally cached body, so the save works offline once the message has been
// opened. A cancelled dialog (empty path) is a no-op that returns no error.
func (a *App) SaveAttachment(messageID string, index int) error {
	body, err := a.body.Body(a.ctx, messageID)
	if err != nil {
		return err
	}
	attachments := body.Attachments()
	if index < 0 || index >= len(attachments) {
		return fmt.Errorf("attachment index %d out of range for message %q", index, messageID)
	}
	attachment := attachments[index]
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: attachment.Filename(),
		Title:           "Save attachment",
		Filters: []runtime.FileFilter{
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return fmt.Errorf("save attachment dialog: %w", err)
	}
	if path == "" {
		return nil
	}
	if err := os.WriteFile(path, attachment.Content(), messageFileMode); err != nil {
		return fmt.Errorf("write attachment file %q: %w", path, err)
	}
	return nil
}

// PickAttachments opens a native dialog for choosing one or more files to attach to a message and
// returns their paths. A cancelled dialog returns an empty list and no error.
func (a *App) PickAttachments() ([]string, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Attach files",
	})
	if err != nil {
		return nil, fmt.Errorf("attach files dialog: %w", err)
	}
	return paths, nil
}

// attachmentAt fetches one attachment of a message by its index in the body's attachment list, from the
// locally cached body so it works offline once the message has been opened.
func (a *App) attachmentAt(messageID string, index int) (domain.Attachment, error) {
	body, err := a.body.Body(a.ctx, messageID)
	if err != nil {
		return domain.Attachment{}, err
	}
	attachments := body.Attachments()
	if index < 0 || index >= len(attachments) {
		return domain.Attachment{}, fmt.Errorf("attachment index %d out of range for message %q", index, messageID)
	}
	return attachments[index], nil
}

// OpenAttachment writes one of a message's attachments to a temporary file and opens it with the operating
// system's default application for that file type, so a received file can be viewed without saving it by
// hand first. Its bytes come from the locally cached body. The temporary copy is left for the OS to reap,
// since the launched application may still be reading it when this returns.
func (a *App) OpenAttachment(messageID string, index int) error {
	attachment, err := a.attachmentAt(messageID, index)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "pigeonpost-attachment-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	path := filepath.Join(dir, safeAttachmentName(attachment.Filename()))
	if err := os.WriteFile(path, attachment.Content(), messageFileMode); err != nil {
		return fmt.Errorf("write attachment file %q: %w", path, err)
	}
	if err := openWithDefaultApp(path); err != nil {
		return fmt.Errorf("open attachment %q: %w", path, err)
	}
	return nil
}

// SaveAllAttachments writes every attachment of a message into a folder the user chooses through a native
// directory dialog, so a message with several attachments is saved in one step. A name already present in
// the folder gets a numbered suffix so nothing is overwritten. A cancelled dialog (empty path) is a no-op.
func (a *App) SaveAllAttachments(messageID string) error {
	body, err := a.body.Body(a.ctx, messageID)
	if err != nil {
		return err
	}
	attachments := body.Attachments()
	if len(attachments) == 0 {
		return nil
	}
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Save all attachments to folder"})
	if err != nil {
		return fmt.Errorf("save all attachments dialog: %w", err)
	}
	if dir == "" {
		return nil
	}
	for _, attachment := range attachments {
		path := uniqueAttachmentPath(dir, safeAttachmentName(attachment.Filename()))
		if err := os.WriteFile(path, attachment.Content(), messageFileMode); err != nil {
			return fmt.Errorf("write attachment file %q: %w", path, err)
		}
	}
	return nil
}

// safeAttachmentName reduces a filename to its base and drops any path separators, so a crafted attachment
// name can never write outside the chosen directory. It falls back to a generic name when nothing usable
// is left.
func safeAttachmentName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "." || base == string(filepath.Separator) || strings.TrimSpace(base) == "" {
		return "attachment"
	}
	return base
}

// uniqueAttachmentPath returns a path in dir for name that does not already exist, inserting " (2)",
// " (3)" and so on before the extension when the plain name is taken, so several same-named attachments
// are all kept rather than overwriting each other.
func uniqueAttachmentPath(dir, name string) string {
	candidate := filepath.Join(dir, name)
	if !pathExists(candidate) {
		return candidate
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 2; ; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
		if !pathExists(candidate) {
			return candidate
		}
	}
}

// pathExists reports whether something already exists at path.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// openWithDefaultApp launches a file with the operating system's default handler for its type.
func openWithDefaultApp(path string) error {
	switch goruntime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

// EmailView is a parsed .eml attachment prepared for the in-app viewer: its key headers and its sanitised
// body, so an attached email opens inside PigeonPost rather than an external mail client.
type EmailView struct {
	Subject string `json:"subject"`
	From    string `json:"from"`
	To      string `json:"to"`
	Date    string `json:"date"`
	HTML    string `json:"html"`
	Plain   string `json:"plain"`
}

// OpenEmailAttachment parses an attached email (a message/rfc822 attachment, saved with a .eml name) into
// its headers and sanitised body for the in-app viewer, so the user reads it in PigeonPost rather than
// handing the .eml to an external mail client.
func (a *App) OpenEmailAttachment(messageID string, index int) (EmailView, error) {
	attachment, err := a.attachmentAt(messageID, index)
	if err != nil {
		return EmailView{}, err
	}
	return parseEmailView(attachment.Content())
}

// emailFileView reads a .eml file from disk and parses it into the viewer's headers and sanitised body,
// reusing the same parse and remote-image blocking as an attached email. It backs opening a .eml that the
// OS handed to PigeonPost as the registered file handler.
func emailFileView(path string) (EmailView, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return EmailView{}, fmt.Errorf("read email file: %w", err)
	}
	return parseEmailView(raw)
}

// parseEmailView reads a raw RFC 5322 message into the headers and body the viewer shows. The body reuses
// the same sanitiser and remote-image blocking as the main reader, so an attached email is as safe to view
// as an ordinary one.
func parseEmailView(raw []byte) (EmailView, error) {
	reader, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return EmailView{}, fmt.Errorf("parse attached email: %w", err)
	}
	subject, _ := reader.Header.Subject()
	date := ""
	if d, err := reader.Header.Date(); err == nil {
		date = d.Format("2006-01-02 15:04")
	}
	parsed, err := mailparse.ParseBody(raw)
	if err != nil {
		return EmailView{}, err
	}
	return EmailView{
		Subject: subject,
		From:    headerAddresses(reader.Header, "From"),
		To:      headerAddresses(reader.Header, "To"),
		Date:    date,
		HTML:    parsed.HTML,
		Plain:   parsed.Plain,
	}, nil
}

// headerAddresses formats an address header (From or To) as a comma-separated "Name <address>" list for
// display. It returns an empty string when the header is absent or unparseable.
func headerAddresses(header mail.Header, key string) string {
	addrs, err := header.AddressList(key)
	if err != nil || len(addrs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if addr.Name != "" {
			parts = append(parts, fmt.Sprintf("%s <%s>", addr.Name, addr.Address))
		} else {
			parts = append(parts, addr.Address)
		}
	}
	return strings.Join(parts, ", ")
}
