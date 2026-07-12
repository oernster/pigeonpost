# PigeonPost: Design Plan

Cross-platform desktop email, calendar and address book client. Go core, React front end,
local-first. Delivered as a signed download at https://www.pigeonpost.ink.

Status: design locked; 0.14.0 in progress on main (tags sync onto IMAP keywords, faithful HTML email rendering and large-folder performance; VERSION 0.14.0, schema v38); 0.13.0 shipped (tagged v0.13.0, the front-end decomposition); 0.12.0 shipped (tagged v0.12.0, the backend cleanup); 0.11.0 released (tagged v0.11.0, schema v30); 0.10.0 released
(tagged v0.10.0, schema v30); 0.9.0 released (tagged v0.9.0, schema v27); 0.8.0 released (tagged v0.8.0, schema v23); 0.7.0 cut at schema v15;
0.6.0 released
(tagged v0.6.0). The 0.5.0 release
(folder create/rename/delete, message move/copy, on-arrival mark/flag rules, a right-click context
menu, in-app reader tabs, full keyboard navigation, Bcc, file and email attachments, save-as/print,
drag-to-move and an outbox view, on top of the 0.4.0 core of account wizard, sanitised-HTML reading with
remote-image blocking, rich-text compose, save-draft, the offline outbox, coloured tags, server-synced
actions and full-text search) is complete. 0.6.0 adds POP3 receive (each account keeping its own
separate inbox) plus a UI, keyboard and rules pass: filter-rule match operators (contains, is,
starts/ends with, does not contain) and To/Cc fields, a reading pane with mark-on-view, a Mark submenu,
per-folder/account/total unread badges, an explicit keyboard focus ring, the outbox surfaced as a
per-account folder, a red taskbar overlay badge, a regrouped title tray and a reader fix for HTML mail
that showed its text duplicated and oversized. 0.7.0 adds the address book (vCard and CSV) and the calendar (ICS, with month, week and
day views), both round-tripping with Outlook and Thunderbird; cross-platform delivery remains ahead for
1.0.0. 0.8.0 carries the calendar to full RFC 5545: recurring events (daily, weekly, monthly and yearly)
that expand across every view with this / this-and-following / all editing, per-event IANA time zones
with VTIMEZONE export, and reminders that fire on-screen banners and round-trip as ICS alarms. It adds
two-way meeting scheduling (iTIP/iMIP over RFC 5546/6047), instant new-mail notifications over IMAP IDLE
with a 60-second poll backstop, a clickable Windows tray icon with a minimise-to-tray close choice (and
native notifications off Windows), message multi-select by mouse and keyboard with bulk delete, mark,
star and move, an F8 reading-pane toggle, a clickable reminder banner that opens its event and clickable
meeting join links (Teams, Meet, Zoom and Webex) in the event editor.
0.9.0 adds per-account rich-text signatures, local draft-recovery autosave (the in-progress compose is
saved locally and offered back after a crash, never touching the server), received attachments (a
paperclip in the list plus view and save from the reader, cached for offline saving), conversation
grouping by subject with a toggle, junk marking to the Junk folder, a Date-sortable list and To/Cc
correspondents in the reader. It also lands an interop pass: subjects, display names and attachment
filenames are RFC 2047 encoded-word and HTML-entity decoded; the Teams
`X-MICROSOFT-SKYPETEAMSMEETINGURL` join link is parsed into the event on import; and imported ICS
descriptions that arrive as HTML are converted to readable text.
0.10.0 makes Microsoft accounts work: sign-in is OAuth (authorization-code plus PKCE over a loopback
redirect, XOAUTH2 mailbox access, a refresh token kept in the keychain and renewed silently); the Entra
registration it needs turned out to be free, so the 0.9.0-era removal is reversed. It adds Gmail and Zoho
provider presets (personal app-password accounts), send-as identities and a reworked account list (drag or
button reordering, a hover action toolbar, a left unread gutter and full-width name and email). Mail
operations are hardened for Gmail scale: bulk delete and move batch into one connection per folder so a
large selection stays under Gmail's connection cap, Ctrl+A selects all, the expunge runs in chunks and sent
messages are saved to the Sent folder. The reader gains attachment Open, Save and Save-all, inline cid:
images and an in-app .eml viewer that also backs a Windows .eml file association (PigeonPost registers as an
Open-with handler that can be set as the default). Compose accepts a semicolon as well as a comma between
addresses and offers a one-click fix when a wrong separator sits between valid ones. The largest strand is a
keyboard, menu and tray pass: full keyboard navigation with an explicit focus ring and menu accelerators,
the title tools grouped into File, Edit, View and Mail menus and a tray toolbar carrying Reply, Reply all,
Forward, Attach, Compose, Add account and Sync. Schema moved from v27 to v30.
0.11.0 is a folder-management pass. Sync becomes per-account: each account syncs on its own and shows a
Synchronising cue on its row, independent of the others. Folder classification is tightened so each
well-known role (Inbox, Sent, Drafts, Trash, Junk and Archive) is held by exactly one folder, the server's
own RFC 6154 special-use flags winning over a name match and a well-known name nested under another special
folder rejected; stray Sent folders are reconciled into a single top-level Sent at the start of every sync.
Custom folders can be reorganised by dragging: dropping one on another nests it as a child and dropping it
into the gap at a shallower level moves it up or out to the top level, both real server moves through a
path-to-path rename, while dropping it into the gap at its own level reorders it amongst its siblings as a
local per-account order (IMAP has no folder order of its own). Folder rename and delete stay, both
keyboard-reachable; the Delete key removes a focused custom folder through the same confirmation as its
delete button. Schema unchanged at v30.
0.12.0 is a behaviour-preserving cleanup with no feature, UI or schema change, paying down the backend
technical debt catalogued in TECH_DEBT.md. The near-duplicate blocks that were the dominant debt are
collapsed behind the coverage gate: shared message-context and folder-by-kind resolution in the
application layer, a generic row-iteration helper across the storage list methods, one outgoing-message
assembler, a value-copy idiom in the domain copy methods, a shared VCALENDAR-encode helper in the ICS
codec and shared credential selection across the IMAP fetch and IDLE paths. The magic numbers and
misplaced constants the house rules target are named or lifted: the vtimezone offsets, the SQLite
busy_timeout, the bare INBOX literal and server-side-Sent modelled as a domain method rather than a
hardcoded host. Dead IMAP folder classifiers are removed; host:port formatting moves to net.JoinHostPort
for correct IPv6 handling; organizer and attendee JSON encode errors propagate rather than dropping data;
and the densest functions (buildFolders, ParseBody, source.go) are decomposed into readable pieces.
staticcheck joins the standard verification and the whole Go module is clean; the 100% domain and
application coverage gate holds throughout. Schema unchanged at v30.
This document is the target design.
Author: Oliver Ernster. Licence: GPL-3.0.

