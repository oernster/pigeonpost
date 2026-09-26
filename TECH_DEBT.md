# PigeonPost: Technical Debt

A standing reference to the project's outstanding technical debt. It records what is still open, weighs whether each item is worth doing and gives the rationale. Every item is a behaviour-preserving internal refactor: nothing here proposes reverting a feature or changing any UI or UX behaviour. Scope is the whole repository (the Go core plus the React front end), read against the documented design and the structural tests.

The sections below the open items are the standing record of what was weighed and deliberately left alone, so the same ground is not covered again. They carry no numbers, because a number here means an open item and a numbered heading that was not one made this file read as three open items when it held one.

---

## 1. Cached copies of one message are matched by a composite key rather than a real identity

A server can present one message in several mailboxes, which Gmail does for every label: a message labelled Work sits in Work, in the Inbox and in All Mail as three rows with three UIDs. `Store.SetFlag` keeps those rows in step so reading a message in one place does not leave it bold in another; it decides which rows are the same message by `(message_id, date_ms, from_address, subject)` within the one account.

That is an approximation of an identity rather than an identity. Message-ID alone is definitely not one: measured across two real accounts, more than 900 messages share one with another row. Those collisions were inspected rather than assumed and are the same message stored twice, differing only in UID and stored size, so the composite key has not been observed to be wrong on real data. It could still be wrong in principle, with nothing to detect it if it is.

Gmail publishes the exact answer, `X-GM-MSGID`, which is one value per message and identical across every label. It is blocked upstream rather than merely unwritten: `go-imap` v2 has no Gmail extension support, its `beginCommand` is unexported so a custom fetch item cannot be sent, while its FETCH parser returns `unsupported msg-att name` for any attribute it does not recognise, so a response carrying one would fail the whole fetch. Adopting it means patching or forking the mail library, then a schema column and a migration to store it.

Blocked on that library work being worth doing. Until then the composite key stands; the risk it carries is stated here rather than hidden.

---

## Looks like debt, not worth touching

- **`App.tsx` is over the module-size limit.** The size guard (`src/test/loc.test.ts`) holds every other front-end module to 400 lines and keeps `App.tsx` on its exemption list at its recorded ceiling, so it cannot grow. What is left in it is composition: one-of-a-kind wiring and the remaining overlays, none of it one shape repeated. Every move that collapsed a duplication has been made; the region moves available traded a wide interface for a modest reduction, so a further split would cost more in interface than it saves in lines. One candidate, collapsing the five localStorage-backed View preferences onto one hook, was tried and put back: it changed three toggles from a functional updater to a closed-over read and gave `useMenus` a toggle whose identity changes every render. Anyone returning to it should ask what a boundary is for rather than what the line count would become.
- **A folder's summaries are fetched in one FETCH.** One message whose body structure the client cannot decode ends the response, so `Source.FetchMessages` falls back to re-fetching the folder without structures and the paperclip is lost for every message in it rather than one. Fetching in batches would confine that loss and stop a large folder being collected into memory as one slice; it would be a change to the adapter alone, with `TestFetchMessagesFallsBackWhenBodyStructureUnreadable` already pinning the fallback. It is left because the loss it guards against was measured and not found: every folder of six real accounts, fetched live the way the adapter fetches it, came to 150 folders holding 83,985 messages (one more was refused by its server for an unrelated sign-in reason) and the fallback fired on none of them. One reading is not a guarantee. Revisit it only if the fallback is seen to fire or a whole-folder slice is shown to cost memory that matters.

- The `application.MailSource.FetchBody` four-value return `(plain, html, invite, attachments, err)` could be reshaped into a body struct to save the destructure-and-re-thread; a four-value return is idiomatic Go and the port shape is fine as it stands, so it is left.
- The three enum parsers (`ParseRole`, `ParseParticipationStatus`, `ParseMethod`) look triplicated but differ in empty-handling (only `ParseMethod` treats an empty string as invalid), so a generic helper would need special-casing that trades three clear functions for a fiddlier abstraction.
- The application error-prefix convention is already consistent within each package (`imap:`, `smtp:`, `folders:`); forcing a single global convention would churn coverage-gated error strings for near-zero benefit.
- The domain `calendar_passthrough` trim guard would change validation for whitespace-only input, so it is a behaviour decision rather than a refactor; it stays unless that behaviour change is intended.
- **The nil-slice-to-JSON hazard on outbound DTOs.** A Go nil slice encodes as `null` rather than `[]` and the front end's generated types declare arrays; reading a length off `null` throws during render; with no error boundary above the app React unmounts the whole window rather than one dialog. This is a real failure mode (it took the window down when `RuleDTO.AccountIDs` shipped nil) but it is not open debt: every plural mapper already builds with `make([]T, 0, len(...))`, which cannot be nil; the two fields assigned straight from a domain accessor (`MessageDTO.TagColours` and `RuleDTO.AccountIDs`) each carry an explicit nil guard at their construction site, the second pinned by a test asserting on the marshalled bytes. A general guard was considered and left: enforcing it by reflection would flag every slice field on a zero-valued struct, since the safety comes from the mapper rather than the type; an AST rule proving each mapper uses `make` is fiddly for a convention already followed everywhere.
- The remaining discretionary nits: the domain slice-copy idioms and the `close` builtin shadow; the `MailStore` 25-method interface (it was 17 when this entry was first written; the rules, folder-baseline and conversation-lookup work took it to 24 and the bulk mark-read's batched flag write (`SetFlagMany`) to 25, so the growth that was to trigger a rethink has happened and was weighed: it stays, because the methods are one cohesive local-cache abstraction and splitting it would churn every implementation and every hand-written fake for a tidier shape rather than a working difference); the codec-level clones (`generatedID` and `locationOf` across `ics`, `vcard`, `csv` and `recurrence`, whose dedup would couple otherwise-independent packages); the `csv` `[3]` phone-slot literal; the `schema`/`migrations` split; and the installer and genicons cosmetic nits.

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
