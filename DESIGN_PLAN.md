# PigeonPost — Design Plan

Cross-platform desktop email, calendar and address book client. Go core, React front end,
local-first. Delivered as a signed download at https://www.pigeonpost.ink.

Status: design locked; 0.5.0 being cut. The 0.4.0 core (account wizard, reading with sanitised HTML and
remote-image blocking, rich-text compose with reply/reply-all/forward, save-draft, the offline outbox,
coloured tags, server-synced actions and full-text search) is complete. 0.5.0 adds folder create/
rename/delete, message move/copy, on-arrival mark/flag rules, a right-click context menu, in-app reader
tabs, full keyboard navigation, Bcc, file and email attachments, save-as/print, drag-to-move and an
outbox view. POP3, OAuth, calendar and contacts remain ahead. This document is the target design; the actual
per-release delivery record lives in NOTES.md. Author: Oliver Ernster. Licence: GPL-3.0.

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
- Address book: contacts with groups, vCard (.vcf) import/export.
- Calendar: month/week/day views, events with reminders, ICS (.ics) import/export, read-only remote
  ICS subscription.
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
| SASL / XOAUTH2 | emersion/go-sasl | OAuth auth mechanism. |
| Calendar ICS | emersion/go-ical | RFC 5545 round-trip. |
| Contacts vCard | emersion/go-vcard | vCard 3/4 round-trip. |
| CalDAV / CardDAV (v2) | emersion/go-webdav | Two-way sync affordable in Go. |
| Storage | modernc.org/sqlite (pure Go, no CGO) + FTS5 | Local-first, single-writer/multi-reader. |
| Credentials | zalando/go-keyring | OS keychain; never in the DB. |
| OAuth2 | golang.org/x/oauth2 | XOAUTH2 for Microsoft/Google. |
| Front end | React 18 + TypeScript (Vite) | Existing React/TS + Wails lineage. |
| List virtualization | @tanstack/react-virtual | 100k-message folders scroll smoothly. |
| Drag/drop | dnd-kit | Message-to-folder, folder reorder. |
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

- account — id, display name, email, protocol (imap/pop3), server config, auth method
  (password/oauth), keychain_ref.
- folder — id, account_id, path, kind (inbox/sent/drafts/trash/junk/archive/custom), uid_validity,
  unread_count.
- message — id, folder_id, imap_uid, message_id, thread_id, from/to/cc, subject, date, size, flags
  bitfield, has_attachments, snippet, is_read, is_starred.
- message_body — message_id, plaintext, html, headers_blob (lazy-loaded).
- attachment — id, message_id, filename, mime, size, content_ref.
- tag — id, name, colour (the coloured-flags system).
- message_tag — message_id, tag_id.
- contact / contact_email / contact_group — address book, vCard-backed.
- calendar / event — event with RRULE, reminders, ICS UID for round-trip.
- filter_rule — condition + action set, evaluated on arrival.
- outbox_op — queued action (send/move/flag/delete) for offline replay with idempotency key.
- message_fts — FTS5 virtual table over subject/body/sender for instant local search.

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

Domain use cases `MoveMessages{message_ids, target_folder}` and `CopyMessages`. UI drag (dnd-kit) and
context menu both call the same use case, so drag is only an input method. Same-account move maps to
IMAP MOVE; cross-account move is copy-to-target then delete-from-source through the outbox so a
mid-way failure is recoverable.

### Coloured flags / tags

Named tags each with a colour, many-to-many with messages, mapped onto IMAP keywords ($Label1 etc.)
so they round-trip with the server. Bold-unread is a pure UI concern driven by is_read.

### Calendar and address book

go-ical for ICS import/export (Thunderbird/Outlook interop) and go-vcard for contacts. v1 supports
file import/export plus read-only remote ICS subscription. Events carry their original ICS UID so
export round-trips cleanly. Two-way CalDAV/CardDAV (go-webdav) is v2.

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

