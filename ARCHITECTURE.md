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
  `MailTransport`, `DraftSaver`, `OutboxStore`, `TagStore`, `Clock`). Depends on Domain and the
  standard library only. Never imports Infrastructure or the Wails runtime.
- **Infrastructure** (`internal/infrastructure`): concrete adapters implementing the Application
  ports. Owns SQLite (`storage`), IMAP (`imap`), SMTP (`smtp`), the shared RFC 5322 MIME builder
  (`message`, used by both `smtp` for send and `imap` for draft append so the message-format logic is
  not duplicated) and the OS keychain (`keychain`); later ICS, vCard and OAuth. Never imported by
  Domain or Application. The `installer` package holds the setup program's install logic and is
  consumed by the `installer/` Wails setup app.
- **UI**: the React front end plus the thin Wails facade in package `main` (`app.go`, `about.go`,
  `send.go`, `accountsetup.go`, `dto.go`). The facade is a client of the Application use cases only; it
  maps domain results to DTOs and holds no business logic.

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
3. The sync use case pulls folders and message summaries from the IMAP source and persists them
   through the store; the UI reads from the store so it works offline.

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
   first open. The IMAP adapter parses the MIME into plain-text and HTML parts; the HTML is sanitised
   there (bluemonday) so only safe markup ever enters the cache, and an HTML-only message also gets a
   plain-text rendering derived from the HTML. Before sanitising, every remote `<img src>` is parked in
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
attachments in v9 so a queued message keeps them on replay) and returns success; the title bar shows
how many operations are waiting and opens an outbox view for reviewing or cancelling them. After the
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

## Errors

Wrapped with `fmt.Errorf("...: %w", err)` and matched with `errors.Is` against sentinel errors. No
custom error types beyond sentinels.

## Quality enforcement

- `internal/domain` at 100% test coverage.
- Application use cases tested against hand-written fakes (no mock libraries).
- Infrastructure tested against a real SQLite database in a temp directory.
- Structural AST tests enforce layering, domain purity, the module-size limit and the composition
  root whitelist.
