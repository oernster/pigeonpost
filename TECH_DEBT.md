# PigeonPost: Technical Debt

A standing reference to the project's outstanding technical debt, not a work order. It lists what is still open, weighs whether each item is worth doing and records the rationale. Nothing here proposes reverting a feature or changing any UI or UX behaviour: every item is a behaviour-preserving internal refactor.

- **Scope:** the whole repository (Go core plus the React front end), read against the documented design and the structural tests.
- **Status:** the backend cleanup from the original review has landed; the whole Go module is staticcheck-clean and the 100% domain and application coverage gate holds. What is left is collected below.
- **Last updated:** 2026-07-10.

---

## What remains, in short

- **One backend decision left**: whether to reshape the `FetchBody` port tuple into a struct. The actionable backend cleanup is otherwise complete. See section 3.
- **One item deliberately left**: `groupByFolder` for `DeleteMany`/`MoveMany`, because it would change observable error aggregation. See section 4.
- **The front end**, almost entirely `App.tsx`: a 2,519-line god component that is the one genuine concentration of debt. Deferred behind a front-end test net. Nothing in `frontend/` has been touched. See section 5.

---

## 1. Bottom line

The early architectural decisions have compounded well, for a structural reason rather than luck: the invariants that matter are enforced by `tests/structural/boundary_test.go`, not left to discipline. Layer direction, domain purity, the 400-line module cap and the single composition root are executable rules, so the classes of debt they cover cannot accumulate. Beyond the enforced invariants the discipline is evident: named constants, a thin facade, immutable value objects, consistent `%w` error wrapping, no dead code to speak of and no TODO or FIXME markers anywhere in the Go tree.

The backend is now essentially clean: the near-duplicate blocks and magic numbers that were the original findings have been collapsed or named behind the coverage gate, leaving only one optional port-shape decision (the `FetchBody` tuple). None of it is structural rot.

The invariants reached every part of the code except one, which is where the material debt sits: **the React front end, almost entirely `App.tsx`**. The structural scan explicitly excludes `frontend/`, there is no 400-line cap there and there are no automated front-end tests at all, so the front end grew a 2,519-line god component holding 66 pieces of state while the Go core stayed clean. That asymmetry is the single most useful finding in this document.

---

## 2. How to read the recommendations

- **Verdict** is one of: **Do** (clear win, low risk), **Consider** (real value but weigh the effort or risk), **Leave** (looks like debt, is not worth touching).
- Two constraints frame every recommendation. No feature is ever reverted and no UI or UX behaviour changes. The safety net is also asymmetric: the Go core is protected by the 100% domain and application coverage gate plus the structural tests, so backend refactors are verifiable and safe, while the front end has no automated tests, so any change there rests on manual verification alone. That is why the safe work is nearly all in the backend and why `App.tsx`, despite being the biggest single target, is the riskiest to touch.

---

## 3. Remaining backend items

The actionable backend cleanup is complete. What is left is one boundary decision and a set of items judged not worth touching.

- **The `FetchBody` tuple (an open decision).** `application.MailSource.FetchBody` returns a four-value tuple `(plain, html, invite, attachments, err)` that both adapters return, the router re-threads and `application/body.go` unpacks. A small body struct would remove the destructure-and-re-thread; the return is an application-owned port contract, so its shape (and whether to change it at all) belongs at the application layer rather than in a mechanical refactor. A four-value return is idiomatic Go, not a bug or a drift hazard, so it is genuinely optional.

**Leave (looks like debt, is not worth touching):**
- The three enum parsers (`ParseRole`, `ParseParticipationStatus`, `ParseMethod`) look triplicated but differ in empty-handling (only `ParseMethod` treats an empty string as invalid), so a generic helper would need special-casing that trades three clear functions for a fiddlier abstraction.
- The application error-prefix convention is already consistent within each package (`imap:`, `smtp:`, `folders:`); forcing a single global convention would churn coverage-gated error strings for near-zero benefit.
- The domain `calendar_passthrough` trim guard would change validation for whitespace-only input, so it is a behaviour decision rather than a refactor; it stays unless that behaviour change is intended.
- The remaining discretionary nits: the domain slice-copy idioms and the `close` builtin shadow; the `MailStore` 15-method interface (a cohesive local-cache abstraction worth revisiting only if it grows); the codec-level clones (`generatedID` and `locationOf` across `ics`, `vcard`, `csv` and `recurrence`, whose dedup would couple otherwise-independent packages); the `csv` `[3]` phone-slot literal; the `schema`/`migrations` split; and the installer and genicons cosmetic nits.