Auth method is a strategy behind an Authenticator interface from day one (password and XOAUTH2 both
implemented against it). v1 provider matrix:

- Microsoft (Outlook365 / Outlook.com): one-click OAuth is DEFERRED. It requires a Microsoft Entra
  directory, which now means a paid-tier-gated Azure sign-up (a card for identity verification, even
  though app registration itself is free), and that friction is not worth taking on for v1, mirroring
  the Gmail decision. Personal Outlook.com/Hotmail accounts connect through the generic IMAP path with
  an app password (the Outlook provider preset covers the servers). Office 365 work/school accounts,
  where Microsoft has disabled basic auth, are unsupported until OAuth ships. The Authenticator/XOAUTH2
  seam stays in place so OAuth can be added later without restructuring.
- Generic IMAP/POP3 + SMTP (Fastmail, self-hosted, ISP, corporate): password auth, the core path,
  from phase 1.
- Gmail: OUT OF SCOPE for v1. Google's IMAP/SMTP scope is restricted, triggering app verification
  plus an annual CASA security assessment (real money, long lead time). The friction is not worth it
  for v1. PigeonPost does nothing Gmail-specific (no Gmail OAuth, no autoconfig entry, no
  app-password guidance, no testing, no support). The generic IMAP path stays open, so a user who
  configures a Gmail app password manually can still connect, but Gmail is unadvertised and
  unsupported. Revisit embedded-credential Gmail OAuth (with CASA) only if the app reaches
  mainstream traction; bring-your-own client ID remains a possible future power-user option.

With Microsoft OAuth deferred, v1 has no external auth dependency at all: every supported account
signs in with a password (or an app password) over the generic IMAP/POP3 + SMTP path.

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
- Frontend: component tests for three-pane interactions and drag/move; avoid fragile pixel
  assertions.
- Read results by exit code, not console text.

---

## 10. Phased roadmap

Each phase ships something usable. Earlier phases de-risk the schema and sync engine (the
hardest-to-reverse parts) first.

1. Skeleton + read-only IMAP. Wails shell, clean-architecture packages, SQLite schema v1, single IMAP
   account via manual setup, three-pane read-only. Proves the sync engine and layering.
2. Send + compose + flags. SMTP send, compose (plain + TipTap rich-text), read/unread bold, coloured
   tags, star/junk.
3. Move/copy/drag + folders + offline outbox. Full folder ops, dnd-kit drag, outbox replay,
   filters/rules.
4. Account wizard + POP3. Autoconfig wizard, POP3, multiple accounts each with a separate inbox.
   (Microsoft XOAUTH2 deferred, see section 7.)
5. Search + address book. FTS5 search, contacts, vCard import/export.
6. Calendar. Month/week/day, events, reminders, ICS import/export, remote ICS subscription.
7. Packaging + polish. Signed installers for all three platforms, keyboard navigation as an explicit
   focus ring, About dialog with OSS credits + GPL-3.0 licence, pigeonpost.ink downloads.
8. v2 candidates. Two-way CalDAV/CardDAV (go-webdav), PGP/S-MIME, message templates.

---

## 11. Locked decisions

- Licence: GPL-3.0, whole app.
- Stack: Go + Wails + React/TS, Emersion mail suite, pure-Go SQLite + FTS5, OS keychain.
- Auth: password auth (generic IMAP/POP3) in v1; OAuth abstraction in place but Microsoft OAuth is
  deferred (it needs an Azure/Entra tenant). Personal Outlook.com connects via an app password; Office
  365 work/school accounts are unsupported until OAuth ships. Gmail out of scope for v1 (generic IMAP
  stays open but unsupported).
- Calendar/contacts: file ICS/vCard import/export + read-only ICS subscription in v1; two-way
  CalDAV/CardDAV in v2.
- Compose: light TipTap rich-text + plain-text toggle in v1; no full HTML-editor parity.
- Inboxes: each account keeps its own separate inbox; there is no unified/combined inbox.
