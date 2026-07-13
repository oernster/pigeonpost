# PigeonPost Development Guide

How to set up, run, test, build and package PigeonPost from source.

## Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Go | 1.23 or newer | The build was verified on Go 1.26. |
| Node.js | 20 or newer | Node 24 verified. Ships with npm. |
| Wails CLI | v2.12 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| WebView2 runtime | current | Pre-installed on Windows 11. Wails uses the system WebView. |
| PowerShell | 7+ | For `build.ps1` and `test.ps1` on Windows. |

Platform build dependencies (C toolchains, gcc/WebKit on Linux, Xcode tools on macOS) are described by
`wails doctor`. Run it once after installing the CLI:

```
wails doctor
```

Note: the Go backend uses no CGO (pure-Go SQLite via modernc.org/sqlite), so the app itself builds
without a C compiler. WebView bindings are provided by the OS.

## First run

```
git clone https://github.com/oernster/pigeonpost
cd pigeonpost
wails dev
```

`wails dev` installs the front-end dependencies, builds the React app, generates the JavaScript
bindings from the Go facade, and launches the app with hot reload.

The app stores its data in a per-user directory:

- Windows: `%APPDATA%\PigeonPost\pigeonpost.db`
- macOS: `~/Library/Application Support/PigeonPost/pigeonpost.db`
- Linux: `~/.config/PigeonPost/pigeonpost.db`

Passwords are never stored there; they live in the OS keychain.

## Project layout

```
main.go + app.go + the feature bindings (send, draftrecovery, export, outbox, rulesapi, tagsapi, templatesapi, calendarapi, contactsapi, schedulingapi) + mailnotifier.go + alarmscheduler.go + dto.go + clock.go   composition root + Wails facade (package main)
internal/domain/            pure value objects, no IO (100% test gate)
internal/application/        use cases + port interfaces (100% test gate)
internal/infrastructure/
    storage/                SQLite store (schema v40, migrations, outbox, rules, contacts, calendar, reminders, meeting scheduling, draft recovery, cached message attachments, CalDAV accounts, two-way calendar sync)
    imap/                   emersion go-imap source adapter (sync, bodies, draft append, IDLE watcher)
    pop3/                   hand-rolled POP3 client (download-to-inbox, local flags)
    smtp/                   emersion go-smtp transport
    mailrouter/             dispatches reads, verification and actions by account protocol
    mailparse/              shared message-body parsing (MIME to plain-text and HTML, invite extraction)
    message/                shared RFC 5322 MIME builder (used by smtp and imap)
    ics/                    emersion go-ical calendar codec (RFC 5545 round-trip, recurrence, iTIP scheduling)
    recurrence/             RRULE expansion over teambition/rrule-go
    vcard/                  emersion go-vcard contacts codec
    csv/                    Outlook CSV contacts codec
    keychain/               OS keychain vault
    taskbar/                Windows taskbar unread badge, tray icon and desktop notifications (no-op stub elsewhere)
    installer/              install logic used by the setup program
installer/                  bespoke per-user setup program (Wails app: install/repair/upgrade/uninstall)
tools/genicons/             icon generator (master PNG -> ico + png set)
tests/structural/           architecture-enforcement tests
frontend/                   React + TypeScript (Vite)
docs/                       GitHub Pages landing site
```

## Common tasks

Run the app in development:

```
wails dev
```

Regenerate the front-end bindings after changing an `App` method:

```
wails generate module
```

Regenerate the icon set after changing `pigeonpost.png`:

```
go run ./tools/genicons
```

Run the tests (see [TESTING.md](TESTING.md) for detail):

```
go test ./...                    # Go suite
./test.ps1                       # Go suite with the coverage gate
cd frontend && npx vitest run    # front-end suite (Vitest + jsdom)
```

## Building

Build just the application executable:

```
wails build
# or
./build.ps1 -SkipInstaller
```

Output: `build/bin/PigeonPost.exe`.

Build the application and the bespoke installer:

```
./build.ps1
```

`build.ps1` runs in order: generate icons, `wails build` (the app), zip the built app as the
installer payload, then `wails build` the installer under `installer/`, which embeds that payload. The
installer is a Wails app so it shares the application's WebView and dark theme, and it supports
install, repair, upgrade and uninstall, plus a launch-on-boot option.

Outputs:

- `build/bin/PigeonPost.exe`: the application.
- `dist-installer/PigeonPostSetup.exe`: the per-user setup program that embeds the app, installs to
  `%LOCALAPPDATA%\Programs\PigeonPost`, writes the uninstall registry entry and creates shortcuts.

## Versioning

The single source of truth for the version is the `VERSION` file at the repo root. The runtime reads
it (embedded via `go:embed`), and the build stamps it into the installer. Do not hardcode a version
anywhere else.

## Cross-platform packaging

The Windows path (Nuitka-free: `wails build` plus the Go installer) is implemented. macOS (`.dmg`,
codesign, notarize) and Linux (AppImage / `.deb` / Flatpak) packaging are planned; the Go core and the
React front end are already portable.
