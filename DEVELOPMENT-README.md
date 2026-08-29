# PigeonPost Development Guide

How to set up, run, test, build and package PigeonPost from source.

## Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Go | 1.25 or newer | The floor is the `go` directive in `go.mod`; the build was verified on Go 1.26. |
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
bindings from the Go facade and launches the app with hot reload.

The app stores its data in a per-user directory:

- Windows: `%APPDATA%\PigeonPost\pigeonpost.db`
- macOS: `~/Library/Application Support/PigeonPost/pigeonpost.db`
- Linux: `~/.config/PigeonPost/pigeonpost.db`

Passwords are never stored there; they live in the OS keychain.

## Project layout

```
main.go + app.go + one binding file per feature surface (accounts, mail, folders, send, draft recovery, outbox, snooze, tags, rules, templates, calendar, CalDAV, contacts, scheduling, export, .eml files, updates) + the background goroutines (mailnotifier.go, alarmscheduler.go, outboxdispatcher.go, the snooze scheduler) + dto.go + clock.go   composition root + Wails facade (package main)
internal/domain/            pure value objects, no IO (100% test gate)
internal/application/        use cases + port interfaces (100% test gate)
internal/infrastructure/
    storage/                SQLite store (migrations, outbox with undo-send and send-later holds, snooze state, rules, contacts, calendar, reminders, meeting scheduling, draft recovery, cached message attachments, CalDAV accounts, two-way calendar sync, folder display state, full-text search index)
    imap/                   emersion go-imap source adapter (sync, bodies, draft append, IDLE watcher)
    pop3/                   hand-rolled POP3 client (download-to-inbox, local flags)
    smtp/                   emersion go-smtp transport
    mailrouter/             dispatches reads, verification and actions by account protocol
    mailparse/              shared message-body parsing (MIME to plain-text and HTML, invite extraction, outgoing embedded-image extraction)
    message/                shared RFC 5322 MIME builder (used by smtp and imap)
    ics/                    emersion go-ical calendar codec (RFC 5545 round-trip, recurrence, iTIP scheduling)
    recurrence/             RRULE expansion over teambition/rrule-go
    vcard/                  emersion go-vcard contacts codec
    csv/                    Outlook CSV contacts codec
    caldav/                 two-way CalDAV calendar sync client
    oauth/                  Microsoft OAuth token flow (authorization code + PKCE, loopback redirect)
    remoteimage/            SSRF-guarded fetcher that inlines blocked remote images and CSS backgrounds on request
    update/                 GitHub latest-release source for the update check
    keychain/               OS keychain vault
    taskbar/                Windows taskbar unread badge, tray icon and desktop notifications (no-op stub elsewhere)
    sound/                  synthesised notification chime, played through winmm on Windows (no-op stub elsewhere)
internal/installer/         install logic used by the setup program
installer/                  bespoke per-user setup program (Wails app: install/repair/upgrade/uninstall)
assets/                     masters for the title-bar and folder-list glyphs, one PNG per glyph
tools/genicons/             image generator (the masters -> ico + png set, the donate artwork and the glyphs)
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

Regenerate the derived image assets after changing any master: `pigeonpost.png` (the app icon) or
`donate.png` (the donate button's artwork) at the repo root; or any PNG in `assets/` (the title-bar and
folder-list glyphs):

```
go run ./tools/genicons
```

It is idempotent: run on an unchanged master it rewrites the same bytes. `build.ps1`, `builddmg.sh` and
`build_flatpak.sh` each run it before they build, so a release never ships stale artwork.

Its output is committed, which is deliberate. Those three scripts are the only things that run it:
`wails.json` carries no hook, so `wails dev` and a bare `wails build` never generate anything. An asset
the front end imports therefore has to be in a fresh clone already; otherwise the build cannot resolve
it. A
regeneration that changes an asset shows up as an ordinary modification and is committed with the
master that caused it. The one output not committed is the flatpak hicolor set under `build/linux/`,
which nothing but `build_flatpak.sh` reads and which that script generates itself.

The front end imports the glyphs through `frontend/src/icons.ts`, which is the one home for the mapping
from a name to a picture.

A glyph master is cropped to its visible pixels, centred on a transparent square and scaled to one common
size, so every glyph carries the same visual weight whatever its own framing was. Dropping a new PNG into
`assets/` is enough to generate it; naming it in `icons.ts` is what puts it on screen. The masters are
held at 512px on their longest side, which leaves every one of them a downscale into the generated size
rather than an enlargement.

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
installer is a Wails app so it shares the application's WebView and dark theme; it supports
install, repair, upgrade and uninstall, plus a launch-on-boot option.

Outputs:

- `build/bin/PigeonPost.exe`: the application.
- `dist-installer/PigeonPostSetup.exe`: the per-user setup program that embeds the app, installs to
  `%LOCALAPPDATA%\Programs\PigeonPost`, writes the uninstall registry entry and creates shortcuts.

### macOS (Apple Silicon)

Prerequisites: an arm64 Mac with the Xcode command line tools, Go and Homebrew; the script
installs the Wails CLI, Node and `create-dmg` itself when they are missing.

```
bash builddmg.sh
```

The script generates icons, builds the app with `wails build -platform darwin/arm64`, stamps the
bundle version from `VERSION`, codesigns the `.app` with the hardened runtime, notarizes and
staples the `.app`, wraps it in a drag-to-Applications DMG, stamps the DMG's own Finder icon, signs
the DMG, then notarizes and staples that too, verifying the result with `stapler` and `spctl`. The
signing identity comes from `DEVELOPER_ID_APPLICATION` (a default is built in).

Notarization is mandatory: since macOS 10.15 Gatekeeper rejects a signed-but-unnotarized app on
every machine except the one that signed it, so the build stops before building anything unless
`APPLE_ID` and `APPLE_APP_PASSWORD` (an app-specific password, checked for shape up front) are both
set, with `APPLE_TEAM_ID` overridable. The notarization credential lives in the keychain under a
per-app profile (created once with `xcrun notarytool store-credentials`; `APPLE_KEYCHAIN_PROFILE`
overrides the name) and the password never reaches the logs. `ALLOW_UNNOTARIZED=1` builds without
notarizing for local testing only; a DMG built that way must never be released.

Output: `PigeonPost.dmg` in the repo root.

### Linux (Flatpak; verified target Ubuntu)

Prerequisites: only `flatpak` and `flatpak-builder` (the script installs them through apt, dnf,
pacman or zypper if missing). Go, Node and WebKit all come from the flatpak SDKs, so nothing else
is needed on the host.

```
bash build_flatpak.sh
```

The script adds flathub, installs the GNOME runtime (which carries the webkit2gtk-4.1 that Wails
renders through) plus the golang and node SDK extensions, generates the desktop file (which claims the
mailto scheme, so GNOME's Default Apps can select PigeonPost as the email client), metainfo and
manifest, then builds the front end and the Go binary inside the sandbox with
`-tags desktop,production,webkit2_41`. It installs the app for the current user and exports a
distributable bundle.

Outputs: a user install (`flatpak run uk.codecrafter.PigeonPost`) and `pigeonpost.flatpak`.

```
bash cleanup_flatpak.sh
```

removes the user install and every flatpak build artefact, touching nothing the Windows or macOS
builds produced.

For a plain `wails dev` or `wails build` on a Linux host instead of the flatpak, install the
platform packages `wails doctor` lists (gcc, gtk3 and webkit2gtk development headers); on a distro
that ships only webkit2gtk-4.1 (Ubuntu 24.04 and later), add `-tags webkit2_41`.

## Line endings

`.gitattributes` declares `* text=auto eol=lf`, so a checkout is LF on every platform regardless of the
machine's `core.autocrlf`. Nothing needs configuring per machine. Leaving it to `core.autocrlf` on a
Windows checkout had git reporting over a hundred Go files as modified with no content behind any of
them, which buries a real change among the noise; `git diff` was empty while `git status` was not.

One consequence on a checkout that predates the attribute: git does not rewrite files already on
disk, so some working copies still hold CRLF even though every blob and every fresh checkout is LF.
Git reports nothing modified (it normalises on the way in, which is the point) but `gofmt -l` lists
each such file, purely for its line endings and with no formatting difference behind it. Run
`git ls-files --eol` to see which files those are; re-checking them out is what converts them.

## Versioning

The single source of truth for the version is the `VERSION` file at the repo root. The runtime reads
it (embedded via `go:embed`); the Windows build stamps it into the installer, the macOS build stamps
it into the app bundle's Info.plist and the flatpak build stamps it into the metainfo release entry.
Do not hardcode a version anywhere else.

## Licence

GPL-3.0 (see [LICENSE](LICENSE)), with credit to the original author (Oliver Ernster) retained in
all copies and derivative works, under all circumstances. Removing or omitting this attribution is
not permitted. The requirement is stated in the LICENSE file's own licensing notice as a GPLv3
section 7(b) additional term and repeated in Help > About.