---

## 1. Product scope and Thunderbird parity

Thunderbird is the source-material guide. PigeonPost implements the load-bearing workflows that
carry daily use and explicitly defers the long tail. Naming what will NOT be built is deliberate:
it stops scope re-litigation mid-build.

### In scope (parity targets)

- Accounts: IMAP and POP3 receive, SMTP send, multiple accounts, each with its own separate inbox
  (no unified inbox).
- Account setup: autoconfig wizard (Mozilla ISPDB-style autodiscovery from the email domain,
  RFC 6186 SRV records, provider well-known endpoints) with a manual host/port/security/auth fallback.
- Three-pane UI: folder tree, message list, message reader, plus a compose window.
- Folder operations: create, rename, delete and copy/move messages by menu, keyboard or drag to the
  same or an adjacent mailbox.
- Message quality-of-life: mark read/unread (unread shown bold), coloured flags/tags, star/important,
  junk mark, threading.
- Compose: plain text and light rich-text HTML, reply/reply-all/forward, attachments, draft autosave,
  per-account signatures.
- Search: fast local full-text search (SQLite FTS5) plus server-side IMAP SEARCH where available.
- Message filters/rules: on-arrival actions (move, tag, mark, delete).
- Address book: contacts with groups, vCard (.vcf) import/export, plus CSV import/export for Outlook
  (whose bulk contact export is CSV, not vCard). Import/export must round-trip with Outlook and
  Thunderbird.
- Calendar: month/week/day views, events with reminders, meeting invites (iTIP/iMIP), ICS (.ics)
  import/export, read-only remote ICS subscription.
- Offline: cached mail readable offline; actions queued and replayed on reconnect.

### Out of scope for v1 (named so they do not creep)

Chat/matrix/newsgroups/RSS, add-on/extension system, message-template gallery, full HTML-editor
parity, PGP/S-MIME (v2 candidate), server-side sieve editing, two-way CalDAV/CardDAV (v2),
Gmail-specific support (see section 7).

---

## 2. Technology stack

Go + Wails + React, chosen over Rust + Tauri because the Emersion Go mail suite covers the entire
email + calendar + contacts surface in one coherent family (including CalDAV/CardDAV via go-webdav),
it matches the proven native-Go-on-Wails delivery lineage (locus, focus-reader), and it avoids
learning async Rust under load on a protocol-heavy app. All chosen dependencies are permissive
(MIT/BSD) and GPL-3.0 compatible.

