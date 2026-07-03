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
	"github.com/oernster/pigeonpost/internal/infrastructure/smtp"
	"github.com/oernster/pigeonpost/internal/infrastructure/storage"
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
	source := imap.NewSource(vault)
	transport := smtp.NewTransport(vault, systemClock{}, newMessageID)

	accountService := application.NewAccountService(store, vault, store)
	setupService := application.NewAccountSetupService(store, vault, source)
	mailboxService := application.NewMailboxService(store)
	syncService := application.NewSyncService(store, store, source)
	composeService := application.NewComposeService(store, transport)
	tagService := application.NewTagService(store)
	bodyService := application.NewMessageBodyService(store, store, source)
	actionService := application.NewMessageActionService(store, store, source)

	app := NewApp(store.Close, accountService, setupService, mailboxService, syncService, composeService, tagService, bodyService, actionService)

	err = wails.Run(&options.App{
		Title:            appName + " " + version(),
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
