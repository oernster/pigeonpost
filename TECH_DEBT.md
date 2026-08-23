# PigeonPost: Technical Debt

A standing reference to the project's outstanding technical debt. It records what is still open, weighs whether each item is worth doing and gives the rationale. Every item is a behaviour-preserving internal refactor: nothing here proposes reverting a feature or changing any UI or UX behaviour. Scope is the whole repository (the Go core plus the React front end), read against the documented design and the structural tests.

**There is no open technical debt.** The sections below are the standing record of what was weighed and deliberately left alone, so the same ground is not covered again. They carry no numbers, because a number here means an open item and a numbered heading that was not one made this file read as three open items when it held one.

---

## Looks like debt, not worth touching

- The `application.MailSource.FetchBody` four-value return `(plain, html, invite, attachments, err)` could be reshaped into a body struct to save the destructure-and-re-thread; a four-value return is idiomatic Go and the port shape is fine as it stands, so it is left.
- The three enum parsers (`ParseRole`, `ParseParticipationStatus`, `ParseMethod`) look triplicated but differ in empty-handling (only `ParseMethod` treats an empty string as invalid), so a generic helper would need special-casing that trades three clear functions for a fiddlier abstraction.
- The application error-prefix convention is already consistent within each package (`imap:`, `smtp:`, `folders:`); forcing a single global convention would churn coverage-gated error strings for near-zero benefit.
- The domain `calendar_passthrough` trim guard would change validation for whitespace-only input, so it is a behaviour decision rather than a refactor; it stays unless that behaviour change is intended.
- **The nil-slice-to-JSON hazard on outbound DTOs.** A Go nil slice encodes as `null` rather than `[]` and the front end's generated types declare arrays; reading a length off `null` throws during render; with no error boundary above the app React unmounts the whole window rather than one dialog. This is a real failure mode (it took the window down when `RuleDTO.AccountIDs` shipped nil) but it is not open debt: every plural mapper already builds with `make([]T, 0, len(...))`, which cannot be nil; the two fields assigned straight from a domain accessor (`MessageDTO.TagColours` and `RuleDTO.AccountIDs`) each carry an explicit nil guard at their construction site, the second pinned by a test asserting on the marshalled bytes. A general guard was considered and left: enforcing it by reflection would flag every slice field on a zero-valued struct, since the safety comes from the mapper rather than the type; an AST rule proving each mapper uses `make` is fiddly for a convention already followed everywhere.
- The remaining discretionary nits: the domain slice-copy idioms and the `close` builtin shadow; the `MailStore` 23-method interface (it was 17 when this entry was first written and the rules and folder-baseline work took it to 23, so the growth that was to trigger a rethink has happened and was weighed: it stays, because the methods are one cohesive local-cache abstraction and splitting it would churn every implementation and every hand-written fake for a tidier shape rather than a working difference); the codec-level clones (`generatedID` and `locationOf` across `ics`, `vcard`, `csv` and `recurrence`, whose dedup would couple otherwise-independent packages); the `csv` `[3]` phone-slot literal; the `schema`/`migrations` split; and the installer and genicons cosmetic nits.

## Intentionally left: groupByFolder for DeleteMany / MoveMany

`DeleteMany` and `MoveMany` share batch-by-folder scaffolding. A shared helper looks tempting but is ruled out: collapsing them would change error aggregation from one-per-folder to one overall, an observable behaviour change.

---

## Not debt (do not "fix" these)

These look like candidates but are correct as they stand; changing them would regress or add cost for nothing.

- **The two `tzdata.go` files** (`ics`, `recurrence`). Each is `import _ "time/tzdata"`. The per-package blank import is what keeps `LoadLocation` resolving zones on Windows and keeps each package's tests self-sufficient. Merging them is a regression.
- **The `_other.go` / `_windows.go` / `_darwin.go` / `_linux.go` split** across `taskbar` and `sound`. The `_other` stubs are pure no-ops (clean build-tag hygiene, zero duplicated logic). The three-way Windows tray split is forced by the 400-line cap, not arbitrary.
- **The Microsoft OAuth endpoints, scopes and client id.** Named consts feeding an overridable `Config`; Microsoft is the sole OAuth provider by design and the tests point at a stub. Correct, not hardcoding.
- **The thin facade's plural DTO mappers and in/out DTO twins.** Idiomatic Go and a defensible evolvability choice.
- **The 400-line-driven file splits** generally (`source_*.go`, `calendar_*.go`, `schema`/`migrations`). These are the module cap doing its job; the resulting files are cohesive.
- **The low-coverage infrastructure packages** (`imap`, `pop3`, `smtp`, `taskbar`). This is the documented, deliberate exclusion of live network and Win32 I/O; the pure logic is factored out and fully covered. Not a coverage gap.
- **Test files above the 400-line module cap.** The structural guard skips `_test.go` by design, so a long table-driven suite is not a violation. Splitting one to satisfy a cap it is deliberately outside of would scatter a coherent set of cases for nothing.
- **The `main` package's untested background logic** (`mailnotifier.go`, the scheduler). Correctly placed at the Wails-coupled facade and excluded by design.