| Concern | Choice | Rationale |
|---|---|---|
| Shell | Wails v2 (WebView2/WebKit hosting React + TS) | Proven delivery lineage; single small binary. |
| Backend | Go 1.21+ | Second first-class language; native cross-platform. |
| IMAP | emersion/go-imap (v2, IDLE) | Async push, mature. |
| POP3 | small client (hand-rolled or minimal dep) | POP3 is a small protocol. |
| SMTP send | emersion/go-smtp | Pairs with the suite. |
| MIME parse/build | emersion/go-message | Production-tested in real clients. |
| SASL | emersion/go-sasl | SASL PLAIN for SMTP AUTH. |
| Calendar ICS | emersion/go-ical | RFC 5545 round-trip (Thunderbird and Outlook). |
| Contacts vCard | emersion/go-vcard | vCard 3/4 round-trip. |
| Contacts CSV | stdlib encoding/csv | Outlook bulk contact import/export (Outlook exports CSV, not vCard). |
| CalDAV / CardDAV (v2) | emersion/go-webdav | Two-way sync affordable in Go. |
| Storage | modernc.org/sqlite (pure Go, no CGO) + FTS5 | Local-first, single-writer/multi-reader. |
| Credentials | zalando/go-keyring | OS keychain; never in the DB. |
| Front end | React 18 + TypeScript (Vite) | Existing React/TS + Wails lineage. |
| List virtualization | @tanstack/react-virtual | 100k-message folders scroll smoothly. |
| Drag/drop | native HTML5 drag-and-drop | Message-to-folder, account reorder, folder reparent and reorder. |
| Rich-text compose | TipTap (ProseMirror) | Clean, sanitisable, email-safe HTML. |
| HTML mail render | sandboxed iframe + sanitiser | Untrusted HTML is the top security surface. |

---

## 3. Architecture

Invariant: `UI -> Application -> Domain <- Infrastructure`, enforced by structural tests, not
convention. Mirrors the house Go pattern exactly.

```
React / TypeScript  (three-pane UI, compose, calendar)         UI
    talks ONLY to Application via Wails-bound methods
                    | typed IPC (serde-style DTOs)
Application (use cases + ports/interfaces)                      APP
    SyncMailbox, SendMessage, MoveMessages, ImportIcs,
    SetupAccount, SearchMail ... depends on Domain + interfaces
      | implements                         | uses
Domain (pure)                       Infrastructure              DOMAIN / INFRA
  Message, Folder, Account,           ImapClient, Pop3Client,
  Flag, Contact, Event                SmtpSender, SqliteRepo,
  no IO, no wall-clock                IcsParser, Keychain, OAuth
```

### Layer rules

- Domain: pure Go, std only, no IO, no wall-clock reads (inject a Clock interface), no network types.
  Value objects validate on construction. Immutable by convention (With* copy methods).
- Application: depends on Domain and defines interfaces (ports): MailStore, MailTransport,
  CalendarStore, ContactStore, CredentialVault, Authenticator, Clock. Never imports Infrastructure.
- Infrastructure: implements the interfaces. Owns SQLite, IMAP/POP3/SMTP, ICS, vCard, keychain, OAuth.
  Never imported by Domain/Application.
- UI: React client of Application only, through Wails-bound methods mapping one-to-one to use cases.
  No business logic in React.

### Composition root

One explicit place (`cmd/pigeonpost/main.go`) wires concrete adapters into use cases via constructor
injection. No global singletons, no service locator. Pinned by a structural whitelist test.

### Layout

```
internal/
  domain/          # pure, std only
  application/     # use cases + interfaces
  infrastructure/  # imap, pop3, smtp, storage, ics, vcard, oauth, keychain
cmd/pigeonpost/
  main.go          # composition root + Wails bindings (thin)
tests/structural/
  boundary_test.go # layer direction + purity + LOC scan
frontend/          # React + TS (Vite)
```

Errors: `fmt.Errorf("...: %w", err)` with `errors.Is` sentinels; no custom error types beyond
sentinels.

---

## 4. Data model (SQLite, local-first)

Credentials are excluded from the DB and live in the OS keychain, referenced by an opaque key.
Schema versioning (`schema_version` row + migrations) from day one, because storage format is the
hardest thing to change later.

- account: id, display name, email, protocol (imap/pop3), server config, auth method
  (password/oauth), keychain_ref.
- folder: id, account_id, path, kind (inbox/sent/drafts/trash/junk/archive/custom), uid_validity,
  unread_count.
