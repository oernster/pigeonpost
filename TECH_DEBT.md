# PigeonPost: Technical Debt

A standing reference to the project's outstanding technical debt. It records what is still open, weighs whether each item is worth doing and gives the rationale. Every item is a behaviour-preserving internal refactor: nothing here proposes reverting a feature or changing any UI or UX behaviour. Scope is the whole repository (the Go core plus the React front end), read against the documented design and the structural tests.

The sections below the open item are the standing record of what was weighed and deliberately left alone, so the same ground is not covered again. They carry no numbers, because a number here means an open item and a numbered heading that was not one made this file read as three open items when it held one.

---

## 1. Nine front-end modules are over the module-size limit

The limit is 400 lines. `tests/structural/boundary_test.go` has always enforced it; it parses Go, so the React front end was never held to it by anything. Nearly all of it stayed small anyway (140 of 149 source modules are inside the limit) while nine grew past it, `App.tsx` furthest by a wide margin:

| Module | Lines |
|---|---|
| `src/App.tsx` | 1485 |
| `src/components/ComposeModal.tsx` | 731 |
| `src/api.ts` | 631 |
| `src/components/EventFormModal.tsx` | 559 |
| `src/components/ContactsModal.tsx` | 491 |
| `src/components/FolderTree.tsx` | 453 |
| `src/hooks/useMenus.ts` | 450 |
| `src/components/CalendarModal.tsx` | 436 |
| `src/components/MessageContextMenu.tsx` | 407 |

The guard now exists (`src/test/loc.test.ts`) and holds every other module, with these nine named in an exemption list that may only shrink: a file leaves it when it is split; a file that is exempt while no longer over the limit fails too, so an entry cannot outlive the debt it records. Nothing new can join it. What is open is the splitting itself. Each is a behaviour-preserving decomposition along a concern boundary rather than an arbitrary slice, taken one module at a time and characterisation-first the way the original `App.tsx` decomposition was, so the front-end suite proves each move rather than review doing it. The four modals are alike enough to share an approach.

`App.tsx` is its own unit and is under way. Six concerns have left it: `useMessageExport`, the four managed collections collapsed onto `useManagedCollection`, `useUndoSend`, `useSearch`, `useSplash` and the About and licence panels collapsed onto `useLoadedPanel`. Each is pinned by characterisation tests written against the un-extracted code and proved by planting a violation.

The JSX split has started with `ManagerModals`, the four dialogs over the four managed collections. It works because the collections pass whole: a manager dialog needs nothing threaded through `App`; a fifth would be one entry there plus one `useManagedCollection` call.

The rest of the overlay stack does not pass whole, which is the open question rather than the remaining lines. Seven `ConfirmDialog` blocks are one shape repeated, yet each draws its gate, its busy flag and its action from a different hook, so lifting them behind one component would thread twenty-odd values across a new boundary and buy a shorter file for a wider interface. The move that pays is the other direction: each of `useMessageActions`, `useBulkActions`, `useFolders`, `useAccounts` and `useOutbox` exposing its own confirmation descriptor, so `App` renders a list it does not assemble. That also makes the house rule that every destructive action carries a confirmation checkable in one place rather than by reading the JSX. It touches five hooks and should be its own unit.

One candidate was tried and put back: collapsing the five localStorage-backed View preferences onto one hook. The reduction was around fifteen lines and it would have changed three toggles from a functional state updater to a closed-over read; it would also have given `useMenus` a toggle whose identity changes every render where the current one is stable. That is a behaviour change traded for very little, so the characterisation tests for those preferences were kept and the collapse was not made. Anyone returning to it should either keep each toggle's updater form or pin the identity first.

---

## 2. The api mock is hand-written in 24 of the 25 test files that use it

`src/test/apiMock.ts` builds the `../api` module mock from the real module's own keys, so a method the
app reaches that no spy declares fails the test by name instead of throwing a TypeError into the nearest
catch and passing silently. `App.test.tsx` uses it. The other 24 files that mock the api still list their
stubs by hand and carry the same hole: `AccountSetupModal`, `BottomBar`, `CalendarModal`, the four
`ComposeModal` suites, both `ContactsModal` suites, `InviteCard`, `MessageBodyView`, `Reader`,
`RuleManagerModal`, `Sidebar`, `ThreadView`, plus the `useBulkActions`, `useComposeLauncher`,
`useConversation`, `useFolderPagination`, `useFolders`, `useMessageActions`, `useMessageClipboard`,
`useRemoteImages` and `useUndoRedo` hook suites.

Each is a small mechanical change (swap the factory, add the `afterEach` assertion) but not a blind one:
switching `App.test.tsx` over failed every test in the file until `snoozedCount` was stubbed, so each
file should expect to surface its own holes and each hole needs a stub whose value is right for that
test rather than a reflexive `undefined`. Worth doing per file as each is next touched, rather than as
one sweep that would mix two dozen unrelated behaviour questions into a single change.

---

## Looks like debt, not worth touching

- The `application.MailSource.FetchBody` four-value return `(plain, html, invite, attachments, err)` could be reshaped into a body struct to save the destructure-and-re-thread; a four-value return is idiomatic Go and the port shape is fine as it stands, so it is left.
- The three enum parsers (`ParseRole`, `ParseParticipationStatus`, `ParseMethod`) look triplicated but differ in empty-handling (only `ParseMethod` treats an empty string as invalid), so a generic helper would need special-casing that trades three clear functions for a fiddlier abstraction.
- The application error-prefix convention is already consistent within each package (`imap:`, `smtp:`, `folders:`); forcing a single global convention would churn coverage-gated error strings for near-zero benefit.
- The domain `calendar_passthrough` trim guard would change validation for whitespace-only input, so it is a behaviour decision rather than a refactor; it stays unless that behaviour change is intended.
- **The nil-slice-to-JSON hazard on outbound DTOs.** A Go nil slice encodes as `null` rather than `[]` and the front end's generated types declare arrays; reading a length off `null` throws during render; with no error boundary above the app React unmounts the whole window rather than one dialog. This is a real failure mode (it took the window down when `RuleDTO.AccountIDs` shipped nil) but it is not open debt: every plural mapper already builds with `make([]T, 0, len(...))`, which cannot be nil; the two fields assigned straight from a domain accessor (`MessageDTO.TagColours` and `RuleDTO.AccountIDs`) each carry an explicit nil guard at their construction site, the second pinned by a test asserting on the marshalled bytes. A general guard was considered and left: enforcing it by reflection would flag every slice field on a zero-valued struct, since the safety comes from the mapper rather than the type; an AST rule proving each mapper uses `make` is fiddly for a convention already followed everywhere.
- The remaining discretionary nits: the domain slice-copy idioms and the `close` builtin shadow; the `MailStore` 24-method interface (it was 17 when this entry was first written; the rules, folder-baseline and conversation-lookup work took it to 24, so the growth that was to trigger a rethink has happened and was weighed: it stays, because the methods are one cohesive local-cache abstraction and splitting it would churn every implementation and every hand-written fake for a tidier shape rather than a working difference); the codec-level clones (`generatedID` and `locationOf` across `ics`, `vcard`, `csv` and `recurrence`, whose dedup would couple otherwise-independent packages); the `csv` `[3]` phone-slot literal; the `schema`/`migrations` split; and the installer and genicons cosmetic nits.

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
