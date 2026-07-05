# PigeonPost Architecture

## Invariant

`UI -> Application -> Domain <- Infrastructure`

Dependencies point inward. The Domain is the stable core and depends on nothing. Every rule below is
enforced by a test in `tests/structural/boundary_test.go`, not by convention.

| Invariant | Enforcing test |
|---|---|
| Domain imports nothing from application/infrastructure/ui/wails | `TestDomainHasNoOutwardImports` |
| Domain is pure: no net, os, database/sql, time.Now, math/rand | `TestDomainIsPure` |
| Application never imports infrastructure or wails | `TestApplicationDoesNotImportInfrastructure` |
| No Go source file exceeds the module-size limit | `TestNoFileExceedsLineLimit` |
| The composition root is the only place that wires concrete adapters | `TestCompositionRootIsWhitelisted` |

## Layers

- **Domain** (`internal/domain`): pure Go, standard library only. Immutable value objects validated
  on construction (`With*` copy methods for change). No IO, no wall-clock reads; time enters through
  the injected `Clock`. This is where correctness lives and where the 100% coverage gate applies.
- **Application** (`internal/application`): use cases plus the port interfaces they depend on
  (`AccountStore`, `CredentialStore`, `AccountVerifier`, `MailStore`, `MailSource`, `MailActions`,
  `MailTransport`, `FolderActions`, `DraftSaver`, `OutboxStore`, `TagStore`, `RuleStore`, `Clock`). The
  `MailSource`, `MailActions` and `AccountVerifier` ports are satisfied by the `mailrouter` adapter,
  which dispatches to the IMAP or POP3 implementation per account protocol. Depends on Domain and the
  standard library only. Never imports Infrastructure or the Wails runtime.
- **Infrastructure** (`internal/infrastructure`): concrete adapters implementing the Application
  ports. Owns SQLite (`storage`), IMAP (`imap`), POP3 (`pop3`, a hand-rolled client), SMTP (`smtp`), the
  shared RFC 5322 MIME builder (`message`, used by both `smtp` for send and `imap` for draft append so
  the message-format logic is not duplicated), the shared message-body parser with HTML sanitising and
  image-blocking (`mailparse`, used by both the IMAP and POP3 read paths), the per-protocol dispatcher
  (`mailrouter`, which routes reads, verification and actions to the IMAP or POP3 adapter by account
  protocol), the Windows taskbar unread-overlay badge (`taskbar`, a build-tagged no-op elsewhere) and
  the OS keychain (`keychain`); later ICS, vCard and OAuth. Never imported by Domain or Application. The
  `installer` package holds the setup program's install logic and is consumed by the `installer/` Wails
  setup app.
- **UI**: the React front end plus the thin Wails facade in package `main` (`app.go` with its
  per-feature binding files `about.go`, `accountsetup.go`, `send.go`, `export.go`, `outbox.go`,
  `rulesapi.go` and `tagsapi.go`, the `dto.go` DTO mappers and the `clock.go` clock). The facade is a
  client of the Application use cases only; it maps domain results to DTOs and holds no business logic.

## Composition root

`main.go` is the single composition root. It constructs concrete infrastructure adapters and injects
them into the application use cases by constructor injection, then hands the assembled facade to
Wails. There are no global singletons, no service locator and no auto-wiring. The structural test
whitelists this file as the only one permitted to import both `application` and `infrastructure`.

## Dependency direction

```
             +-------------------+
   Wails/UI  |   app.go (facade) |
             +---------+---------+
                       | calls
             +---------v---------+
             |    application    |  ports (interfaces) + use cases
             +----+---------+----+
        depends on |         ^ implements
             +-----v----+    |
             |  domain  |    |
             +----------+    |
                       +-----+---------------+
                       |   infrastructure    |  sqlite, imap, ...
                       +---------------------+
```

## Execution flow

Sync and read:

1. `main.go` opens the SQLite store, builds the use cases (accounts, mailbox, sync, compose) and the
   Wails facade, injecting the concrete adapters.
2. The React UI asks the facade for accounts, folders and message summaries.
3. The sync use case pulls folders and message summaries from the mail source and persists them
   through the store; the UI reads from the store so it works offline. The `mailrouter` picks the
   adapter by account protocol: an IMAP account lists its server folders, while a POP3 account
   downloads into a single local inbox (POP3 has no server-side folders), deduped by its UIDL, with
   read and star marks kept locally since POP3 carries no server flags. The message server handle is an
   opaque string (schema v11) that holds an IMAP UID or a POP3 UIDL. Folder unread and total counts are
   computed from the cached messages, so the per-folder, per-account and total badges are populated
   without a separate server STATUS pass.

