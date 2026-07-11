# PigeonPost: Technical Debt

A standing reference to the project's outstanding technical debt. It records what is still open, weighs whether each item is worth doing and gives the rationale. Every item is a behaviour-preserving internal refactor: nothing here proposes reverting a feature or changing any UI or UX behaviour. Scope is the whole repository (the Go core plus the React front end), read against the documented design and the structural tests.

---

## 1. Open decision: the FetchBody port tuple

`application.MailSource.FetchBody` returns a four-value tuple `(plain, html, invite, attachments, err)` that both adapters return, the router re-threads and `application/body.go` unpacks. A small body struct would remove the destructure-and-re-thread. The return is an application-owned port contract, so its shape (and whether to change it at all) belongs at the application layer rather than in a mechanical refactor. A four-value return is idiomatic Go, not a bug or a drift hazard, so this is genuinely optional.

## 2. Looks like debt, not worth touching

- The three enum parsers (`ParseRole`, `ParseParticipationStatus`, `ParseMethod`) look triplicated but differ in empty-handling (only `ParseMethod` treats an empty string as invalid), so a generic helper would need special-casing that trades three clear functions for a fiddlier abstraction.
- The application error-prefix convention is already consistent within each package (`imap:`, `smtp:`, `folders:`); forcing a single global convention would churn coverage-gated error strings for near-zero benefit.
- The domain `calendar_passthrough` trim guard would change validation for whitespace-only input, so it is a behaviour decision rather than a refactor; it stays unless that behaviour change is intended.
- The remaining discretionary nits: the domain slice-copy idioms and the `close` builtin shadow; the `MailStore` 15-method interface (a cohesive local-cache abstraction worth revisiting only if it grows); the codec-level clones (`generatedID` and `locationOf` across `ics`, `vcard`, `csv` and `recurrence`, whose dedup would couple otherwise-independent packages); the `csv` `[3]` phone-slot literal; the `schema`/`migrations` split; and the installer and genicons cosmetic nits.

## 3. Intentionally left: groupByFolder for DeleteMany / MoveMany

`DeleteMany` and `MoveMany` share batch-by-folder scaffolding. A shared helper looks tempting but is ruled out: collapsing them would change error aggregation from one-per-folder to one overall, an observable behaviour change.

---

## Not debt (do not "fix" these)

These look like candidates but are correct as they stand; changing them would regress or add cost for nothing.

- **The two `tzdata.go` files** (`ics`, `recurrence`). Each is `import _ "time/tzdata"`. The per-package blank import is what keeps `LoadLocation` resolving zones on Windows and keeps each package's tests self-sufficient. Merging them is a regression.
- **The `_other.go` / `_windows.go` / `_darwin.go` / `_linux.go` split** across `taskbar`. The `_other` stubs are pure no-ops (clean build-tag hygiene, zero duplicated logic). The three-way Windows tray split is forced by the 400-line cap, not arbitrary.
- **The Microsoft OAuth endpoints, scopes and client id.** Named consts feeding an overridable `Config`; Microsoft is the sole OAuth provider by design and the tests point at a stub. Correct, not hardcoding.
- **The thin facade's plural DTO mappers and in/out DTO twins.** Idiomatic Go and a defensible evolvability choice.
- **The 400-line-driven file splits** generally (`source_*.go`, `calendar_*.go`, `schema`/`migrations`). These are the module cap doing its job; the resulting files are cohesive.
- **The low-coverage infrastructure packages** (`imap`, `pop3`, `smtp`, `taskbar`). This is the documented, deliberate exclusion of live network and Win32 I/O; the pure logic is factored out and fully covered. Not a coverage gap.
- **The `main` package's untested background logic** (`mailnotifier.go`, the scheduler). Correctly placed at the Wails-coupled facade and excluded by design.
