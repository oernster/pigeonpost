# PigeonPost Testing

## Philosophy

- Correctness first. The domain and application layers hold the logic that must be right; they
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
is factored into pure functions that are fully tested; the thin I/O shell is excluded and
documented here.

## What is tested and how

| Layer | Test type | Real resource |
|---|---|---|
| `internal/domain` | pure unit | none |
| `internal/application` | unit against hand-written fakes | none |
| `internal/infrastructure/storage` | integration against a real SQLite file | temp dir |
| `internal/infrastructure/message` | unit on the RFC 5322 MIME builder | none |
| `internal/infrastructure/mailparse` | unit on the MIME body parsing, HTML sanitising, URL linkifying (bare and markdown-labelled links, solo-line button marking), image and CSS-background parking, hidden-preheader removal that keeps MJML layout wrappers and the outgoing embedded-image extraction (data: URI to cid part) | none |
| `internal/infrastructure/mailrouter` | unit on the per-protocol dispatch | none |
| `internal/infrastructure/smtp` | none (live send only; MIME building lives in `message`) | n/a |
| `internal/infrastructure/imap` | unit on the source adapter's pure helpers (parsing moved to `mailparse`) | none |
| `internal/infrastructure/pop3` | unit on the response and UIDL parsing; live download excluded | none |
| `internal/infrastructure/ics` | unit on the RFC 5545 codec round-trip, recurrence and scheduling payloads | none |
| `internal/infrastructure/recurrence` | unit on RRULE expansion and truncation | none |
| `internal/infrastructure/vcard` | unit on the vCard codec round-trip | none |
| `internal/infrastructure/csv` | unit on the Outlook CSV codec round-trip | none |
| `internal/infrastructure/caldav` | unit against a local stub CalDAV server (httptest) | local HTTP server |
| `internal/infrastructure/oauth` | unit against stubbed token endpoints (httptest) | local HTTP server |
| `internal/infrastructure/remoteimage` | unit on the SSRF guard and the resolver for both parked images and parked CSS backgrounds, against local stub servers (httptest) and an injected fetch seam | local HTTP server |
| `internal/infrastructure/update` | unit on the GitHub latest-release source via an injected HTTP client | none |
| `internal/infrastructure/keychain` | unit via go-keyring's in-memory mock | none |
| `internal/infrastructure/taskbar` | unit on the pure label formatting; Win32 overlay excluded | none |
| `internal/infrastructure/sound` | unit on the chime synthesis and WAV encoding; the winmm playback call excluded | none |
| `internal/installer` | unit on payload extraction and paths | temp dir |
| `main` (the Wails facade) | unit on its pure helpers only: mailto parsing, attachment decoding, the offline-error translation and the DTO wire shape | none |
| `tests/structural` | AST scan of the source tree | file reads |

## Coverage snapshot