Add account:

1. The UI submits the setup wizard payload (identity, password, incoming and outgoing servers) to the
   facade's `AddAccount`.
2. The facade maps the wire strings to domain enums and builds a validated `Account` (its id is the
   email address), then calls the `AccountSetupService`.
3. The setup use case verifies the credentials against the incoming server through the
   `AccountVerifier` port (IMAP login) *first*, then stores the password through the `CredentialStore`
   port (keychain) and persists the account through `AccountStore`. Because nothing is written until
   the password is known good, a failed verify leaves the keychain and store untouched.

Edit account:

1. The UI opens the wizard prefilled from the account (its email is fixed, so that field is locked)
   and calls the facade's `UpdateAccount`.
2. The `AccountSetupService.Update` use case verifies first: a blank password re-verifies with the
   existing keychain secret (read server-side, never sent to the UI); a new password is verified and,
   only if good, replaces the stored one. The account is then persisted. A failed verify never
   disturbs the working account's stored password.

Remove account:

1. The UI confirms the destructive action in a modal, then calls the facade's `RemoveAccount`.
2. The `AccountService` deletes the account row (so it leaves the UI at once), then clears its cached
   folders and messages through `MailStore` and finally deletes its keychain secret through
   `CredentialStore`. The account's mail on the server is never touched.

Read a message body:

1. Opening a message calls the facade's `GetMessageBody`.
2. The `MessageBodyService` serves the cached body when present; on a miss it resolves the message,
   its folder and account through the stores, fetches the full body from the `MailSource`, caches it
   (schema v3 `message_body` table) and returns it. The message therefore reads offline after its
   first open. The shared `mailparse` package (used by both the IMAP and POP3 read paths) parses the MIME
   into plain-text and HTML parts; the HTML is sanitised there (bluemonday) so only safe markup ever
   enters the cache, and an HTML-only message also gets a plain-text rendering derived from the HTML.
   The same pre-sanitise pass drops nodes the sender hid with inline CSS (a preheader / preview-text
   block): the sanitiser strips the style that hid them, so left in place they would surface and
   duplicate the visible content. Before sanitising, every remote `<img src>` is parked in
   a `data-pp-src` attribute (and `srcset` dropped) so images do not auto-load, which would leak that
   the reader opened the message; the UI shows a "Load images" action that restores the source on
   request. The UI renders the sanitised HTML when present (links open in the external browser via the
   facade, never the app's own webview) and falls back to the plain text otherwise.

Send (also reply, reply-all and forward, which just pre-fill the same compose window before the
identical send path runs: reply pre-fills the sender; reply-all pre-fills the sender plus the original
To and Cc with the reader's own address and duplicates removed; forward pre-fills the quoted original;
all set a `Re:`/`Fwd:` subject). Reply-all is possible because the cached message summary now stores the
original To and Cc (schema v6):

1. The UI submits a compose request (recipients, subject, body) to the facade.
2. The facade parses the addresses into domain value objects and calls the compose use case.
3. The compose use case loads the account, builds a validated `OutgoingMessage` (sender taken from
   the account) and hands it to the SMTP transport, which authenticates using the keychain password
   and delivers it. The compose editor is TipTap rich text: the draft carries both a plain-text body
   and an optional HTML body, and when HTML is present the shared `message` MIME builder emits a
   `multipart/alternative` message (plain text first, HTML second) so plain-text clients still render.
   Bcc recipients (schema v8) are added to the SMTP envelope (de-duplicated with To and Cc) but never
   written to the headers. Attachments (schema v9) turn the body into `multipart/mixed`: files chosen
   from disk plus, optionally, an existing message fetched as a `message/rfc822` part, bounded by a
   total-size cap in the facade.

Save draft: Compose > Save draft calls the compose use case, which resolves the account's Drafts
mailbox from the cached folders and, through the `DraftSaver` port, renders the message with the shared
`message` builder and appends it to that mailbox on the server (IMAP APPEND, flagged `\Draft \Seen`).
Unlike a send, a draft may be incomplete (no recipients, empty body), so it is built with the lenient
`NewDraftMessage`.

Offline outbox: the SMTP and IMAP adapters wrap a failed dial with the `ErrOffline` sentinel. When the
compose use case sees `ErrOffline` from a send or a draft append, instead of failing it queues the
operation through the `OutboxStore` port (schema v5 `outbox` table, extended with Bcc in v8 and
attachments in v9 so a queued message keeps them on replay) and returns success; the UI surfaces the
queue as a per-account outbox folder where the waiting messages can be reviewed or cancelled. After the
next successful sync the UI calls replay, which drains the queue oldest-first: each item is re-sent or
re-appended, removed on success, left in place if still offline, and dropped (with its error reported)
if it can never succeed. The queue covers outgoing mail only; message flag/delete/move actions remain
online-only by design.

Delete a message: after a confirmation modal, the UI calls the facade, routed through the
`MessageActionService`. It resolves the message's folder and account, then via the `MailActions` port
moves the message to the account's Trash folder when one exists, or deletes it permanently (mark
`\Deleted` and expunge) when the message is already in Trash or the account has no Trash folder. The
cached message and everything derived from it (body, tags, index row) are then removed locally.