- message: id, folder_id, imap_uid, message_id, thread_id, from/to/cc, subject, date, size, flags
  bitfield, has_attachments, snippet, is_read, is_starred.
- message_body: message_id, plaintext, html, headers_blob (lazy-loaded).
- attachment: id, message_id, filename, mime, size, content_ref.
- tag: id, name, colour (the coloured-flags system).
- message_tag: message_id, tag_id.
- contact / contact_email / contact_group: address book, vCard-backed.
- calendar / event: event with RRULE, reminders, ICS UID for round-trip.
- filter_rule: condition + action set, evaluated on arrival.
- outbox_op: queued action (send/move/flag/delete) for offline replay with idempotency key.
- message_fts: FTS5 virtual table over subject/body/sender for instant local search.

---

## 5. Key subsystems

### Sync engine (the heart)

Per-account goroutine coordinated by context and channels (the locus model). IMAP path uses
UID-based incremental sync with UIDVALIDITY/UIDNEXT bookkeeping, IDLE for push where supported,
polling fallback otherwise. POP3 path is download-and-optionally-leave-on-server, deduped by
Message-ID. All mutations go through the outbox queue so the UI is optimistic and offline-tolerant:
a move marks locally, enqueues the IMAP MOVE and reconciles on reconnect. Idempotency keys make
replay safe.

### Account setup wizard

Two paths behind one use case: (1) autoconfig from the email domain (Mozilla ISPDB, RFC 6186 SRV,
provider well-known endpoints), present discovered settings for confirmation; (2) manual host, port,
security (SSL/TLS/STARTTLS), auth method. OAuth accounts branch into browser consent and store the
refresh token in the keychain.

### Move / copy / drag

Domain use cases `MoveMessages{message_ids, target_folder}` and `CopyMessages`. UI drag (native HTML5 drag-and-drop) and
context menu both call the same use case, so drag is only an input method. Same-account move maps to
IMAP MOVE; cross-account move is copy-to-target then delete-from-source through the outbox so a
mid-way failure is recoverable.

### Coloured flags / tags

Named tags each with a colour, many-to-many with messages, mapped onto IMAP keywords ($Label1 etc.)
so they round-trip with the server. Bold-unread is a pure UI concern driven by is_read.

### Calendar and address book

go-ical for ICS import/export and go-vcard for contacts. v1 supports file import/export plus read-only
remote ICS subscription. Events carry their original ICS UID so export round-trips cleanly. Two-way
CalDAV/CardDAV (go-webdav) is v2.

Outlook and Thunderbird interop is an explicit v1 requirement, not just a by-product of using standard
formats. Calendar uses ICS (RFC 5545), which both apps read and write. Contacts need two formats: vCard
(.vcf) for Thunderbird and single-contact Outlook, and CSV for Outlook's bulk contact export/import
(Outlook exports the address book as CSV, not vCard; Thunderbird reads CSV too), so a CSV mapper sits
alongside the vCard one behind the same import/export port. The acceptance test is a real export from
each app importing cleanly into PigeonPost, and a PigeonPost export importing back into both without
loss.

### Search

FTS5 for instant local search; optional server-side IMAP SEARCH for content not yet cached.
Debounced query use case returns ranked results.

### HTML mail security

Render in a sandboxed iframe, sanitise HTML, and block remote images/trackers by default with a
per-sender allow. This is the single most important security decision and is in the reader from v1.

---

## 6. Security

- Credentials and OAuth refresh tokens: OS keychain only, never in SQLite or config. The DB stores an
  opaque reference.
- Transport: TLS enforced, STARTTLS downgrade protection, certificate validation on by default.
- Untrusted content: sanitise HTML, sandbox rendering, block remote content and external protocol
  handlers by default.
- OAuth2/XOAUTH2 (PKCE, loopback redirect) for providers that require it.

---

## 7. Authentication decision (locked)

Auth method is a strategy behind an auth-method seam from day one (both password and the XOAUTH2 OAuth
path are implemented). v1 provider matrix:

- Microsoft (Outlook.com / Hotmail / Live / Microsoft 365): SUPPORTED through OAuth. Microsoft disabled
  basic auth for personal accounts (from Sept 2024) so a password or app password over IMAP fails with
  `NO AUTHENTICATE failed`; only OAuth (XOAUTH2) connects them. PigeonPost signs in with the
  authorization-code-plus-PKCE flow over a loopback redirect: it opens the system browser for consent,
  exchanges the code for tokens, verifies mailbox access with XOAUTH2 and keeps a refresh token in the
  OS keychain, renewing it silently. The Entra app registration this needs turned out to be free (no
  card charge on the free Azure tier), so it passes the provider-inclusion test. The public client id is
  embedded; a public client holds no secret.
