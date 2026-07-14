# <img width="64" height="64" alt="pigeonpost" src="https://github.com/user-attachments/assets/fcc90cad-786e-4d04-a7a9-6f5d82be309d" /> PigeonPost

A cross-platform desktop email, calendar and address book client. Go core, React front end,
local-first. A calmer, more predictable alternative to Thunderbird.

![In active development](https://img.shields.io/badge/status-active%20development-blue)

## Who it is for

- IMAP/POP3 mailbox users (self-hosted, ISP, Fastmail, corporate) who want a fast, native,
  local-first desktop client.
- Gmail, iCloud, Yahoo, Zoho, Fastmail and StartMail users who connect with an app password (the
  setup wizard fills in the servers).
- Microsoft users (Outlook.com, Hotmail, Live, Microsoft 365) who sign in through Microsoft OAuth in
  the browser; the refresh token is kept in the OS keychain.

## Who it is not for

- Webmail-only users who never want a desktop install.
- Google Workspace (work/school) accounts: OAuth-only, so an app password will not work. Personal
  Gmail works via an app password; only the one-click "Sign in with Google" is declined, because its
  full-mail scope carries a paid annual assessment.

## Capabilities

- **Accounts**: IMAP and POP3 from a two-step wizard (presets for Gmail, iCloud, Yahoo, Zoho, Fastmail
  and StartMail, plus a manual host/port/security path), credentials verified before saving. A preset
  whose provider needs an app password says so plainly (a normal login password will not work) and links
  to the page that creates one. Microsoft accounts
  via one-click OAuth. Each account keeps its own inbox, with an optional unified mailbox (a View tick)
  that merges every inbox into one list, each row dotted with its account's colour; replies from it send
  from the row's own account. Send-as addresses and drag or button reordering. POP3 downloads into one
  local mailbox with read and star marks kept locally.
- **Mail**: folders and summaries cached to local SQLite and read offline; bodies fetched on open and
  cached. HTML renders faithfully in a sandboxed frame that keeps the sender's own styles while running
  no scripts and making no remote requests; remote images are blocked by default until you load them
  (per message or a "Load images by default" toggle). In the dark theme a message that ships its own dark
  mode is shown in that native design, while a light-designed message is inverted to read comfortably, its
  photos and logos keeping their true colour. Attachments save from the reader (cached for offline). Sync runs per account with its own
  progress cue.
- **Compose**: TipTap rich text, To/Cc/Bcc, reply, reply-all and forward, file or message attachments
  (25 MB), reusable message templates, per-account signatures and server Drafts. In-progress writing autosaves locally and is
  offered back after a crash. Undo send holds each message for a few configurable seconds with an Undo
  that reopens it exactly as written. Send later schedules a message for a chosen moment (presets or an
  exact date and time); it waits in the Outbox with Cancel send and leaves while the app is running, or
  at the next launch after the chosen time. Sends made offline queue in a per-account outbox and deliver on the next sync.
- **Organise**: mark read/star, delete to Trash or purge, junk, instant offline full-text search with
  operators (from:, to:, has:attachment, dates and more) and colour tags
  that sync across devices as IMAP keywords. Snooze hides a message until a chosen time then returns it
  untouched with a desktop notification (while the app is running, or at the next launch); hidden
  messages wait in a Snoozed view with their due times and an Unsnooze. Move or copy messages by menu or by dragging onto a folder
  (dragging onto a collapsed folder springs it open). Create, rename and delete folders; reorganise the
  tree by dragging a folder to nest it, move it out or reorder its siblings.
  One folder each holds Inbox, Sent, Drafts, Trash, Junk and Archive, leading a collapsible tree with
  unread badges per folder, account and total. On-arrival rules mark or flag by From, To, Cc or Subject.
- **Read**: an optional reading pane (mark-on-view, F8 toggle), a right-click context menu, in-app
  reader tabs (double-click or Enter opens a message full-width, Back returns to the list), mouse and
  keyboard multi-select with bulk actions, plus full keyboard control through an explicit focus ring. The list stays fluid in folders of tens of thousands of messages. A Date sort and
  an optional threaded conversation view. A `.eml` file opens in an in-app viewer; on Windows PigeonPost
  can be set as the default `.eml` handler. Print a message through the system print dialog.
- **Notifications**: new mail raises a native desktop notification and updates a Windows taskbar badge.
  Each IMAP account is watched by a persistent IDLE connection with a 60-second poll backstop (and for
  POP3); an account's first sync is silent.
- **Calendar**: month, week and day views (a multi-day event is drawn as one bar across its days), recurring events with per-event time zones, nine
  emoji-labelled event categories, on-screen reminders and ICS import/export (RFC 5545) that round-trips
  with Outlook and Thunderbird. Meeting
  invites over iTIP/iMIP (accept, decline, cancel, reply) with clickable join links (Teams, Meet, Zoom,
  Webex). Early two-way CalDAV sync: a calendar-server account (app password) syncs events both ways,
  server-wins on conflict with the losing local edit kept as a copy.
- **Contacts**: an address book with postal addresses and birthdays, plus vCard (.vcf) and CSV
  import/export that round-trips with Outlook and Thunderbird.
- **Trust**: a dark theme with a light toggle, passwords held in the OS keychain (never the database)
  and external links opened in your browser.

Planned: move/delete rules, OS-delivered calendar alarms, macOS and Linux builds, two-way
CardDAV contact sync. The candidates parked beyond these are triaged with their rationale in
[FEATURES_PLAN.md](FEATURES_PLAN.md).

## Stack

| Concern | Choice |
|---|---|
| Shell | Wails v2 (Go + system WebView) |
| Backend | Go 1.23+ |
| Front end | React 18 + TypeScript (Vite) |
| Mail | emersion go-imap / go-smtp / go-message |
| Storage | modernc.org/sqlite (pure Go) + FTS5 |
| Credentials | OS keychain (zalando/go-keyring) |
| Installer | Wails app, same theme as the app |

## Documentation

- [DEVELOPMENT-README.md](DEVELOPMENT-README.md): prerequisites, running, building and packaging.
- [ARCHITECTURE.md](ARCHITECTURE.md): the clean-architecture invariants and how they are enforced.
- [TESTING.md](TESTING.md): the test strategy, the coverage gate and how to run everything.
- [TECH_DEBT.md](TECH_DEBT.md): the standing technical-debt reference.
- [FEATURES_PLAN.md](FEATURES_PLAN.md): the triaged feature backlog (parked candidates and confirmed
  won't-dos, each with its rationale).

## Quick start

```
wails dev        # run the app in development
go test ./...    # run the Go test suite
cd frontend && npx vitest run   # run the front-end test suite
./build.ps1      # build the app exe and the installer (Windows)
```

## Licence

GPL-3.0. See [LICENSE](LICENSE). The full text is also available in the app under Help > Licence.