---

## 4. Intentionally left (not a task)

- **`groupByFolder` for `DeleteMany`/`MoveMany`.** The two share batch-by-folder scaffolding; a shared helper looks tempting but is ruled out, because collapsing them would change error aggregation from one-per-folder to one overall, an observable behaviour change.

---

## 5. The front end (the one material concentration of debt)

#### A1. `App.tsx` (2,519 lines): the headline

**State:** one component holds **66 `useState` declarations** and about **95** further hooks (`useEffect`, `useCallback`, `useMemo`, `useRef`). It owns every concern: data loading for accounts, folders, messages, tags, rules, contacts and events; account and folder CRUD; single and bulk message operations; multi-select with Ctrl and Shift; compose orchestration (reply, reply-all, forward, attach, restore); search; the outbox; reader tabs; the reading-pane toggle; printing; roughly fifteen modal flags; a ~230-line global keyboard effect with a ~28-entry dependency array; about 140 lines of inline menu configuration; and a ~390-line render tree.

This is not spaghetti. The comments are excellent, the pure helpers are already lifted to module scope (`escapeHtml`, `subjectWithPrefix`, `emlFilename`, `neighbourAfterRemoval`, `matchesShortcut`, the focus-ring helpers) and the view is genuinely decomposed into child components. What was never extracted is the **state and the orchestration**. Two duplication patterns are visible inside it: the "apply an optimistic change to every list" block (`setMessages` then `setSearchResults` then `setTabs` then `setSelectedMessage`) appears in five handlers; the "remove ids from every list" block appears in four single-item handlers even though `removeIdsFromLists` already generalises it for the bulk path.

**Refactor pros:**
- Cuts the single largest maintenance surface in the codebase into testable units (custom hooks such as `useAccounts`, `useFolders`, `useMessageActions`, `useSelection`, `useCompose`, `useKeyboardNav`, `useWailsEvents`).
- Shrinks the giant dependency arrays and the re-render blast radius, so a change to one concern stops forcing the whole app to reason about the rest.
- Collapses the fan-out-to-lists duplication into one helper, removing the drift risk where a new list is added and one handler forgets it.
- Makes the intricate keyboard and selection logic isolatable and unit-testable for the first time.

**Refactor cons:**
- **This is the highest-risk change in the review.** The keyboard, focus-ring, multi-select and reading-pane behaviours are subtle, interdependent and exactly the UX that must not change, with **no automated front-end tests** to catch a regression.
- The surface is large: a full extraction touches most of the file and is hard to review as one diff.
- Behaviour-preserving refactors of stateful React are easy to get subtly wrong (stale closures, effect timing, dependency arrays).

**Verdict: Consider, staged and behind tests.** A big-bang rewrite is the wrong approach; the safe path is incremental and test-guarded: (1) add a front-end test runner and characterization tests for the load-bearing flows (keyboard navigation, multi-select ranges, delete and move, compose pre-fill) so there is a net; (2) extract the lowest-risk, purely-derived pieces first (the menu definitions to a module, the list-mutation helpers into one function); (3) only then lift cohesive state clusters into custom hooks one at a time, verifying the UI after each. Each step is independently shippable and reversible. This is the one item where the effort and risk are real enough that "judiciously, if at all" is the right frame: the value is high, though it is the only place a slip could break the UX.

#### A2. `App.css` (3,070 lines)

Large but genuinely disciplined: **zero `!important`**, only **6 hardcoded hex colours** against **325 `var(--…)`** references and sectioned feature-by-feature with descriptive comments (reading pane, focus ring, menus, calendar views, recurrence editor, contacts). It is not rotting; the only debt is that one file carries the whole app's styling.

- **Refactor pros:** splitting into per-feature stylesheets (or CSS modules) would improve navigability and make ownership of a feature's styles obvious.
- **Refactor cons:** CSS is order and specificity sensitive, so splitting risks subtle cascade changes to a UI that must not change, for a modest navigability gain; the file is already comment-navigable. Churn without much payoff.
- **Verdict: Leave** (optional low-priority split at most). If `App.tsx` is broken into feature hooks, co-locating each feature's CSS at that point is the natural, low-risk moment to do it.