| Package | Coverage | Notes |
|---|---|---|
| internal/domain | 100% | gated |
| internal/application | 100% | gated |
| internal/infrastructure/message | 100% | the RFC 5322 MIME builder (pure): multipart assembly, the inline-image related nesting, outgoing linkify, quoted-printable text parts |
| internal/infrastructure/mailrouter | 100% | per-protocol dispatch (pure) |
| internal/infrastructure/keychain | 100% | account and CalDAV calendar password paths via go-keyring's in-memory mock |
| internal/infrastructure/recurrence | ~97% | RRULE expansion and truncation; a few defensive edges uncovered |
| internal/infrastructure/vcard | ~97% | vCard codec round-trip |
| internal/infrastructure/sound | ~97% | the notification chime's synthesis, normalisation and WAV encoding (pure); only the winmm playback call is excluded |
| internal/infrastructure/oauth | ~95% | token flow against stubbed endpoints; real-network edges excluded |
| internal/infrastructure/update | ~88% | the GitHub latest-release source against an injected fake client; the live-wired constructor and defensive request/read branches excluded |
| internal/infrastructure/mailparse | ~94% | MIME body parsing, HTML sanitising, URL linkifying, image and CSS-background parking (including the font-source exception) and hidden-preheader removal that keeps MJML layout wrappers and mso-hide content (pure); a few defensive decode branches uncovered |
| internal/infrastructure/ics | ~92% | RFC 5545 codec round-trip, recurrence and scheduling payloads |
| internal/infrastructure/remoteimage | ~92% | the SSRF guard and the resolver for parked images and parked CSS backgrounds against stub servers; the live-wired constructor excluded |
| internal/infrastructure/csv | ~95% | Outlook CSV codec round-trip |
| internal/infrastructure/caldav | ~82% | request and parse logic against a stub server; live-server edges and the live-wired writer factory excluded |
| internal/infrastructure/storage | ~79% | logic and error paths covered, including keyset message pagination, the atomic tag-keyword and flag-pending sync writes and the folder-baseline mark; see exclusions |
| internal/infrastructure/pop3 | ~40% | response and UIDL parsing covered; the live dial and download excluded |
| internal/installer | ~22% | extract and paths covered; Win32 side effects excluded |
| internal/infrastructure/imap | ~27% | the source adapter's pure helpers; the wire-to-domain and HTML logic now lives in `mailparse`; live fetch/append plus the IDLE watcher are excluded |
| internal/infrastructure/taskbar | ~17% | the pure label formatting and no-op stub covered; the Windows-only Win32 overlay excluded |
| internal/infrastructure/smtp | 0% | transport is live `Send` only; MIME building lives in `message` |
| main package | ~5% | composition root and the Wails facade, excluded; the covered statements are the package's own pure helpers, which carry unit tests of their own (mailto parsing, attachment decoding, the offline-error translation and the rule DTO's wire shape) |
| installer app, tools/genicons | 0% | GUI and one-shot tooling, excluded |

## Documented exclusions (and why)

- **Live IMAP fetch/append and the IDLE watcher** (`imap/source.go`, `imap/idle.go`), **live POP3
  download** (`pop3/`) and **live SMTP send**
  (`smtp/transport.go`): these dial a real server, authenticate and stream data. They cannot be
  unit-tested without a network, so the IMAP path sits behind a skippable integration test (below). The
  pure logic is separated out and covered independently: MIME body parsing plus HTML sanitising and
  image-blocking in the shared `internal/infrastructure/mailparse` package, the RFC 5322 MIME builder in
  `internal/infrastructure/message`, plus the response and UIDL parsing in `pop3`.
- **Live CalDAV, OAuth and remote-image network paths** (`caldav`, `oauth`, `remoteimage`): the
  request, parse and guard logic is tested against local `httptest` stub servers; the live-wired
  constructors and real-network edges (a real CalDAV server, the browser hand-off, a real image host)
  are excluded.
- **Windows taskbar overlay** (`taskbar/overlay_windows.go`): the Win32 `ITaskbarList3` calls that draw
  the unread badge are Windows-only and build-tagged; the no-op stub and the pure label formatting are
  covered.
- **Notification chime playback** (`sound/play_windows.go`): the `winmm` `PlaySound` call and the
  `Play` entry point that reaches it would make an audible noise on every test run. The synthesis
  behind it is pure and fully covered, down to the WAV header fields and the fades at both ends, so
  what is excluded is the syscall alone.
- **Win32 side effects** (`installer/windows.go`): registry writes, shortcut creation and shell-folder
  resolution. These mutate the real machine and are verified by running the installer, not in unit
  tests.
- **Installer GUI** (`installer/`, a Wails app) and its facade: driven by the WebView, verified by
  running the setup program, not by unit tests.
- **Composition root and startup** (the whole `main` package: `main.go` plus the Wails facade files,
  namely `app.go`, one binding file per feature surface (accounts, mail, folders, send, draft recovery,
  outbox, snooze, tags, rules, templates, calendar, CalDAV, contacts, scheduling, export, `.eml`
  files and updates), the background goroutines (the new-mail notifier, the reminder scheduler, the outbox
  dispatcher and the snooze scheduler) plus the DTO mappers and clock) and the **icon tool**
  (`tools/genicons`): wiring and one-shot programs, verified by the app and the build succeeding. The
  exclusion is the wiring, not the whole package: the pure helpers that do live here carry their own
  unit tests. `rulesapi_test.go` is the one to copy when adding another: it asserts on the DTO's
  marshalled JSON bytes rather than on a hand-written fixture, because a Go nil slice encodes as
  `null` while the generated front-end type declares an array; only the real encoder output shows
  that.
