# PigeonPost Testing

## Philosophy

- Correctness first. The domain and application layers hold the logic that must be right, and they
  are covered to 100%.
- No mock libraries. Collaborators are exercised through real implementations or small, hand-written
  fakes that implement the same interfaces (with error-injection fields). This keeps tests honest
  about real behaviour.
- Deterministic. The domain never reads the wall clock; time is injected via a `Clock`, so
  time-dependent behaviour is reproducible.

## The coverage gate

There is a hard 100% coverage gate on the correctness core:

- `internal/domain`
- `internal/application`

`./test.ps1` runs the whole suite with coverage and fails if any statement in those two packages is
uncovered. It also prints the full per-package report.

```
./test.ps1          # run tests and enforce the gate
./test.ps1 -Html    # also open the HTML coverage report
go test ./...       # plain run without the gate
```

Whole-repo 100% is deliberately not the target. The layers below only orchestrate live network I/O,
a native GUI, Win32 system calls or process startup; forcing coverage there would mean mocking the
operating system, which the no-mocks policy rejects. Instead, the *logic* in each of those packages
is factored into pure functions that are fully tested, and the thin I/O shell is excluded and
documented here.

## What is tested, and how

| Layer | Test type | Real resource |
|---|---|---|
| `internal/domain` | pure unit | none |
| `internal/application` | unit against hand-written fakes | none |
| `internal/infrastructure/storage` | integration against a real SQLite file | temp dir |
| `internal/infrastructure/message` | unit on the RFC 5322 MIME builder | none |
| `internal/infrastructure/mailparse` | unit on the MIME body parsing, HTML sanitising, image blocking and hidden-preheader removal that keeps MJML layout wrappers | none |
| `internal/infrastructure/mailrouter` | unit on the per-protocol dispatch | none |
| `internal/infrastructure/smtp` | none (live send only; MIME building lives in `message`) | n/a |
| `internal/infrastructure/imap` | unit on the source adapter's pure helpers (parsing moved to `mailparse`) | none |
| `internal/infrastructure/pop3` | unit on the response and UIDL parsing; live download excluded | none |
| `internal/infrastructure/keychain` | unit via go-keyring's in-memory mock | none |
| `internal/infrastructure/taskbar` | unit on the pure label formatting; Win32 overlay excluded | none |
| `internal/installer` | unit on payload extraction and paths | temp dir |
| `tests/structural` | AST scan of the source tree | file reads |

## Coverage snapshot

| Package | Coverage | Notes |
|---|---|---|
| internal/domain | 100% | gated |
| internal/application | 100% | gated |
| internal/infrastructure/message | 100% | the RFC 5322 MIME builder (pure) |
| internal/infrastructure/mailrouter | 100% | per-protocol dispatch (pure) |
| internal/infrastructure/keychain | 100% | go-keyring mock |
| internal/infrastructure/mailparse | ~92% | MIME body parsing, HTML sanitising, image blocking and hidden-preheader removal that keeps MJML layout wrappers and mso-hide content (pure); a few defensive decode branches uncovered |
| internal/infrastructure/storage | ~78% | logic and error paths covered, including keyset message pagination and the atomic tag-keyword sync writes; see exclusions |
| internal/infrastructure/pop3 | ~44% | response and UIDL parsing covered; the live dial and download excluded |
| internal/infrastructure/taskbar | ~22% | the pure label formatting and no-op stub covered; the Windows-only Win32 overlay excluded |
| internal/infrastructure/imap | ~24% | the source adapter's pure helpers; the wire-to-domain and HTML logic now lives in `mailparse`, and live fetch/append plus the IDLE watcher are excluded |
| internal/infrastructure/smtp | 0% | transport is live `Send` only; MIME building lives in `message` |
| internal/installer | ~38% | extract and paths covered; Win32 side effects excluded |
| main package, installer app, tools/genicons | 0% | composition root, GUI and tooling, excluded |

## Documented exclusions (and why)

