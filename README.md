# <img width="64" height="64" alt="pigeonpost" src="https://github.com/user-attachments/assets/fcc90cad-786e-4d04-a7a9-6f5d82be309d" /> PigeonPost

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
  `multipart/alternative`. To, Cc and Bcc. Reply, reply-all and forward. Attach files from disk or an
  existing email (as a `message/rfc822` part), up to a 25 MB total. Save a draft to the server's Drafts
  mailbox.
- **Offline** — sends and drafts made while disconnected are queued and delivered automatically on the
  next sync (attachments included). A title-bar pill shows what is waiting; click it to review the
  outbox and cancel any queued message.
- **Organise** — mark read/unread (unread shows bold), star/flag, delete (to Trash) or delete
  permanently, move and copy between folders, and colour-coded tags. Create, rename and delete folders
  (rename is correct on non-`/` delimiter servers). Well-known folders sort to the top of a nested,
  collapsible tree. Filter rules mark-read or flag messages on arrival. Instant local full-text search.
- **Read** — a right-click context menu on every message (open in new tab, reply, forward, save as
  `.eml`, print, attach to a new message, tag, move, copy, delete). Open messages in in-app reader
  tabs. Full keyboard control of the message list (arrows to move, Delete to Trash, Shift+Delete to
  purge).
- **Trust** — dark theme by default with a light mode toggle; passwords held in the OS keychain, never
  in the database; external links open in your browser, not the app's webview.
- **Help menu** — About (with credits), Licence, and Check for Updates.

Planned (see [DESIGN_PLAN.md](DESIGN_PLAN.md) for the full roadmap):

- Drag-and-drop to move messages, tags round-tripped onto IMAP keywords, move/delete filter rules.
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
