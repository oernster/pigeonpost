package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/infrastructure/keychain"
	"github.com/oernster/pigeonpost/internal/installer"
)

// App is the Wails facade for the setup program.
type App struct {
	ctx         context.Context
	payload     []byte
	version     string
	uninstallal bool
}

// NewApp constructs the facade. The setup exe run with -uninstall starts in uninstall mode.
func NewApp(payload []byte, version string) *App {
	uninstall := len(os.Args) > 1 && os.Args[1] == "-uninstall"
	return &App{payload: payload, version: version, uninstallal: uninstall}
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// StateDTO describes what the setup program should offer, based on what is already installed.
type StateDTO struct {
	Mode             string `json:"mode"` // install | manage | uninstall
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion"`
	ThisVersion      string `json:"thisVersion"`
	InstallDir       string `json:"installDir"`
	LaunchOnBoot     bool   `json:"launchOnBoot"`
	UpgradeAvailable bool   `json:"upgradeAvailable"`
}

// Progress is emitted on the "progress" event during long operations.
type Progress struct {
	Pct int    `json:"pct"`
	Msg string `json:"msg"`
}

// DetectState inspects the machine and returns the appropriate setup mode.
func (a *App) DetectState() StateDTO {
	dir, _ := installer.InstallDir()
	installedVer, installed := installer.InstalledVersion()
	mode := "install"
	if installed {
		mode = "manage"
	}
	if a.uninstallal {
		mode = "uninstall"
	}
	return StateDTO{
		Mode:             mode,
		Installed:        installed,
		InstalledVersion: installedVer,
		ThisVersion:      a.version,
		InstallDir:       dir,
		LaunchOnBoot:     installer.IsLaunchOnBoot(),
		UpgradeAvailable: installed && semverNewer(a.version, installedVer),
	}
}

// AppRunning reports whether PigeonPost is currently running, so the front end can offer to close it
// before an install or uninstall rather than failing when the executable is locked.
func (a *App) AppRunning() bool {
	return installer.IsAppRunning()
}

// CloseRunningApp ends the running PigeonPost so setup can proceed, backing the front end's offer to
// close it. It returns an error if the app is still running after being asked to close.
func (a *App) CloseRunningApp() error {
	return installer.CloseRunningApp()
}

// Install performs a fresh install or an upgrade (it overwrites files and preserves user data).
func (a *App) Install(launchOnBoot bool) error {
	return a.installInto(launchOnBoot)
}

// Repair re-extracts and re-registers the application, keeping the current launch-on-boot setting.
func (a *App) Repair() error {
	return a.installInto(installer.IsLaunchOnBoot())
}

func (a *App) installInto(launchOnBoot bool) error {
	// Refuse to overwrite a running instance: extracting over a locked PigeonPost.exe would fail
	// mid-way and leave a half-written install, so ask the user to close it first.
	if installer.IsAppRunning() {
		return installer.ErrAppRunning
	}
	dir, err := installer.InstallDir()
	if err != nil {
		return err
	}
	a.progress(10, "Extracting files...")
	if err := installer.ExtractZip(a.payload, dir); err != nil {
		return fmt.Errorf("extract files: %w", err)
	}
	exePath := filepath.Join(dir, installer.ExeName)

	a.progress(55, "Registering application...")
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate installer: %w", err)
	}
	uninstallExe := filepath.Join(dir, "uninstall.exe")
	if err := copyFile(self, uninstallExe); err != nil {
		return fmt.Errorf("write uninstaller: %w", err)
	}
	sizeKB, _ := installer.DirSizeKB(dir)
	if err := installer.WriteUninstallEntry(installer.UninstallInfo{
		Version:      a.version,
		InstallDir:   dir,
		UninstallExe: uninstallExe,
		IconPath:     exePath,
		EstimatedKB:  sizeKB,
	}); err != nil {
		return fmt.Errorf("register application: %w", err)
	}

	a.progress(80, "Creating shortcuts...")
	installer.CreateShortcuts(exePath, dir)

	a.progress(92, "Applying settings...")
	if err := installer.SetLaunchOnBoot(exePath, launchOnBoot); err != nil {
		return fmt.Errorf("configure launch on boot: %w", err)
	}

	a.progress(100, "Done.")
	return nil
}

// SetLaunchOnBoot toggles the login start entry from the manage screen.
func (a *App) SetLaunchOnBoot(enabled bool) error {
	dir, err := installer.InstallDir()
	if err != nil {
		return err
	}
	exePath := filepath.Join(dir, installer.ExeName)
	return installer.SetLaunchOnBoot(exePath, enabled)
}

// Uninstall removes shortcuts, the login entry, the registry record and the installed files,
// optionally deleting the user's mail data and saved passwords as well.
func (a *App) Uninstall(removeData bool) error {
	// Refuse to remove a running instance: the scheduled directory deletion cannot remove a locked
	// PigeonPost.exe, so ask the user to close it first.
	if installer.IsAppRunning() {
		return installer.ErrAppRunning
	}
	dir, err := installer.InstallDir()
	if err != nil {
		return err
	}
	a.progress(20, "Removing shortcuts...")
	installer.RemoveShortcuts()
	_ = installer.SetLaunchOnBoot("", false)

	a.progress(50, "Removing registry entries...")
	_ = installer.RemoveUninstallEntry()

	if removeData {
		a.progress(70, "Removing your data...")
		if data, err := installer.UserDataDir(); err == nil {
			_ = installer.RemoveTree(data)
		}
		// The mail cache and settings live on disk; the account passwords live in the OS keychain.
		// Removing the user's data means clearing both, so no saved secrets are left behind.
		_ = keychain.NewVault().PurgeAll()
	}

	a.progress(90, "Removing files...")
	installer.ScheduleDirDeletion(dir)

	a.progress(100, "Done.")
	return nil
}

// Quit closes the setup program.
func (a *App) Quit() {
	wailsruntime.Quit(a.ctx)
}

func (a *App) progress(pct int, msg string) {
	wailsruntime.EventsEmit(a.ctx, "progress", Progress{Pct: pct, Msg: msg})
}
