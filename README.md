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

Shipped:

- **Accounts** — add, edit and remove IMAP accounts from a two-step setup wizard (provider presets for
  Outlook, iCloud, Yahoo, Fastmail and StartMail, plus manual host/port/security). Credentials are
  verified against the server before anything is saved.
- **Sync and read** — folders and message summaries pulled into a local SQLite cache and read offline;
  full message bodies fetched on open and cached. HTML mail is sanitised, and remote images are blocked
  by default with a per-message "Load images" toggle.
- **Compose** — TipTap rich-text composer (bold, italic, lists, quote, links), sent as
  `multipart/alternative`. Reply, reply-all and forward. Save a draft to the server's Drafts mailbox.
- **Offline** — sends and drafts made while disconnected are queued and delivered automatically on the
  next sync, with a header indicator for what is waiting.
- **Organise** — mark read/unread (unread shows bold), star/flag, delete (to Trash), move between
  folders, and colour-coded tags. Instant local full-text search.
- **Trust** — dark theme by default with a light mode toggle; passwords held in the OS keychain, never
  in the database; external links open in your browser, not the app's webview.
- **Help menu** — About (with credits), Licence, and Check for Updates.

Planned (see [DESIGN_PLAN.md](DESIGN_PLAN.md) for the full roadmap):

- Folder create/rename/delete, drag to move, filters and rules.
- POP3, Microsoft OAuth, multiple accounts with a unified inbox.
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
