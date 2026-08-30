# PigeonPost: Technical Debt

A standing reference to the project's outstanding technical debt. It records what is still open, weighs whether each item is worth doing and gives the rationale. Every item is a behaviour-preserving internal refactor: nothing here proposes reverting a feature or changing any UI or UX behaviour. Scope is the whole repository (the Go core plus the React front end), read against the documented design and the structural tests.

The sections below the open item are the standing record of what was weighed and deliberately left alone, so the same ground is not covered again. They carry no numbers, because a number here means an open item and a numbered heading that was not one made this file read as three open items when it held one.

---

## 1. Nine front-end modules are over the module-size limit

The limit is 400 lines. `tests/structural/boundary_test.go` has always enforced it; it parses Go, so the React front end was never held to it by anything. Nearly all of it stayed small anyway while nine grew past it, `App.tsx` furthest by a wide margin:

| Module | Lines |
|---|---|
| `src/App.tsx` | 1355 |
| `src/components/ComposeModal.tsx` | 699 |
| `src/api.ts` | 627 |
| `src/components/EventFormModal.tsx` | 559 |
| `src/components/ContactsModal.tsx` | 491 |
| `src/components/FolderTree.tsx` | 445 |
| `src/components/CalendarModal.tsx` | 436 |
| `src/hooks/useMenus.ts` | 434 |
| `src/components/MessageContextMenu.tsx` | 407 |

The lengths above are what each module holds now; the guard records the length each held when it was written, which is the ceiling an exempt module may not exceed. The guard now exists (`src/test/loc.test.ts`) and holds every other module, with these nine named in an exemption list that may only shrink: a file leaves it when it is split; a file that is exempt while no longer over the limit fails too, so an entry cannot outlive the debt it records. Nothing new can join it. What is open is the splitting itself. Each is a behaviour-preserving decomposition along a concern boundary rather than an arbitrary slice, taken one module at a time and characterisation-first the way the original `App.tsx` decomposition was, so the front-end suite proves each move rather than review doing it. The four modals are alike enough to share an approach.

`App.tsx` is its own unit and is under way. Five concerns have left it: `useMessageExport`, the four managed collections collapsed onto `useManagedCollection`, `useSearch`, `useSplash` and the About and licence panels collapsed onto `useLoadedPanel`. Each is pinned by characterisation tests written against the un-extracted code and proved by planting a violation.

The JSX split has started with `ManagerModals`, the four dialogs over the four managed collections. It works because the collections pass whole: a manager dialog needs nothing threaded through `App`; a fifth would be one entry there plus one `useManagedCollection` call.

The seven `ConfirmDialog` blocks have gone the same way, though not by lifting them behind a component: each of `useMessageActions`, `useBulkActions`, `useFolders`, `useAccounts` and `useOutbox` now returns its own confirmation descriptor, built beside the action it describes; `ConfirmStack` renders the list `App` assembles but does not write. The wording lives in the pure `src/confirmations.ts` under the coverage gate, so it is testable without opening a dialog.

One claim made when this was proposed turned out to be wrong and is withdrawn: it would not make the confirm-before-destroy rule checkable in one place. `ConfirmDialog` is used directly by nine other components (the compose discard, the contacts and rules and templates managers, the calendar's own editors), each a local confirmation belonging to its own dialog, so the list covers the main window's confirmations rather than the application's.

The panes block has moved to `Panes`, which now owns the three-column grid, its CSS variables, the choice between the panes and the splitters that sit on their boundaries. It was taken with the cost measured and accepted rather than on a claim that it was cheap: it takes 18 lines out of `App.tsx` behind a 27-value interface and adds an 88-line module to the tree. It buys cohesion instead of length, since the grid and the splitters were previously stated in two places; it also takes the sidebar wiring out of `App` by passing the underlying values rather than a props object the caller would have to write out again.

Both right-click menus have gone the same way, into `ContextMenus`. The wiring of the two was untested at the `App` level before this, so five characterisation tests now hold it: the gesture that opens each menu, an entry reaching its handler and each menu's dismissal. It takes 16 lines out of `App.tsx` behind a 38-value interface and adds a 138-line module. The gain is that the whole right-click surface is in one place, including the message clipboard both menus read, which is now passed as the one object the hook already returns rather than as four separate values.

What is left in `App.tsx` is composition and the remaining overlays, none of which is one shape repeated. Every collapse-the-duplication move is spent and the two region moves have now been taken, each trading a wide interface for a modest reduction. There is no further unit here worth proposing: what remains is either one-of-a-kind wiring or a move that costs more in interface than it saves in lines. Anyone returning to this should start by asking what the boundary is for rather than what the line count would become.

One candidate was tried and put back: collapsing the five localStorage-backed View preferences onto one hook. The reduction was around fifteen lines and it would have changed three toggles from a functional state updater to a closed-over read; it would also have given `useMenus` a toggle whose identity changes every render where the current one is stable. That is a behaviour change traded for very little, so the characterisation tests for those preferences were kept and the collapse was not made. Anyone returning to it should either keep each toggle's updater form or pin the identity first.

---

## 2. Cached copies of one message are matched by a composite key rather than a real identity

A server can present one message in several mailboxes, which Gmail does for every label: a message labelled Work sits in Work, in the Inbox and in All Mail as three rows with three UIDs. `Store.SetFlag` keeps those rows in step so reading a message in one place does not leave it bold in another; it decides which rows are the same message by `(message_id, date_ms, from_address, subject)` within the one account.

That is an approximation of an identity rather than an identity. Message-ID alone is definitely not one: measured across two real accounts, more than 900 messages share one with another row. Those collisions were inspected rather than assumed and are the same message stored twice, differing only in UID and stored size, so the composite key has not been observed to be wrong on real data. It could still be wrong in principle, with nothing to detect it if it is.

Gmail publishes the exact answer, `X-GM-MSGID`, which is one value per message and identical across every label. It is blocked upstream rather than merely unwritten: `go-imap` v2 has no Gmail extension support, its `beginCommand` is unexported so a custom fetch item cannot be sent, while its FETCH parser returns `unsupported msg-att name` for any attribute it does not recognise, so a response carrying one would fail the whole fetch. Adopting it means patching or forking the mail library, then a schema column and a migration to store it.

Blocked on that library work being worth doing. Until then the composite key stands; the risk it carries is stated here rather than hidden.

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
