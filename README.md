# <img width="64" height="64" alt="pigeonpost" src="https://github.com/user-attachments/assets/fcc90cad-786e-4d04-a7a9-6f5d82be309d" /> PigeonPost

A cross-platform desktop email, calendar and address book client. Go core, React front end,
local-first. Built as a calmer, more predictable alternative to Thunderbird.

![In active development](https://img.shields.io/badge/status-active%20development-blue)

## Who it is for

- People who run standard IMAP/POP3 mailboxes (self-hosted, ISP, Fastmail, corporate) and want a
  fast, native, local-first desktop client.
- Outlook.com / Hotmail users, through an app password over IMAP (the Outlook preset fills in the
  servers). One-click Microsoft sign-in is deferred, and Office 365 work/school accounts are not yet
  supported.

## Who it is not for

- Webmail-only users who never want a desktop install.
- Gmail-first users: Gmail is out of scope for v1 (Google's restricted-scope verification is too much
  friction). A Gmail app password will connect through the generic IMAP path, but Gmail is not tested
  or supported.

## Capabilities

Shipped:

- **Accounts**: add, edit and remove IMAP and POP3 accounts from a two-step setup wizard (provider
  presets for Outlook, iCloud, Yahoo, Fastmail and StartMail, plus manual host/port/security).
  Credentials are verified against the server before anything is saved. Each account keeps its own
  separate inbox; there is no unified inbox. POP3 accounts download into a single mailbox with read and
  star marks kept locally, and their folder, move/copy and draft actions are hidden.
- **Sync and read**: folders and message summaries pulled into a local SQLite cache and read offline;
  full message bodies fetched on open and cached. HTML mail is sanitised, and remote images are blocked
  by default with a per-message "Load images" toggle.
- **Compose**: TipTap rich-text composer (bold, italic, lists, quote, links), sent as
  `multipart/alternative`. To, Cc and Bcc. Reply, reply-all and forward. Attach files from disk or an
  existing email (as a `message/rfc822` part), up to a 25 MB total. Save a draft to the server's Drafts
  mailbox.
- **Offline**: sends and drafts made while disconnected are queued and delivered automatically on the
  next sync (attachments included). The outbox is a per-account folder: select it to review what is
  waiting and cancel any queued message before it sends.
- **Organise**: mark read/unread (unread shows bold), star/flag, delete (to Trash) or delete
  permanently, move and copy between folders (by menu or by dragging a message onto a folder) and
  colour-coded tags. Create, rename and delete folders
  (rename is correct on non-`/` delimiter servers). Well-known folders sort to the top of a nested,
  collapsible tree, with unread-count badges per folder, account and total. Filter rules mark-read or
  flag messages on arrival, matching From, To, Cc or Subject with contains, is, starts-with, ends-with
  or does-not-contain. Instant local full-text search.
- **Read**: an optional reading pane (opening a message marks it read on view), a right-click context
  menu on every message (open in new tab, reply, forward, save as `.eml`, print, attach to a new
  message, a Mark submenu for read/unread/star/tag, move, copy, delete), and in-app reader tabs. Full
  keyboard control: arrows move within the message and folder lists, an explicit focus ring steps the
  whole window with Tab or Left/Right, Delete sends to Trash and Shift+Delete purges.
- **Trust**: dark theme by default with a light mode toggle; passwords held in the OS keychain, never
  in the database; external links open in your browser, not the app's webview; the unread total shows
  as a taskbar overlay badge on Windows.
- **Calendar**: month, week and day views. Week and day are an hour time-grid with an all-day strip;
  clashing events sit in side-by-side lanes. ICS (.ics) import and export (RFC 5545) round-trips with
  Outlook and Thunderbird; an event keeps its ICS UID so an export re-imports cleanly.
- **Contacts**: an address book with a list and an editor. vCard (.vcf) and CSV import and export, so
  contacts round-trip with Outlook (whose bulk export is CSV) and Thunderbird.
- **Help menu**: About (with credits), Licence and Check for Updates.

Planned (see [DESIGN_PLAN.md](DESIGN_PLAN.md) for the full roadmap):

- Tags round-tripped onto IMAP keywords, move/delete filter rules.
- Microsoft one-click sign-in (deferred: it needs an Azure/Entra tenant). Multiple accounts already
  work, each with its own inbox.
- Colour-per-calendar and contact-group management UIs; calendar recurrence-edit, invites and alarms.
- Cross-platform delivery (macOS and Linux) and two-way CalDAV / CardDAV.

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

- [DEVELOPMENT-README.md](DEVELOPMENT-README.md): prerequisites, running, building and packaging.
- [ARCHITECTURE.md](ARCHITECTURE.md): the clean-architecture invariants and how they are enforced.
- [TESTING.md](TESTING.md): the test strategy, the coverage gate and how to run everything.
- [DESIGN_PLAN.md](DESIGN_PLAN.md): the product design and phased roadmap.

## Quick start

```
wails dev        # run the app in development
go test ./...    # run the test suite
./build.ps1      # build the app exe and the installer (Windows)
```

## Licence

GPL-3.0. See [LICENSE](LICENSE). The full text is also available in the app under Help > Licence.
