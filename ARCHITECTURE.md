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
  (`AccountStore`, `MailStore`, `MailSource`, `MailTransport`, `Clock`). Depends on Domain and the
  standard library only. Never imports Infrastructure or the Wails runtime.
- **Infrastructure** (`internal/infrastructure`): concrete adapters implementing the Application
  ports. Owns SQLite (`storage`), IMAP (`imap`), SMTP (`smtp`) and the OS keychain (`keychain`);
  later ICS, vCard and OAuth. Never imported by Domain or Application. The `installer` package holds
  the setup program's install logic and is consumed by the `installer/` Wails setup app.
- **UI**: the React front end plus the thin Wails facade in package `main` (`app.go`, `about.go`,
  `send.go`, `dto.go`). The facade is a client of the Application use cases only; it maps domain
  results to DTOs and holds no business logic.

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

Send:

1. The UI submits a compose request (recipients, subject, body) to the facade.
2. The facade parses the addresses into domain value objects and calls the compose use case.
3. The compose use case loads the account, builds a validated `OutgoingMessage` (sender taken from
   the account) and hands it to the SMTP transport, which authenticates using the keychain password
   and delivers it.

Mark read/unread: the UI calls the facade, which routes through the mailbox use case to update the
message's Seen flag in the local store; the unread (bold) state follows.

## Errors

Wrapped with `fmt.Errorf("...: %w", err)` and matched with `errors.Is` against sentinel errors. No
custom error types beyond sentinels.

## Quality enforcement

- `internal/domain` at 100% test coverage.
- Application use cases tested against hand-written fakes (no mock libraries).
- Infrastructure tested against a real SQLite database in a temp directory.
- Structural AST tests enforce layering, domain purity, the module-size limit and the composition
  root whitelist.