Move a message: the UI offers the account's other folders; choosing one routes through the
`MessageActionService`, which checks the destination is in the same account, moves the message on the
server via the `MailActions` port and removes the local copy (the destination folder re-lists it, with
its new server UID, on the next sync). Copy is the same path without removing the original.

Folder operations: the `FolderService` creates, renames and deletes mailboxes on the server through the
`FolderActions` port. Each cached `Folder` records the server's mailbox hierarchy delimiter (schema
v10), captured from the IMAP `LIST` response, so the leaf name and a rename's destination path are
derived with the real separator ("." on StartMail, not the default "/"); a folder with an unknown
delimiter falls back to "/".

Mark read/unread and star/flag: the UI calls the facade, which routes through the
`MessageActionService`. It writes the flag (`\Seen` or `\Flagged`) to the IMAP server first (via the
`MailActions` port) and only then updates the local cache, so the change is durable: a later sync
mirrors server state back and preserves it rather than overwriting a local-only flag. The unread
(bold) state and the star follow the cached flags.

Search: the `MailboxService.Search` use case runs a free-text query against the local cache through
the `MailStore`. The store keeps a SQLite FTS5 index (`message_fts`, schema v4) in step with the
message table on every save and turns raw user input into a safe prefix-match MATCH expression, so
search is instant and offline. The UI runs it debounced and shows the ranked results in place of the
folder listing.

Coloured tags: the `TagService` use case manages user-defined tags (a name plus a validated `#rrggbb`
`Colour`) and their many-to-many association with messages, through the `TagStore` port. Tags and the
`message_tag` link table are added by schema v2; migrations apply incrementally from the recorded
`user_version`. Tags are local to the cache for now; round-tripping them onto IMAP keywords is a
later phase.

Filter rules: the `RuleService` use case manages user-defined rules through the `RuleStore` port and
applies them to arriving messages. A domain `Rule` matches one field (From, To, Cc or Subject) against a
value with an operator (contains, is, starts with, ends with or does not contain) and carries an action
(mark read or flag). The operator column is schema v12 (added by `ALTER TABLE` defaulting to contains,
so pre-existing rules keep their behaviour). Matching and the action are pure domain logic; move and
delete on arrival stay deferred because they need UID reconciliation to be safe.

## Errors

Wrapped with `fmt.Errorf("...: %w", err)` and matched with `errors.Is` against sentinel errors. No
custom error types beyond sentinels.

## Quality enforcement

- `internal/domain` at 100% test coverage.
- Application use cases tested against hand-written fakes (no mock libraries).
- Infrastructure tested against a real SQLite database in a temp directory.
- Structural AST tests enforce layering, domain purity, the module-size limit and the composition
  root whitelist.

## Calendar and contacts (0.7.0)

Shipped in 0.7.0. This section records the shape of the address book and calendar; each piece is held to
the same layer rules and tests as the body above.
The invariant is unchanged: `UI -> Application -> Domain <- Infrastructure`, same layer rules, same
composition root, same 100% domain gate. The address book is built before the calendar because it is
the simpler half (no recurrence, timezones or RRULE) and it exercises the shared import/export seam the
calendar then reuses.

**Domain.** New pure value objects, immutable and validated on construction like the mail entities.
Address book first: `Contact` (id, vCard UID for lossless round-trip, formatted name, given/family
name, organization, title, note, and slices of `ContactEmail` and `ContactPhone`, each a labelled
value) and `ContactGroup` (id, name, member contact ids, with `With*` copy methods for membership).
Calendar: `Calendar` and `Event` (id, ICS UID, summary, start/end, all-day flag, location,
description, and an optional recurrence rule), with time entering only as already-resolved values, the
domain still reads no wall clock.

**Application.** New ports mirroring the mail stores: `ContactStore` (list, get, save, delete contacts
and groups) and `CalendarStore` (calendars and events). Import and export sit behind a codec seam
so the use case is format-agnostic: a `ContactCodec` interface with `Decode([]byte) ([]domain.Contact,
error)` and `Encode([]domain.Contact) ([]byte, error)`, implemented once per format, and a
`CalendarCodec` likewise. An `ImportContacts` / `ExportContacts` use case selects the codec by the
chosen format and reconciles by UID so a re-import updates rather than duplicates.

