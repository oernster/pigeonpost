# PigeonPost

A cross-platform desktop email, calendar and address book client. Go core, React front end,
local-first. Built as a calmer, more predictable alternative to Thunderbird.

![In active development](https://img.shields.io/badge/status-active%20development-blue)

## Who it is for

- People who run standard IMAP/POP3 mailboxes (self-hosted, ISP, Fastmail, corporate) and want a
  fast, native, local-first desktop client.
- Outlook365 / Outlook.com users (one-click OAuth planned).

## Who it is not for

- Webmail-only users who never want a desktop install.
- Gmail-first users: Gmail is out of scope for v1 (Google's restricted-scope verification is too much
  friction). A Gmail app password will connect through the generic IMAP path, but Gmail is not tested
  or supported.

## Capabilities

Shipped so far (phase 1 and phase 2):

- IMAP account sync (folders and message summaries), read into a local cache.
- Three-pane UI: folder tree, message list, message reader.
- Compose and send over SMTP, with plain-text and address validation.
- Mark messages read or unread (unread shows bold).
- Dark theme by default with a light mode toggle.
- Help menu: About (with credits), Licence, and Check for Updates.
- Passwords held in the OS keychain, never in the database.

Planned (see [DESIGN_PLAN.md](DESIGN_PLAN.md) for the full roadmap):

- Account setup wizard, POP3, Microsoft OAuth.
- Coloured tags, drag/move between folders, filters, search.
- Calendar with ICS import/export, address book with vCard.

## Stack

| Concern | Choice |
|---|---|
| Shell | Wails v2 (Go + system WebView) |
| Backend | Go 1.23+ |
| Front end | React 18 + TypeScript (Vite) |
| Mail | emersion go-imap / go-smtp / go-message |
| Storage | modernc.org/sqlite (pure Go) + FTS5 |
| Credentials | OS keychain (zalando/go-keyring) |
| Installer | Wails app (Go + WebView), same theme as the app |

## Documentation

- [DEVELOPMENT-README.md](DEVELOPMENT-README.md) — prerequisites, running, building and packaging.
- [ARCHITECTURE.md](ARCHITECTURE.md) — the clean-architecture invariants and how they are enforced.
- [TESTING.md](TESTING.md) — the test strategy, the coverage gate and how to run everything.
- [DESIGN_PLAN.md](DESIGN_PLAN.md) — the product design and phased roadmap.

## Quick start

```
wails dev        # run the app in development
go test ./...    # run the test suite
./build.ps1      # build the app exe and the installer (Windows)
```

## Licence

GPL-3.0. See [LICENSE](LICENSE). The full text is also available in the app under Help > Licence.
