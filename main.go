package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/infrastructure/imap"
	"github.com/oernster/pigeonpost/internal/infrastructure/keychain"
	"github.com/oernster/pigeonpost/internal/infrastructure/mailrouter"
	"github.com/oernster/pigeonpost/internal/infrastructure/pop3"
	"github.com/oernster/pigeonpost/internal/infrastructure/recurrence"
	"github.com/oernster/pigeonpost/internal/infrastructure/smtp"
	"github.com/oernster/pigeonpost/internal/infrastructure/storage"
	"github.com/oernster/pigeonpost/internal/infrastructure/taskbar"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed VERSION
var versionRaw string

//go:embed LICENSE
var licenceText string

const (
	appName     = "PigeonPost"
	dataDirName = "PigeonPost"
	dbFileName  = "pigeonpost.db"
	windowW     = 1200
	windowH     = 800
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "pigeonpost:", err)
		os.Exit(1)
	}
}

// run is the composition root. It wires concrete infrastructure adapters into the application use
// cases by constructor injection, assembles the Wails facade, and starts the runtime.
func run() error {
	ctx := context.Background()

	dbPath, err := databasePath()
	if err != nil {
		return err
	}
	store, err := storage.Open(ctx, dbPath)
	if err != nil {
		return err
	}

	vault := keychain.NewVault()
	clock := systemClock{}
	// The read paths (sync, body) and account verification are routed by protocol; the write paths
	// (draft append, message and folder actions) remain IMAP-specific, as POP3 has no server-side
	// mailbox actions.
	imapSource := imap.NewSource(vault, clock, newMessageID)
	pop3Source := pop3.NewSource(vault)
	mailSource := mailrouter.NewRouter(imapSource, pop3Source)
	transport := smtp.NewTransport(vault, clock, newMessageID)

	accountService := application.NewAccountService(store, vault, store)
	setupService := application.NewAccountSetupService(store, vault, mailSource)
	mailboxService := application.NewMailboxService(store)
	syncService := application.NewSyncService(store, store, mailSource, store)
	composeService := application.NewComposeService(store, store, transport, imapSource, store, clock, newOutboxID)
	tagService := application.NewTagService(store)
	bodyService := application.NewMessageBodyService(store, store, mailSource)
	actionService := application.NewMessageActionService(store, store, mailSource)
	folderService := application.NewFolderService(store, store, imapSource, imapSource)
	ruleService := application.NewRuleService(store, newRuleID)
	contactService := application.NewContactService(store, newContactID)
	calendarService := application.NewCalendarService(store, newCalendarID, recurrence.New())

	// The taskbar overlay badge reflects the total unread count, and the flasher flashes the taskbar
	// button when a reminder fires while the window is in the background. Both locate the main window by
	// its title, so they are given the same title the Wails window uses below.
	windowTitle := appName + " " + version()
	overlay := taskbar.NewOverlay(windowTitle)
	overlay.Start()
	flasher := taskbar.NewFlasher(windowTitle)
	// The tray icon is persistent and clickable; its context menu emits Wails events the front end maps
	// to the Help dialogs, so the App facade supplies the callbacks at startup once the runtime exists.
	tray := taskbar.NewTray(windowTitle, appName)

	app := NewApp(store.Close, overlay, flasher, tray, accountService, setupService, mailboxService, syncService, composeService, tagService, bodyService, actionService, folderService, ruleService, contactService, calendarService)

	err = wails.Run(&options.App{
		Title:            windowTitle,
		Width:            windowW,
		Height:           windowH,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 22, G: 27, B: 34, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
	})
	if err != nil {
		return fmt.Errorf("run wails: %w", err)
	}
	return nil
}

// version returns the trimmed contents of the embedded VERSION file.
func version() string {
	return strings.TrimSpace(versionRaw)
}

// databasePath resolves the per-user database location and ensures its directory exists.
func databasePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	dir := filepath.Join(base, dataDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create data dir %q: %w", dir, err)
	}
	return filepath.Join(dir, dbFileName), nil
}
