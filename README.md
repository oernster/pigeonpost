# <img width="64" height="64" alt="pigeonpost" src="https://github.com/user-attachments/assets/fcc90cad-786e-4d04-a7a9-6f5d82be309d" /> PigeonPost

A cross-platform desktop email, calendar and address book client. Go core, React front end,
local-first. Built as a calmer, more predictable alternative to Thunderbird.

![In active development](https://img.shields.io/badge/status-active%20development-blue)

## Who it is for

- People who run standard IMAP/POP3 mailboxes (self-hosted, ISP, Fastmail, corporate) and want a
  fast, native, local-first desktop client.

## Who it is not for

- Webmail-only users who never want a desktop install.
- Microsoft users (Outlook.com, Hotmail, Live, Office 365): not supported. Microsoft disabled basic
  auth for personal accounts so only OAuth connects them; OAuth in turn needs an app registration that
  Microsoft gates behind a paid Azure sign-up, so PigeonPost does not offer a Microsoft option.
- Gmail-first users: Gmail is out of scope for v1 (Google's restricted-scope verification is too much
  friction). A Gmail app password will connect through the generic IMAP path, but Gmail is not tested
  or supported.

## Capabilities

Shipped:

- **Accounts**: add, edit and remove IMAP and POP3 accounts from a two-step setup wizard (provider
  presets for iCloud, Yahoo, Fastmail and StartMail, plus manual host/port/security).
  Credentials are verified against the server before anything is saved. Each account keeps its own
  separate inbox; there is no unified inbox. POP3 accounts download into a single mailbox with read and
  star marks kept locally, and their folder, move/copy and draft actions are hidden.
- **Sync and read**: folders and message summaries pulled into a local SQLite cache and read offline;
  full message bodies fetched on open and cached. HTML mail is sanitised, and remote images are blocked
  by default with a per-message "Load images" toggle. A message that carries attachments shows a paperclip
  in the list; opening it lists each attachment with its size and saves it to a location you choose (the
  bytes are cached with the body, so saving works offline after the first open). Opening a message also
  shows its From, To and Cc correspondents.
- **Compose**: TipTap rich-text composer (bold, italic, lists, quote, links), sent as
  `multipart/alternative`. To, Cc and Bcc. Reply, reply-all and forward. Attach files from disk or an
  existing email (as a `message/rfc822` part), up to a 25 MB total. Save a draft to the server's Drafts
  mailbox. Each account can carry its own rich-text signature, inserted into a new message and above the
  quoted text on a reply or forward. The message you are writing is also autosaved locally as you type and
  offered back for recovery after an accidental close or a crash; this recovery copy is local only and
  never sent to the server.
- **Offline**: sends and drafts made while disconnected are queued and delivered automatically on the
  next sync (attachments included). The outbox is a per-account folder: select it to review what is
  waiting and cancel any queued message before it sends.
- **Organise**: mark read/unread (unread shows bold), star/flag, delete (to Trash) or delete
  permanently, mark as junk (moves the message to the account's Junk folder), move and copy between
  folders (by menu or by dragging a message onto a folder) and
  colour-coded tags. Create, rename and delete folders
  (rename is correct on non-`/` delimiter servers). Well-known folders sort to the top of a nested,
  collapsible tree, with unread-count badges per folder, account and total. Filter rules mark-read or
  flag messages on arrival, matching From, To, Cc or Subject with contains, is, starts-with, ends-with
  or does-not-contain. Instant local full-text search.
- **Read**: an optional reading pane (opening a message marks it read on view, and F8 toggles the pane),
  a right-click context menu on every message (open in new tab, reply, forward, save as `.eml`, print,
  attach to a new message, a Mark submenu for read/unread/star/tag, move, copy, delete), and in-app
  reader tabs. Messages multi-select the standard way, by Ctrl or Shift clicking or from the keyboard
  with Shift+Arrow to extend a range and Ctrl+Arrow then Ctrl+Space to pick individual rows; a selection
  can then be deleted, marked, starred or moved in one action, with a bulk delete confirming the count.
  Full keyboard control: arrows move within the message and folder lists, an explicit focus ring steps
  the whole window with Tab or Left/Right, Delete sends to Trash and Shift+Delete purges. A Date header
  sorts the folder list newest-first or oldest-first (remembered across launches); an optional
  conversation view groups a folder's messages into threads by subject.
- **Notifications**: newly arrived mail raises a native desktop notification (a Windows toast, or the
  platform equivalent) naming the subject and sender, so you are alerted even when the window is hidden
  to the tray. Each IMAP account is watched by a persistent IDLE connection that pushes the instant mail
  arrives; a 60-second backstop poll covers a missed push and POP3, which has no IDLE. An account's first
  sync is silent, so it never notifies for an existing backlog.
- **Trust**: dark theme by default with a light mode toggle; passwords held in the OS keychain, never
  in the database; external links open in your browser, not the app's webview; the unread total shows
  as a taskbar overlay badge on Windows.
- **Calendar**: month, week and day views. Week and day are an hour time-grid with an all-day strip;
  clashing events sit in side-by-side lanes. Recurring events (daily, weekly on chosen weekdays, monthly
  or yearly, with an interval and an optional end) expand across every view, and an edit or delete of a
  recurring event asks whether it applies to this occurrence, this and following, or all. Each event
  carries its own time zone, so a recurring event keeps its local time across daylight-saving changes.
  Events carry reminders (a lead time before the start) that fire an on-screen banner while the app runs;
  clicking the banner opens the calendar on that event. Reminders round-trip as ICS alarms. ICS (.ics)
  import and export (RFC 5545, including RRULE, RDATE, EXDATE and RECURRENCE-ID) round-trips with Outlook
  and Thunderbird; an event keeps its ICS UID so an export re-imports cleanly. A description that arrives
  as HTML (as many Teams and Outlook invites send) is converted to readable text on import.
- **Meeting invites**: an event with attendees is a meeting. Sending an invitation emails an iTIP
  REQUEST (RFC 5546 over RFC 6047 iMIP) as a `text/calendar` part; a recipient opens the message and
  can Accept, Tentatively accept or Decline, which saves the meeting to their calendar and emails a
  REPLY back to the organizer. The organizer can send a cancellation (a CANCEL the recipient removes
  with one click), and an incoming reply folds each attendee's response into the stored meeting.
  Recurring meetings are carried as the series master plus its overrides. A join link in an invite
  (Microsoft Teams, Google Meet, Zoom or Webex) shows as a Join button in the event that opens your
  browser, and any other link in the description is clickable.
- **Contacts**: an address book with a list and an editor. vCard (.vcf) and CSV import and export, so
  contacts round-trip with Outlook (whose bulk export is CSV) and Thunderbird.
- **Help menu**: About (with credits), Licence and Check for Updates.

Planned (see [DESIGN_PLAN.md](DESIGN_PLAN.md) for the full roadmap):

- Tags round-tripped onto IMAP keywords, move/delete filter rules.
- Calendar alarms delivered as OS notifications (on-screen reminder banners already ship).
- Cross-platform delivery (macOS and Linux) and two-way CalDAV / CardDAV.

## Known limitations

- A reminder you set as the organizer of a meeting is advisory to attendees. Under iTIP (RFC 5546) a
  receiving client such as Thunderbird applies the recipient's own default reminder and ignores the
  organizer's alarm, so an attendee may not see the lead time you chose.
- POP3 accounts have no IMAP IDLE, so their new mail is found by the 60-second backstop poll rather than
  pushed the instant it arrives.
- The Join button reads the event location and description; the Teams `X-MICROSOFT-SKYPETEAMSMEETINGURL`
  property is folded into the description on import so it surfaces too. A provider that carries the
  join URL only in some other non-standard property PigeonPost does not parse yet shows no button.
- Opening a reminder for a recurring event opens the series, not the single occurrence.

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