- Generic IMAP/POP3 + SMTP (Fastmail, self-hosted, ISP, corporate): password auth, the core path,
  from phase 1.
- Gmail: SUPPORTED for personal accounts through an app password, added as a provider preset like
  iCloud and Yahoo. Personal Gmail still issues app passwords in 2026 with 2-Step Verification on; a
  plain IMAP preset costs the developer nothing, so it passes the provider-inclusion test. What stays
  OUT is one-click "Sign in with Google" (XOAUTH2): the restricted mail.google.com scope triggers an
  annual CASA security assessment that costs real money every year, which fails the provider-inclusion
  test. Google Workspace (work/school) accounts are OAuth-only since March 2025, so they are not covered
  by the personal-account preset.

Every supported account signs in either with a password (or app password) over the generic IMAP/POP3 +
SMTP path or with Microsoft OAuth over the same servers. The free Entra registration is the only
external dependency; it costs nothing to run.

---

## 8. Cross-platform packaging and delivery

- Windows: NSIS installer, code-signed, per-user install, themed to the app.
- macOS: .dmg, codesign with Developer ID, notarize, staple.
- Linux: AppImage + .deb (Flatpak optional).
- Build: `wails build` + `go build -ldflags="-s -w"` per platform.
- Version: a root VERSION file is the single source of truth; runtime reads it; static download pages
  at pigeonpost.ink are stamped from it, never hand-edited.
- Icon: one 1024x1024 master PNG (pigeonpost.png); a generator emits the platform set (.ico, .icns,
  hicolor PNGs); no default framework icon ships.
- Downloads page lists the three platform artifacts, checksums and the stamped version.

---

## 9. Testing strategy

- Domain: pure unit tests, 100% gate (correctness lives here).
- Application: use cases against hand-written fake adapters implementing the interfaces (no mock
  libraries).
- Infrastructure: integration tests against a real local SQLite tmpdir; IMAP/SMTP against a
  throwaway GreenMail/Dovecot container in CI.
- Structural: boundary direction, domain purity (no IO/time/network in domain), composition-root
  whitelist, module-size limit; `tests/structural/boundary_test.go` AST scan.
- Frontend: Vitest and jsdom component and hook tests, a coverage gate on the pure modules and a
  structural boundary test; characterization-first for the App.tsx and component decomposition. Avoid
  fragile pixel assertions.
- Read results by exit code, not console text.

---

## 10. Phased roadmap

Each phase ships something usable. Earlier phases de-risk the schema and sync engine (the
hardest-to-reverse parts) first.

1. Skeleton + read-only IMAP. Wails shell, clean-architecture packages, SQLite schema v1, single IMAP
   account via manual setup, three-pane read-only. Proves the sync engine and layering.
2. Send + compose + flags. SMTP send, compose (plain + TipTap rich-text), read/unread bold, coloured
   tags, star/junk.
3. Move/copy/drag + folders + offline outbox. Full folder ops, native HTML5 drag, outbox replay,
   filters/rules.
4. Account wizard + POP3. Autoconfig wizard, POP3, multiple accounts each with a separate inbox.
5. Search + address book. FTS5 search, contacts, vCard import/export.
6. Calendar. Month/week/day, events, reminders, ICS import/export, remote ICS subscription.
7. Packaging + polish. Signed installers for all three platforms, keyboard navigation as an explicit
   focus ring, About dialog with OSS credits + GPL-3.0 licence, pigeonpost.ink downloads.
8. v2 candidates. Two-way CalDAV/CardDAV (go-webdav) and PGP/S-MIME; message templates shipped in 0.14.0.

---

## 11. Locked decisions

- Licence: GPL-3.0, whole app.
- Stack: Go + Wails + React/TS, Emersion mail suite, pure-Go SQLite + FTS5, OS keychain.
- Auth: password (or app password) over generic IMAP/POP3 is the core path; XOAUTH2 OAuth is
  implemented and Microsoft uses it (authorization-code plus PKCE, loopback redirect, free Entra
  registration). Gmail personal accounts are supported via an app-password preset; one-click Google
  sign-in is declined (annual CASA fee) and Workspace accounts are OAuth-only, so they are not covered
  by the personal preset.
- Calendar/contacts: file ICS/vCard import/export + read-only ICS subscription in v1; two-way
  CalDAV/CardDAV in v2.
- Compose: light TipTap rich-text + plain-text toggle in v1; no full HTML-editor parity.
- Inboxes: each account keeps its own separate inbox; there is no unified/combined inbox.
