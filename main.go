package main

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/infrastructure/caldav"
	"github.com/oernster/pigeonpost/internal/infrastructure/ics"
	"github.com/oernster/pigeonpost/internal/infrastructure/imap"
	"github.com/oernster/pigeonpost/internal/infrastructure/keychain"
	"github.com/oernster/pigeonpost/internal/infrastructure/mailrouter"
	"github.com/oernster/pigeonpost/internal/infrastructure/oauth"
	"github.com/oernster/pigeonpost/internal/infrastructure/pop3"
	"github.com/oernster/pigeonpost/internal/infrastructure/recurrence"
	"github.com/oernster/pigeonpost/internal/infrastructure/remoteimage"
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

//go:embed pigeonpost.png
var appIconPNG []byte

const (
	appName     = "PigeonPost"
	dataDirName = "PigeonPost"
	dbFileName  = "pigeonpost.db"
	windowW     = 1200
	windowH     = 800
	// oauthHTTPTimeout bounds each OAuth token endpoint request so a stalled network never hangs a
	// sign-in or a silent token refresh indefinitely.
	oauthHTTPTimeout = 30 * time.Second
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
	// OAuth accounts (Microsoft) present a bearer token rather than a password. The token manager reads
	// the stored token from the keychain and refreshes it silently when it has expired, so the IMAP and
	// SMTP adapters get a live access token without the read paths knowing about OAuth. The authorizer
	// runs the interactive browser sign-in; app is assigned below, before any sign-in can be triggered.
	var app *App
	oauthClient := &http.Client{Timeout: oauthHTTPTimeout}
	tokenManager := oauth.NewTokenManager(vault, oauth.MicrosoftConfig(), oauthClient, clock)
	authorizer := oauth.NewAuthorizer(oauth.MicrosoftConfig(), oauthClient, func(url string) error {
		runtime.BrowserOpenURL(app.ctx, url)
		return nil
	}, clock)
	// The read paths (sync, body) and account verification are routed by protocol; the write paths
	// (draft append, message and folder actions) remain IMAP-specific, as POP3 has no server-side
	// mailbox actions.
	imapSource := imap.NewSource(vault, tokenManager, clock, newMessageID)
	pop3Source := pop3.NewSource(vault)
	mailSource := mailrouter.NewRouter(imapSource, pop3Source)
	transport := smtp.NewTransport(vault, tokenManager, clock, newMessageID)

	accountService := application.NewAccountService(store, vault, store)
	setupService := application.NewAccountSetupService(store, vault, mailSource)
	// The Microsoft account is IMAP, so the router verifies it through the XOAUTH2-aware IMAP adapter.
	microsoftSetupService := application.NewMicrosoftSetupService(store, vault, mailSource, authorizer, buildMicrosoftAccount)
	// Search date operators (before:/after:/on:) are read in the user's local calendar.
	mailboxService := application.NewMailboxService(store, time.Local)
	// The unified mailbox reads the same local cache, merged across every account's inbox.
	unifiedService := application.NewUnifiedMailboxService(store, store)
	// The tag-sync service rounds user tags onto the server as IMAP keywords; the sync service drives its
	// flush and reconcile, so it is constructed first and injected into the sync.
	tagSyncService := application.NewTagSyncService(store, store, store, mailSource)
	syncService := application.NewSyncService(store, store, mailSource, store, tagSyncService)
	composeService := application.NewComposeService(store, store, transport, imapSource, imapSource, store, store, clock, newOutboxID)
	tagService := application.NewTagService(store)
	bodyService := application.NewMessageBodyService(store, store, mailSource)
	// The resolver fetches a message's blocked remote images server-side (hardened against SSRF) and inlines
	// them as data: URIs, so the reader can show images a browser cannot load cross-origin.
	remoteImageService := application.NewRemoteImageService(remoteimage.NewResolver())
	actionService := application.NewMessageActionService(store, store, mailSource)
	folderService := application.NewFolderService(store, store, imapSource, imapSource)
	ruleService := application.NewRuleService(store, newRuleID)
	templateService := application.NewTemplateService(store, newTemplateID)
	contactService := application.NewContactService(store, newContactID)
	calendarService := application.NewCalendarService(store, newCalendarID, recurrence.New())
	// CalendarEditService is the remote-aware editing wrapper: creating, editing or deleting an event in a
	// CalDAV-mirrored calendar records the pending write intent a later sync pushes, while an event in a local
	// calendar stays purely local. The Wails calendar API routes SaveEvent and DeleteEvent through it.
	calendarEditService := application.NewCalendarEditService(calendarService, store, newCalendarID)
	// The CalDAV service is the account-aware two-way sync orchestrator. The store implements both the account
	// store and the calendar sync store; the keychain vault holds each account's password; one caldav factory
	// builds both the per-account read source and the write client; the ICS codec is the same one
	// CalendarService uses; newCalendarID supplies the reconcile's safety-copy ids.
	davFactory := caldav.NewFactory()
	caldavService := application.NewCalDAVService(store, vault, davFactory, davFactory, ics.New(), store, newCalendarID)
	// The scheduling service reads incoming meeting invites and replies (the ICS codec also implements
	// the iTIP SchedulingCodec), saves accepted meetings to the calendar store and sends replies,
	// requests and cancellations through the same SMTP transport as ordinary mail.
	schedulingService := application.NewSchedulingService(ics.New(), store, store, store, transport)

	// The taskbar overlay badge reflects the total unread count, and the flasher flashes the taskbar
	// button when a reminder fires while the window is in the background. Both locate the main window by
	// its title, so they are given the same title the Wails window uses below.
	windowTitle := appName + " " + version()
	overlay := taskbar.NewOverlay(windowTitle)
	overlay.Start()
	flasher := taskbar.NewFlasher(windowTitle)
	// The tray icon is persistent and clickable; its context menu emits Wails events the front end maps
	// to the Help dialogs, so the App facade supplies the callbacks at startup once the runtime exists.
	// The embedded app icon is composited with the unread badge to form the tray icon.
	tray := taskbar.NewTray(windowTitle, appName, appIconPNG)
	// The IDLE watcher holds a live connection per IMAP account so the server pushes new mail instantly,
	// rather than waiting for the poll; it authenticates through the same keychain vault as fetches.
	watcher := imap.NewWatcher(vault, tokenManager)

	app = NewApp(store.Close, overlay, flasher, tray, watcher, accountService, setupService, microsoftSetupService, mailboxService, unifiedService, syncService, composeService, tagService, tagSyncService, bodyService, actionService, folderService, ruleService, templateService, contactService, calendarService, calendarEditService, schedulingService, remoteImageService, caldavService)
	app.title = windowTitle

	err = wails.Run(&options.App{
		Title:            windowTitle,
		Width:            windowW,
		Height:           windowH,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 22, G: 27, B: 34, A: 1},
		// Only one PigeonPost runs per user; a second launch reveals the existing instance (which may be
		// hidden in the tray) rather than opening a new window.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               singleInstanceID,
			OnSecondInstanceLaunch: app.onSecondInstance,
		},
		// Clicking the window's close button asks whether to minimise to the tray or quit, so the reminder
		// scheduler and mail sync can keep running in the background; see App.beforeClose.
		OnBeforeClose: app.beforeClose,
		OnStartup:     app.startup,
		OnDomReady:    app.domReady,
		OnShutdown:    app.shutdown,
		Bind:          []interface{}{app},
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