- **A few defensive branches in storage** (a commit failing after a successful transaction, a driver
  read error mid-iteration): not reachably triggerable with a real SQLite file.

## Migration tests

A migration that backfills or rewrites data decides how an existing installation behaves the moment it
updates, which a fresh database can never exercise: applied to an empty schema every backfill is a
no-op and passes vacuously. Those steps are tested against a database built at the version the step
upgrades FROM: apply `migrations[:n]` by hand, set `PRAGMA user_version = n`, write the old-shape rows,
then `Open` the file so the real migration runs and assert on what comes back.

`TestRuleMigrationCarriesLegacyRules` (a flat rule reads back as an equivalent one-condition rule) and
`TestFolderBaselineMigrationMarksExistingFolders` (an existing folder comes out baselined, so the first
sync after updating does not re-arm the destructive-rule exemption) are the two worked examples, each
paired with a fresh-database counterpart proving the other side. The `n` in each is a fixed number with
a comment saying so, never an offset from `schemaVersion`, so a later migration cannot quietly move the
test off the step it exists to cover.

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
  `readerFormat`, `composeAddresses`, `composeAttachment`, `composeIntake`, `recipientSuggest`,
  `autoCollect`, `datePicker`, `accountProviders`, `sidebarDnd`, `calendarModel`, `replyDraft`,
  `caldavAccount`, `unified`, `schedule`, `snooze`, `toolbarNav`, `undoStack`, `editClipboard`,
  `paneLayout`, `emailColors`, `dragScroll`, `optimisticList`, `autoScroll`, `draftEdit`,
  `modalDrag`)
  carry a v8 coverage gate at 100% lines, functions, statements and branches, listed in `vite.config.ts`
  under `coverage.include`. Hooks and components are tested but not gated: a React hook fuses logic with
  framework plumbing, so a blanket 100% there buys brittle tests, not correctness.
- **Structural boundary test.** `src/test/boundary.test.ts` scans the top-level `src/*.ts` modules and
  keeps the gated pure modules pure, the front-end analogue of `boundary_test.go`.
- **Modal layout test.** `src/components/modalLayout.test.ts` scans the dialog source and holds two
  rules: every modal carrying an action row pins it; every pinned modal has something that
  actually scrolls. Both matter because a dialog that scrolls as one block takes its buttons off a
  short window, which is invisible on a large screen and so cannot be left to review. It reads raw
  source through Vite's glob rather than `node:fs`, the same as the boundary test; it asserts it
  found panels at all so a rename cannot turn it into a vacuous pass.
- **The reader's colour treatment** is tested on both halves: its colour arithmetic (colour parsing, alpha
  compositing, sRGB relative luminance, contrast ratio, what counts as a dark background and what counts as
  a background image that actually paints) lives in `src/emailColors.ts` under the gate above, pinned
  directly by `emailColors.test.ts`; the DOM walk that applies those judgements stays in
  `components/emailDarkMode.ts` and its per-region decisions go through the rendered frame in
  `EmailHtmlFrame.test.tsx`, which reads the resulting element styles rather than a stylesheet string
  because the decision depends on each element's own background.
- **Time and frames are driven, never waited on.** The drag auto-scroll and the self-reading help panes
  both run on a clock, so their tests replace it: `useDragAutoScroll.test.tsx` stubs
  `requestAnimationFrame` with a queue it steps one frame at a time; `useAutoScroll.test.tsx` uses
  fake timers. jsdom lays nothing out, so both stub the element's bounds, `scrollTop` and scroll metrics;
  both dispatch drag events by hand: jsdom drops `clientY` and `relatedTarget` from a drag event's
  init, so the fields are set as own properties on a plain `Event` (the same workaround the sidebar drop
  tests use). No test sleeps.
- **Characterisation-first.** The `App.tsx` and component decomposition was done test-first: each
  extraction was preceded by a characterisation test pinning the behaviour on the un-extracted code, so
  every move was behaviour-preserving by construction. `App.test.tsx` characterises App at its outer
  interface (what it renders and which `api` calls fire); the one Wails seam (`../api`) and the runtime
  bindings are stubbed while the pure modules run for real.
- The Go `./test.ps1` gate and this front-end suite are separate; run both to verify the whole app.
