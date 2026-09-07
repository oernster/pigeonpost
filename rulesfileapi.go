package main

// rulesfileapi.go is the Wails surface for moving a rule set between installations through a file:
// export writes every rule, import reads a file back. It owns the native dialogs and the file I/O and
// nothing else; the format lives in infrastructure/rulefile and the reconciliation in the use case.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/infrastructure/rulefile"
)

// rulesFileName is the name a rules export is offered under. It carries the .json extension because
// that is what the file is, so a text editor and a file manager both know what to do with it.
const rulesFileName = "pigeonpost-rules.json"

// RuleImportPlanDTO is what importing the chosen file WOULD do, reported before anything is written.
// Cancelled is set when the user closed the dialog, which is not an error.
type RuleImportPlanDTO struct {
	Cancelled bool `json:"cancelled"`
	// Path is handed back to ApplyRuleImport, so the file is read again rather than the plan carrying
	// a copy of it through the interface and back.
	Path string `json:"path"`
	File string `json:"file"`
	// Add, Replace, Disable and Destructive name rules rather than counting them, so the confirmation
	// can say which rules it means.
	Add         []string `json:"add"`
	Replace     []string `json:"replace"`
	Disable     []string `json:"disable"`
	Destructive []string `json:"destructive"`
}

// RuleImportResultDTO reports what an import actually did.
type RuleImportResultDTO struct {
	Added    int      `json:"added"`
	Replaced int      `json:"replaced"`
	Disabled []string `json:"disabled"`
}

// ExportRulesToFile writes every filter rule to a file the user chooses through a native save dialog.
// It reports whether a file was written: a cancelled dialog is a no-op rather than an error.
func (a *App) ExportRulesToFile() (bool, error) {
	data, err := a.ruleTransfer.Export(a.ctx, rulefile.New())
	if err != nil {
		return false, err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: rulesFileName,
		Title:           "Export rules",
		Filters:         []runtime.FileFilter{{DisplayName: "Rules file (*.json)", Pattern: "*.json"}},
	})
	if err != nil {
		return false, fmt.Errorf("export rules dialog: %w", err)
	}
	if path == "" {
		return false, nil
	}
	if err := os.WriteFile(path, data, messageFileMode); err != nil {
		return false, fmt.Errorf("write rules file %q: %w", path, err)
	}
	return true, nil
}

// PreviewRuleImport opens a rules file and reports what importing it would do, writing nothing. A
// rule set can move and destroy mail unattended, so the file is described and agreed to before it is
// applied rather than reported afterwards.
func (a *App) PreviewRuleImport() (RuleImportPlanDTO, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Import rules",
		Filters: []runtime.FileFilter{{DisplayName: "Rules file (*.json)", Pattern: "*.json"}},
	})
	if err != nil {
		return RuleImportPlanDTO{}, fmt.Errorf("import rules dialog: %w", err)
	}
	if path == "" {
		return RuleImportPlanDTO{Cancelled: true}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return RuleImportPlanDTO{}, fmt.Errorf("read rules file %q: %w", path, err)
	}
	plan, err := a.ruleTransfer.Plan(a.ctx, rulefile.New(), data)
	if err != nil {
		return RuleImportPlanDTO{}, err
	}
	return RuleImportPlanDTO{
		Path: path, File: filepath.Base(path),
		Add:         stringsOrEmpty(plan.Add),
		Replace:     stringsOrEmpty(plan.Replace),
		Disable:     stringsOrEmpty(plan.Disable),
		Destructive: stringsOrEmpty(plan.Destructive),
	}, nil
}

// ApplyRuleImport imports the file the plan named. The file is read again rather than carried through
// the interface: it is the same path the user chose a moment earlier, so re-reading keeps one code
// path from bytes to stored rules.
func (a *App) ApplyRuleImport(path string) (RuleImportResultDTO, error) {
	if path == "" {
		return RuleImportResultDTO{Disabled: []string{}}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return RuleImportResultDTO{}, fmt.Errorf("read rules file %q: %w", path, err)
	}
	result, err := a.ruleTransfer.Import(a.ctx, rulefile.New(), data)
	if err != nil {
		return RuleImportResultDTO{}, err
	}
	return RuleImportResultDTO{
		Added: result.Added, Replaced: result.Replaced, Disabled: stringsOrEmpty(result.Disabled),
	}, nil
}

// stringsOrEmpty returns an empty slice rather than nil, because a nil slice encodes as JSON null
// while the front end's generated type declares an array; reading a length off null takes the window
// down rather than one dialog.
func stringsOrEmpty(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