#### A3. `CalendarModal.tsx` (1,061 lines)

The second-largest front-end file but well organised: named constants throughout (`DAYS_IN_WEEK`, `REMINDER_PRESETS`, `DEFAULT_REMINDER_MINUTES`, weekday and month tables, attendee-status labels), pure helpers (`extractUrls`, `meetingProvider`), typed form interfaces and delegation to `CalendarTimeGrid`, `RecurrenceEditor`, `ScopeChooser` and `PickerButton`. Its size is inherent: month, week and day views plus an event editor plus recurrence plus attendees plus timezones.

- **Refactor pros:** the event-editor form could split from the calendar grid and views, giving two smaller, single-purpose components.
- **Refactor cons:** calendar state (selected date, view mode, the open event) is genuinely shared across those pieces, so the split adds prop-threading; moderate risk against no automated tests; modest benefit since the file is already constant-clean and sub-component-backed.
- **Verdict: Consider (low priority).** A good candidate only once a front-end test net exists and `App.tsx` itself has been dealt with.

#### A4. The other front-end components

`Sidebar` (705), `AccountSetupModal` (554), `Reader` (523), `ComposeModal` (520), `api.ts` (415), `ContactsModal` (396), `RecurrenceEditor` (335), `MessageContextMenu` (327), `Menu` (308) and the fifteen-odd small dialogs. These are the correctly-scoped extractions, each a single responsibility receiving props. Their existence is precisely why `App.tsx` is 2,519 lines rather than 6,000. `api.ts` is a flat typed wrapper mirroring the Go facade one-to-one, which is the expected shape.

- **Verdict: Leave.** This is the decomposition done right. Pros of touching them: negligible. Cons: churn for no benefit.

#### A5. The front-end test gap (a Gap, not a refactor)

There are no `.test`/`.spec` files and no component-test setup, though the design plan lists front-end component tests as the strategy. This is not itself "debt" in the refactor sense; it is the reason `App.tsx` is risky to touch, so it belongs here.

- **Recommendation:** a front-end test harness plus characterization tests for the keyboard, selection, delete/move and compose flows are the **prerequisite** for A1, not a separate nicety. They convert the riskiest refactor in the codebase into a safe one.

---

## 6. Explicitly not debt (do not "fix" these)

These look like candidates but are correct as they stand; changing them would regress or add cost for nothing.

- **The two `tzdata.go` files** (`ics`, `recurrence`). Each is `import _ "time/tzdata"`. The per-package blank import is what keeps `LoadLocation` resolving zones on Windows and keeps each package's tests self-sufficient. Merging them is a regression.
- **The `_other.go` / `_windows.go` / `_darwin.go` / `_linux.go` split** across `taskbar`. The `_other` stubs are pure no-ops (clean build-tag hygiene, zero duplicated logic). The three-way Windows tray split is forced by the 400-line cap, not arbitrary.
- **The Microsoft OAuth endpoints, scopes and client id.** Named consts feeding an overridable `Config`; Microsoft is the sole OAuth provider by design and the tests point at a stub. Correct, not hardcoding.
- **The thin facade's plural DTO mappers and in/out DTO twins.** Idiomatic Go and a defensible evolvability choice.
- **The 400-line-driven file splits** generally (`source_*.go`, `calendar_*.go`, `schema`/`migrations`). These are the module cap doing its job; the resulting files are cohesive.
- **The low-coverage infrastructure packages** (`imap`, `pop3`, `smtp`, `taskbar` at 0 to 44%). This is the documented, deliberate exclusion of live network and Win32 I/O; the pure logic is factored out and fully covered. Not a coverage gap.
- **The `main` package's untested background logic** (`mailnotifier.go`, the scheduler). Correctly placed at the Wails-coupled facade and excluded by design.

---

## 7. Closing

The structural evidence is consistent: the decisions compounded because the important invariants were made executable, so they held under speed. The backend is not clean by accident; it is clean because the gates make the bad states unrepresentable. The actionable backend tidy-ups are now done; what remains is one optional port-shape decision and the front end.

The one genuine concentration of debt is where the invariants never reached: the ungated, untested front end, almost entirely `App.tsx`. It is the file to weigh most carefully, both because it is the highest-value and highest-risk target and because it is the one place a careless change could break the UX. It should be handled last, backed by tests first and staged.
