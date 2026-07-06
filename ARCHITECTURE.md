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
  protocol), the reminder and unread surfaces (`taskbar`: the Windows taskbar unread-overlay badge and
  reminder flash, no-ops off Windows, plus the notification tray, a Windows tray icon that also carries
  the unread badge, or a native desktop notification elsewhere) and the OS keychain (`keychain`); later
  ICS and vCard. Never imported by Domain or Application. The
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
   first open. The parser also extracts attachment parts, cached alongside the body (schema v26
   `message_attachment` table) so a received attachment can be saved offline from the reader; the list
   shows a paperclip for a message whose fetched IMAP body structure (BODYSTRUCTURE) has an
   attachment-disposition part. Schema v27 clears the cached bodies once so each re-fetches with the
   attachment-aware parser. Subjects and display names are RFC 2047 decoded (through a charset reader, so
   windows-1252 and the like decode) and HTML-entity unescaped in the mail-source mapping via
   `mailparse.DecodeHeader`, shared by the IMAP envelope path and POP3, so encoded-word and
   template-built headers read as text. The shared `mailparse` package (used by both the IMAP and POP3
   read paths) parses the MIME into plain-text and HTML parts; the HTML is sanitised there (bluemonday) so only safe markup ever
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

Draft recovery: separately from the server-side Save draft, the compose window autosaves its in-progress
content (debounced, once the user has edited it) to a single-row local slot through the
`DraftRecoveryStore` port (schema v25 `draft_recovery` table). It is local only and never sent to the
server; on the next launch the UI offers to restore it; sending, saving a server draft or discarding
it clears the slot.

Offline outbox: the SMTP and IMAP adapters wrap a failed dial with the `ErrOffline` sentinel. When the
compose use case sees `ErrOffline` from a send or a draft append, instead of failing it queues the
operation through the `OutboxStore` port (schema v5 `outbox` table, extended with Bcc in v8 and
attachments in v9 so a queued message keeps them on replay) and returns success; the UI surfaces the
queue as a per-account outbox folder where the waiting messages can be reviewed or cancelled. After the
next successful sync the UI calls replay, which drains the queue oldest-first: each item is re-sent or
re-appended, removed on success, left in place if still offline, and dropped (with its error reported)
if it can never succeed. The queue covers outgoing mail only; message flag/delete/move actions remain
online-only by design.

Junk, conversations and list order: marking a message as junk moves it to the account's Junk folder
through the same online path as Move (`MessageActionService.MarkJunk`, resolving the Junk folder by kind).
Conversation grouping and list order are read-side concerns over the same cached summaries the flat list
uses: the domain `GroupThreads` groups a folder's summaries into conversations by normalised subject
(reply/forward prefixes stripped), exposed through `MailboxService.Threads`; the UI sorts the list by
date in either direction. The desktop list mirrors the grouping client-side so it updates instantly with
optimistic changes, keeping the domain function as the single tested definition.

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
(bold) state and the star follow the cached flags. `UnreadCounts` is the single derived-total choke
point: it reflects the cross-account total onto both the taskbar overlay badge and the tray icon (the
tray icon composites the app icon with the same red count badge, so the count stays visible even when
the window is hidden to the tray).

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
and groups) and `CalendarStore` (calendars, events and preserved passthrough components). Import and export sit behind a codec seam
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
new series from the split (migrating later overrides), and `all` rewrites the master. When the split
leaves the recurrence unchanged, `SplitCountForward` reduces a COUNT-based rule by the occurrences before
the split so the forward series carries the remaining count and the two halves keep the original total
(an open-ended or UNTIL rule needs no adjustment; a rule the user changed is honoured as given). The `ics` codec
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
zone, and occurrences render in the browser's local zone. Export also emits a `VTIMEZONE` for every zone
the events use, so the file defines the zones its `TZID` parameters reference rather than relying on the
reading application's own database. Each is generated by probing the zone across the earliest event's
year to find its standard and daylight offsets and the transitions between them, then writing STANDARD
and DAYLIGHT sub-components with an RRULE derived from each transition date (a zone without daylight
saving gets a single STANDARD). RDATE, EXDATE and RECURRENCE-ID are written as UTC instants.