**Infrastructure.** New adapters implementing the ports: `storage` gains `contact`, `contact_email`,
`contact_phone`, `contact_group` and `contact_group_member` tables (schema v14), and `calendar` and
`event` tables (schema v15). Codec adapters: `vcard` (emersion/go-vcard) and `csv`
(stdlib `encoding/csv`) for contacts, and `ical` (emersion/go-ical) for calendar. Two contact codecs
exist deliberately: vCard covers Thunderbird and single-contact Outlook, and CSV covers Outlook's bulk
contact export/import (Outlook exports the address book as CSV, not vCard; Thunderbird reads CSV too).
The pure decode/encode logic lives in these packages and is covered to 100%; only genuine file or OS
edges are excluded.

**UI.** A contacts pane (list plus an editor dialog, reusing the confirm-before-delete rule) and
calendar month, week and day views, both clients of the Application use cases only. The week and day
views are an hour time-grid: an all-day strip, timed events sized by start and end, clashing events in
side-by-side lanes.

**Interop acceptance.** A real export from Outlook and from Thunderbird imports cleanly into PigeonPost;
a PigeonPost export imports back into both without loss, for calendar (ICS) and contacts (vCard and CSV).

**Calendar recurrence (RFC 5545 expansion).** The `Event` now models the whole recurrence set: the raw
RRULE plus RDATE and EXDATE occurrence lists and a RECURRENCE-ID for an override event, all as
already-resolved values so the domain still reads no wall clock (schema v18 stores the date lists as
comma-separated Unix milliseconds and the recurrence id as milliseconds). Expansion needs an RRULE
parser, which the domain must not depend on, so it lives behind a new Application port,
`RecurrenceService` (`Expand` an event into `EventInstance` occurrences within a window; `TruncateBefore`
rewrite a rule to end before a time), implemented in `infrastructure/recurrence` over the pure-Go
`teambition/rrule-go` library. `CalendarService.ListEventInstances(from, to)` groups events by series
(UID, or id when absent), expands each master, suppresses the generated occurrence an override replaces,
and merges one-off events, all sorted by start; a malformed rule degrades to a single instance rather
than losing the event. Editing or deleting a recurring occurrence carries a scope (this, this-and-future,
all): `this` writes a single-occurrence override, `future` truncates the master with UNTIL and starts a
new series from the split (migrating later overrides), and `all` rewrites the master. The `ics` codec
extracts and re-emits RDATE, EXDATE and RECURRENCE-ID alongside the existing opaque `Extra`
pass-through, so the round-trip stays lossless.

**Event timezones.** An `Event` also carries an IANA zone (schema v19 `time_zone`), so a recurring event
keeps its local wall-clock time across daylight-saving changes: its Start and End stay absolute instants,
and the zone says how they are shown and expanded. The expander anchors DTSTART in that zone before
generating, so a 9am daily event stays 9am local while its UTC instant shifts across the DST boundary;
the IANA database is embedded (`time/tzdata`) so `LoadLocation` resolves on Windows. The `ics` codec reads
the `TZID` parameter on import and writes `DTSTART;TZID=...` on export (the IANA name, which Google,
Outlook and Thunderbird resolve from their own databases); a UTC or all-day event carries no zone. On the
front end a zone picker sets the event zone, the form interprets and shows its wall-clock times in that
zone, and occurrences render in the browser's local zone. A generated `VTIMEZONE` block is a later
refinement; RDATE, EXDATE and RECURRENCE-ID are written as UTC instants.

**Reminders.** An `Event` carries a list of `Alarm` reminders, each a signed trigger offset from the start
(schema v20 stores them as comma-separated seconds; the facade exposes them to the UI as whole
minutes-before). The `ics` codec reads relative-trigger `VALARM` children into alarms and re-emits one
`DISPLAY VALARM` per modelled alarm with a friendly duration (`-PT15M`, not the library's `-PT900S`);
because it owns the property it strips existing VALARMs first, so an exotic imported alarm (an absolute
trigger, an email action) is not preserved. `CalendarService.DueReminders(since, now)` expands events and
returns the reminders whose trigger falls in that window; a scheduler goroutine in the composition root
polls it every thirty seconds (starting from launch so no backlog fires) and emits a Wails event that the
front end shows as an on-screen banner. It fires while the app runs; OS-level toasts and a taskbar flash
for a minimised window are a later addition.
