package main

import (
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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
