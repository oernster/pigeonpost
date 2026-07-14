# PigeonPost: Design Plan

Cross-platform desktop email, calendar and address book client. Go core, React front end,
local-first. Delivered as a signed download at https://www.pigeonpost.ink.

Status: living roadmap. Working version 0.15.0 (schema v42). The per-version change history lives in the release notes, not in this document.

This document is the target design and the forward roadmap. It is living: delivered work is exorcised as it lands (a shipped feature lives in the code and the release notes, not here), so section 10 holds only what is not yet built. Sections 2 to 9 still describe the delivered v1 as the standing design reference.
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
- Search: fast local full-text search (SQLite FTS5) with an operator grammar; server-side IMAP SEARCH
  is parked (the local index is the product).
- Message filters/rules: on-arrival actions (move, tag, mark, delete).
- Address book: contacts with groups, vCard (.vcf) import/export, plus CSV import/export for Outlook
  (whose bulk contact export is CSV, not vCard). Import/export must round-trip with Outlook and
  Thunderbird.
- Calendar: month/week/day views, events with reminders, meeting invites (iTIP/iMIP), ICS (.ics)
  import/export, read-only remote ICS subscription.
- Offline: cached mail readable offline; actions queued and replayed on reconnect.

### Out of scope for v1 (named so they do not creep)

Chat/matrix/newsgroups/RSS, add-on/extension system, message-template gallery, full HTML-editor
parity, PGP/S-MIME (v2 candidate), server-side sieve editing, Gmail-specific support (see section 7).
Two-way CalDAV/CardDAV was a v2 item; it is now active (section 10).

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
- message_search: FTS5 table over subject/snippet/sender/recipients/body/filenames for instant local
  search, fed from the message_searchable_text view (the single definition of a message's searchable
  text). Bodies index as they are cached on first open.

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
CalDAV/CardDAV (go-webdav) is now active (section 10; phase 1 in progress).

Outlook and Thunderbird interop is an explicit v1 requirement, not just a by-product of using standard
formats. Calendar uses ICS (RFC 5545), which both apps read and write. Contacts need two formats: vCard
(.vcf) for Thunderbird and single-contact Outlook, and CSV for Outlook's bulk contact export/import
(Outlook exports the address book as CSV, not vCard; Thunderbird reads CSV too), so a CSV mapper sits
alongside the vCard one behind the same import/export port. The acceptance test is a real export from
each app importing cleanly into PigeonPost, and a PigeonPost export importing back into both without
loss.

### Search

Local FTS5 with a modelled operator grammar (field operators, structural predicates, phrases, OR and
negation), BM25 ranking with subject and sender boosted, highlighted match snippets and a scope
selector; debounced as-you-type. Server-side IMAP SEARCH is parked: the local index is the product.
Attachment-content indexing and unified mail+calendar+contacts search stay deferred.

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

## 10. Roadmap (forward only)

This section holds only work not yet delivered. A feature is removed once it ships: a delivered
capability lives in the code and the release notes, not here. The v1 phases (skeleton and read-only
IMAP, send and compose and flags, move/copy/drag and folders and outbox, the account wizard and POP3,
search and address book, calendar, packaging) are all delivered and have been removed, along with the
later work that also shipped: coloured tags with IMAP-keyword sync, message templates, event categories,
the multi-day month view, full RFC 5545 recurrence (expansion, RECURRENCE-ID overrides, the
this/this-and-future/all editor) and per-event IANA time zones with DST-correct expansion and
Windows-zone import.

### Two-way CalDAV / CardDAV (active, the 1.x flagship)

The biggest remaining "real Thunderbird replacement" win: sync calendars and contacts both ways with a
DAV server (Fastmail, iCloud, Google, Nextcloud), so calendar and contacts stop being local-only.

**Reuse (already built).** The whole sync discipline exists to lift: the tag two-way sync
(`tags_sync.go`: pending-intent write plus three-way reconcile of server/local/pending) is the object-sync
template; the Mail `Source`/`Actions`/`FolderActions` port split mirrors into `CalDAVSource`/`CalDAVActions`
and the CardDAV pair; the OAuth `TokenManager` and authorizer and `credentialFor` cover DAV-over-HTTPS
auth; the `OutboxStore` offline queue models pending object writes; `domain.Event`/`Contact` and the
ICS/vCard codecs are the serialization the engine PUTs and parses; `event.uid`/`contact.uid` seed object
identity.

**Build (greenfield).** `emersion/go-webdav`; a DAV account model (base URL plus principal, discovered);
per-object `etag` and `href` columns on `event` and `contact` (schema bump from v38); per-collection
`account_id` and a sync-token/CTag; pending-intent tables plus tombstones so offline create/update/delete
propagate; the RFC 6578 sync-collection incremental loop with a CTag-plus-ETag fallback; collection
discovery (`.well-known/caldav`, current-user-principal, calendar-home-set).

**Data flow.** Discover (principal, home-set, collections); initial pull (REPORT hrefs plus etags,
multiget the ICS bodies, decode with the existing codec, store with href plus etag); incremental
(sync-collection with the stored token, changed and removed hrefs); push (PUT `If-Match` for update and
`If-None-Match: *` for create, DELETE `If-Match`; a 412 refetches and reconciles).

**Phasing.**
1. Read-only calendar pull (in progress). DAV account plus discovery plus a full pull plus CTag refresh;
   remote calendars appear read-only next to local. Lowest risk, immediate value.
2. Two-way calendar. Pending-intent write-back, `If-Match` conflict handling, tombstones, sync-collection
   incremental.
3. CardDAV contacts. The same engine over the vCard codec and an address-book collection.
4. Polish. Multiple collections per account, colour and displayname sync, optional CalDAV scheduling inbox.

**Open decisions.** First server target (Fastmail plus a generic Nextcloud/Radicale, then iCloud, then
Google OAuth CalDAV last); conflict policy (last-writer-wins with a safety copy is the recommendation, to
match the calm automatic tag-sync ethos); account model (a separate `CalendarAccount` aggregate rather than
extending the mail `Account`, so DAV and mail hosts stay decoupled); incremental strategy (sync-token
preferred, CTag-plus-ETag fallback).

**Prerequisite (met).** A pulled DAV calendar is parsed by the same ICS codec, so the Windows-zone import
fix had to land first; it has.

### PGP / S-MIME (deferred, v2)

End-to-end mail encryption and signing. Deferred until the DAV work lands; no design started.

---

## 11. Locked decisions

- Licence: GPL-3.0, whole app.
- Stack: Go + Wails + React/TS, Emersion mail suite, pure-Go SQLite + FTS5, OS keychain.
- Auth: password (or app password) over generic IMAP/POP3 is the core path; XOAUTH2 OAuth is
  implemented and Microsoft uses it (authorization-code plus PKCE, loopback redirect, free Entra
  registration). Gmail personal accounts are supported via an app-password preset; one-click Google
  sign-in is declined (annual CASA fee) and Workspace accounts are OAuth-only, so they are not covered
  by the personal preset.
- Calendar/contacts: file ICS/vCard import/export + read-only ICS subscription shipped; two-way
  CalDAV/CardDAV is now active (section 10).
- Compose: light TipTap rich-text + plain-text toggle in v1; no full HTML-editor parity.
- Inboxes: each account keeps its own separate inbox; there is no unified/combined inbox.