- **Live IMAP fetch/append and the IDLE watcher** (`imap/source.go`, `imap/idle.go`), **live POP3
  download** (`pop3/`) and **live SMTP send**
  (`smtp/transport.go`): these dial a real server, authenticate and stream data. They cannot be
  unit-tested without a network, so the IMAP path sits behind a skippable integration test (below). The
  pure logic is separated out and covered independently: MIME body parsing plus HTML sanitising and
  image-blocking in the shared `internal/infrastructure/mailparse` package, the RFC 5322 MIME builder in
  `internal/infrastructure/message`, and the response and UIDL parsing in `pop3`.
- **Windows taskbar overlay** (`taskbar/overlay_windows.go`): the Win32 `ITaskbarList3` calls that draw
  the unread badge are Windows-only and build-tagged; the no-op stub and the pure label formatting are
  covered.
- **Win32 side effects** (`installer/windows.go`): registry writes, shortcut creation and shell-folder
  resolution. These mutate the real machine and are verified by running the installer, not in unit
  tests.
- **Installer GUI** (`installer/`, a Wails app) and its facade: driven by the WebView, verified by
  running the setup program, not by unit tests.
- **Composition root and startup** (the whole `main` package: `main.go` plus the Wails facade files,
  namely `app.go`, the per-feature `*api.go` bindings (send, export, outbox, rules, tags, calendar,
  contacts and scheduling), the background new-mail notifier and reminder scheduler, and the DTO mappers
  and clock) and the **icon tool** (`tools/genicons`): wiring and one-shot programs, verified by the app
  and the build succeeding.
- **A few defensive branches in storage** (a commit failing after a successful transaction, a driver
  read error mid-iteration): not reachably triggerable with a real SQLite file.

## Skippable live integration tests

Two tests connect to real servers and are skipped unless the environment is configured.

IMAP (`internal/infrastructure/imap`):

```
PIGEONPOST_IMAP_HOST=imap.example.com
PIGEONPOST_IMAP_PORT=993
PIGEONPOST_IMAP_EMAIL=you@example.com
PIGEONPOST_IMAP_PASSWORD=your-app-password
go test ./internal/infrastructure/imap/ -run TestSourceLive -v
```

When these variables are unset (the default, including CI), the test calls `t.Skip`, so `go test ./...`
stays fully offline.

## Structural tests

`tests/structural/boundary_test.go` parses the repository and enforces the architecture as executable
rules, not review conventions:

- the domain imports nothing outward and stays free of IO and wall-clock reads;
- the application layer never imports infrastructure or the UI framework;
- no source file exceeds the module-size limit;
- only the composition root wires both the application and infrastructure layers.

A violation fails `go test`, the same as any other test.

## Front-end tests

The React front end has its own suite under `frontend/`, run with Vitest on jsdom:

```
cd frontend
npx vitest run              # run the front-end suite once
npx vitest                  # watch mode
npx vitest run --coverage   # enforce the pure-module coverage gate
```

- **Pure modules gated to 100%.** The pure logic modules (`messageText`, `shortcuts`, `print`,
  `readerFormat`, `composeAddresses`, `accountProviders`, `sidebarDnd`, `calendarModel`, `replyDraft`)
  carry a v8 coverage gate at 100% lines, functions, statements and branches, listed in `vite.config.ts`
  under `coverage.include`. Hooks and components are tested but not gated: a React hook fuses logic with
  framework plumbing, so a blanket 100% there buys brittle tests, not correctness.
- **Structural boundary test.** `src/test/boundary.test.ts` scans the top-level `src/*.ts` modules and
  keeps the gated pure modules pure, the front-end analogue of `boundary_test.go`.
- **Characterization-first.** The `App.tsx` and component decomposition was done test-first: each
  extraction was preceded by a characterization test pinning the behaviour on the un-extracted code, so
  every move was behaviour-preserving by construction. `App.test.tsx` characterizes App at its outer
  interface (what it renders and which `api` calls fire); the one Wails seam (`../api`) and the runtime
  bindings are stubbed while the pure modules run for real.
- The Go `./test.ps1` gate and this front-end suite are separate; run both to verify the whole app.
