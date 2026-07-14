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
main.go + app.go + one binding file per feature surface (accounts, mail, folders, send, draft recovery, outbox, snooze, tags, rules, templates, calendar, CalDAV, contacts, scheduling, export, .eml files) + the background goroutines (mailnotifier.go, alarmscheduler.go, outboxdispatcher.go, the snooze scheduler) + dto.go + clock.go   composition root + Wails facade (package main)
internal/domain/            pure value objects, no IO (100% test gate)
internal/application/        use cases + port interfaces (100% test gate)
internal/infrastructure/
    storage/                SQLite store (migrations, outbox with undo-send and send-later holds, snooze state, rules, contacts, calendar, reminders, meeting scheduling, draft recovery, cached message attachments, CalDAV accounts, two-way calendar sync, full-text search index)
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
    caldav/                 two-way CalDAV calendar sync client
    oauth/                  Microsoft OAuth token flow (authorization code + PKCE, loopback redirect)
    remoteimage/            SSRF-guarded fetcher that inlines blocked remote images on request
    keychain/               OS keychain vault
    taskbar/                Windows taskbar unread badge, tray icon and desktop notifications (no-op stub elsewhere)
internal/installer/         install logic used by the setup program
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

Each platform has one entry-point script at the repo root; each produces that platform's
distributable.

### Windows

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

### macOS (Apple Silicon)

Prerequisites: an arm64 Mac with the Xcode command line tools, Go, Node and the Wails CLI (the
same table as above); Homebrew for the tools the script installs itself (`create-dmg`).

```
bash builddmg.sh
```

The script generates icons, builds the app with `wails build -platform darwin/arm64`, stamps the
bundle version from `VERSION`, codesigns the `.app` with the hardened runtime and wraps it in a
drag-to-Applications DMG. The signing identity comes from `DEVELOPER_ID_APPLICATION` (a default is
built in); notarization runs only when `APPLE_ID` and `APPLE_APP_PASSWORD` are set (with
`APPLE_TEAM_ID` overridable), otherwise it is skipped and the DMG is still usable locally.

Output: `dist-dmg/PigeonPost-<version>-macos-arm64.dmg`.

### Linux (Flatpak; verified target Ubuntu)

Prerequisites: only `flatpak` and `flatpak-builder` (the script installs them through apt, dnf,
pacman or zypper if missing). Go, Node and WebKit all come from the flatpak SDKs, so nothing else
is needed on the host.

```
bash build_flatpak.sh
```

The script adds flathub, installs the GNOME runtime (which carries the webkit2gtk-4.1 that Wails
renders through) plus the golang and node SDK extensions, generates the desktop file, metainfo and
manifest, then builds the front end and the Go binary inside the sandbox with
`-tags desktop,production,webkit2_41`. It installs the app for the current user and exports a
distributable bundle.

Outputs: a user install (`flatpak run uk.codecrafter.PigeonPost`) and `pigeonpost.flatpak`.

```
bash clean_flatpak.sh
```

removes the user install and every flatpak build artefact, touching nothing the Windows or macOS
builds produced.

For a plain `wails dev` or `wails build` on a Linux host instead of the flatpak, install the
platform packages `wails doctor` lists (gcc, gtk3 and webkit2gtk development headers); on a distro
that ships only webkit2gtk-4.1 (Ubuntu 24.04 and later), add `-tags webkit2_41`.

## Versioning

The single source of truth for the version is the `VERSION` file at the repo root. The runtime reads
it (embedded via `go:embed`), and the build stamps it into the installer. Do not hardcode a version
anywhere else.