**To-dos and journals.** The `ics` codec models only VEVENTs, but a VTODO or VJOURNAL is preserved
verbatim as a `domain.CalendarPassthrough` (UID, kind, the component re-serialised as a standalone
VCALENDAR) rather than dropped. `Decode` returns passthrough alongside the events; `ImportEvents` stores
each in the `calendar_passthrough` table (schema v21, keyed by UID so a re-import replaces); and
`ExportEvents` re-emits them. So an imported calendar's tasks and notes survive an import and export
round-trip even though PigeonPost does not yet display them.

**Reminders.** An `Event` carries a list of `Alarm` reminders, each a signed trigger offset from the start
(schema v20 stores them as comma-separated seconds; the facade exposes them to the UI as whole
minutes-before). The `ics` codec reads relative-trigger `VALARM` children into alarms and re-emits one
`DISPLAY VALARM` per modelled alarm with a friendly duration (`-PT15M`, not the library's `-PT900S`);
because it owns the property it strips existing VALARMs first, so an exotic imported alarm (an absolute
trigger, an email action) is not preserved. `CalendarService.DueReminders(since, now)` expands events and
returns the reminders whose trigger falls in that window; a scheduler goroutine in the composition root
polls it every thirty seconds and emits a Wails event that the front end shows as an on-screen banner. On
launch it first calls `PendingReminders(now)`, which fires reminders for still-imminent events (starting
at or after now) whose trigger lapsed while the app was closed, so a reminder for an upcoming event is not
missed; a reminder for an event already started or past is not resurrected, and the catch-up and live
windows do not overlap. When a batch of reminders fires, the composition root also draws attention from
outside the window: it flashes the taskbar button through an injected `ReminderAlerter` (the `taskbar`
package's `Flasher`, a build-tagged no-op off Windows) and raises a notification through the `taskbar`
package's `Tray`. The tray notification is a Windows balloon on the tray icon, or a native desktop
notification off Windows (a freedesktop D-Bus notification on Linux, an `osascript` notification on
macOS, a no-op on any other platform). Both alerts skip when the window is already in the foreground, so
an in-view reminder relies on its banner alone. On Windows the `Tray` is a persistent, clickable
notification-area icon: left-clicking it reopens the window, and its right-click menu mirrors the Help
menu (About, Licence, Check for Updates) plus Open and Quit. Where a restorable tray icon exists (only
Windows, gated by `Tray.CanHideToTray`), the window's close button does not quit: `OnBeforeClose` keeps
the window open and emits `app:close-request`, and the front end shows its own dark-themed dialog
offering Minimise to tray or Quit. Minimise calls `MinimiseToTray` (which hides the window so the
scheduler and mail sync keep running in the background); Quit calls `RequestQuit`; dismissing the dialog
leaves the window open. A native dialog is deliberately avoided so the prompt matches the app theme.
Where no tray icon exists the close button simply quits. The tray menu's Quit sets a flag so it exits
without re-triggering that prompt, since it drives the same close path. To keep the `taskbar` package
free of any UI-framework dependency, the tray's Open and menu items invoke callbacks supplied by the
`App` facade, which reopen the window (`WindowShow`), emit `menu:*` Wails events the front end turns into
the same dialogs the in-window Help menu opens, or quit.

**Meeting scheduling (iTIP / iMIP).** An event with attendees is a meeting, and PigeonPost sends and
receives the RFC 5546 scheduling messages (REQUEST, REPLY, CANCEL) as RFC 6047 iMIP `text/calendar` mail
parts. New pure domain value objects carry the data: `Organizer` (a validated address plus an optional
common name) and `Attendee` (address, common name, a `Role` and a `ParticipationStatus` enum each parsed
leniently, and an RSVP flag with a `WithStatus` copy method), with `Event` gaining an organizer and an
attendee list (schema v23 stores them as an `event.organizer` column and a JSON `event.attendees`
column). A `scheduling.go` domain file adds the `Method` enum, the `SchedulingMessage` (a method plus its
events) and the `CalendarPart` (a method plus the encoded bytes) that an outgoing message carries. These
value objects and their parse rules are held to the 100% domain gate.

The codec seam gains a `SchedulingCodec` port (`DecodeScheduling` reads a VCALENDAR's METHOD and events;
`EncodeRequest`, `EncodeReply` and `EncodeCancel` build the payloads), satisfied by the same `ics`
adapter over go-ical. The `SchedulingService` use case (application layer, 100% gated) drives the flows:
`Respond` saves an incoming REQUEST to the calendar with the chosen PARTSTAT and emails a REPLY to the
organizer; `ApplyReply` folds an incoming REPLY into the organizer's stored meeting, updating the
responding attendee's status; `ApplyCancellation` removes the meeting a CANCEL withdraws; and
`SendRequest` / `SendCancel` email a REQUEST or CANCEL to a meeting's attendees from the organizing
account. A recurring meeting is matched as its series master plus any overrides, keyed by UID and
RECURRENCE-ID.

Mail carries the invites both ways. Incoming: the shared `mailparse` parser diverts a `text/calendar`
part into a `ParsedBody.Invite`, and the cached `MessageBody` gains an `invite` column (schema v22) with
`HasInvite` / `Invite`, so a message reading offline still shows its invitation. The `MailSource.FetchBody`
port and both the IMAP and POP3 adapters return the raw calendar bytes alongside the plain and HTML
parts. Outgoing: an `OutgoingMessage` carries an optional `CalendarPart`, and the shared `message` MIME
builder writes it as a `text/calendar; method=...; charset=utf-8` part inside the `multipart/mixed` body,
so one sent message is both a readable email and a valid iMIP scheduling message.

The Wails facade (`schedulingapi.go`) exposes the flow through `OrganizerDTO`, `AttendeeDTO` and
`InvitationDTO` and the methods `GetInvitation`, `RespondToInvitation`, `RemoveCancelledMeeting`,
`ApplyMeetingReply`, `SendMeetingRequest` and `SendMeetingCancel`; `EventDTO` and `EventRequest` carry the
organizer and attendees so a meeting round-trips through the calendar editor. As with the rest of the
facade, these binding files are build-verified (they hold no logic beyond DTO mapping) rather than
unit-tested; the correctness lives in the domain and application layers behind them. In the UI the reader
shows an invite card (Accept, Tentative or Decline a request, remove a cancellation, apply a reply) and
the calendar event editor edits a meeting's attendee list and sends its invitations and cancellations. A
join link an invite carries in its location or description (Microsoft Teams, Google Meet, Zoom or Webex,
matched by host) surfaces as a Join button in the event editor, and any other link in the description is
clickable; both open in the external browser through the existing `OpenExternal` facade method, so this
adds no new port.

**New-mail notifications and IMAP IDLE (0.8.0).** New mail is surfaced the moment it arrives. A
`runMailNotifier` goroutine in the composition root owns the flow: it primes a baseline first (an existing
inbox is cached, not announced), then feeds two detection paths through one serialised `checkMail`, so a
push and the backstop poll can never double-notify. `SyncInboxes` (application) refreshes every account's
inbox and returns the messages whose id is not already cached, keyed on arrival rather than read state, so
a message another client already marked read still counts while only a filter-rule read-on-arrival is
silenced. A new message raises a desktop notification through the same `taskbar` `Tray` the reminders use,
forced to show even when the window is focused because new mail has no in-window cue.

Instant delivery is an IMAP IDLE watcher. `infrastructure/imap`'s `Watcher` holds a persistent,
authenticated IDLE connection per IMAP account and invokes a callback the moment the server reports the
mailbox changed, reconnecting with capped exponential backoff and reissuing the IDLE inside the server's
timeout window; a server without the IDLE capability stops cleanly and is left to the poll. The watcher is
injected into the facade behind a `MailWatcher` port, so the application layer keeps no IMAP dependency. A
60-second poll is the backstop for a missed push and for POP3, which has no IDLE.

The watcher set is kept in step with the accounts, so an account added after launch gets instant push
without a restart. Each account's watcher runs under its own cancellable child of the app context, tracked
by id: `AddAccount` starts one, `UpdateAccount` restarts it so changed server settings take effect (and a
switch to POP3 drops the IMAP watcher), and `RemoveAccount` stops it so no stale connection is left.
Shutdown cancels the app context and stops them all. A fired reminder banner is clickable, opening the
calendar on that event through the existing calendar binding.

Only one PigeonPost runs per user, enforced by Wails' `SingleInstanceLock` (a named mutex on Windows).
A second launch does not open a new window: the running instance's `OnSecondInstanceLaunch` reveals its
window through the same `WindowShow`/`WindowUnminimise` path the tray uses, so relaunching an app hidden
in the tray simply brings it back.
