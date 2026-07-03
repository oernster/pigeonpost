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
| `internal/infrastructure/smtp` | unit on the MIME builder | none |
| `internal/infrastructure/imap` | unit on the wire-to-domain mapping | none |
| `internal/infrastructure/keychain` | unit via go-keyring's in-memory mock | none |
| `internal/installer` | unit on payload extraction and paths | temp dir |
| `tests/structural` | AST scan of the source tree | file reads |

## Coverage snapshot

| Package | Coverage | Notes |
|---|---|---|
| internal/domain | 100% | gated |
| internal/application | 100% | gated |
| internal/infrastructure/keychain | 100% | go-keyring mock |
| internal/infrastructure/storage | ~88% | logic and error paths covered; see exclusions |
| internal/infrastructure/smtp | ~44% | `BuildMIME` is 100%; live `Send` excluded |
| internal/infrastructure/imap | ~42% | mapping is 100%; live fetch excluded |
| internal/installer | ~49% | extract and paths covered; Win32 side effects excluded |
| main package, installer app, tools/genicons | 0% | composition root, GUI and tooling, excluded |

## Documented exclusions (and why)

- **Live IMAP fetch** (`imap/source.go`) and **live SMTP send** (`smtp/transport.go`): these dial a
  real server, authenticate and stream data. They cannot be unit-tested without a network, so they
  sit behind skippable integration tests (below). Their pure mapping and message-building logic is
  separated out and covered to 100%.
- **Win32 side effects** (`installer/windows.go`): registry writes, shortcut creation and shell-folder
  resolution. These mutate the real machine and are verified by running the installer, not in unit
  tests.
- **Installer GUI** (`installer/`, a Wails app) and its facade: driven by the WebView, verified by
  running the setup program, not by unit tests.
- **Composition root and startup** (`main.go`, `app.go`, `send.go`, `clock.go`) and the **icon tool**
  (`tools/genicons`): wiring and one-shot programs, verified by the app and the build succeeding.
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
