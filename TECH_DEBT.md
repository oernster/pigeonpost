# PigeonPost: Technical Debt

A standing reference to the project's outstanding technical debt, not a work order. It lists what is still open, weighs whether each item is worth doing and records the rationale. Nothing here proposes reverting a feature or changing any UI or UX behaviour: every item is a behaviour-preserving internal refactor.

- **Scope:** the whole repository (Go core plus the React front end), read against the documented design and the structural tests.
- **Status:** the backend cleanup and the front-end decomposition have both landed. The Go module is staticcheck-clean with the 100% domain and application gate holding; the React front end is now decomposed (`App.tsx` broken into hooks and focused components behind a Vitest test net, `App.css` split into a per-concern manifest). What little is left is collected below.
- **Last updated:** 2026-07-11.

---

## What remains, in short

- **One backend decision left**: whether to reshape the `FetchBody` port tuple into a struct. The actionable backend cleanup is otherwise complete. See section 3.
- **One item deliberately left**: `groupByFolder` for `DeleteMany`/`MoveMany`, because it would change observable error aggregation. See section 4.
- **The front end is now decomposed** (it was the one genuine concentration of debt). `App.tsx` went from a 2,519-line god component to 986 lines across fourteen custom hooks and four focused components, on a Vitest and jsdom test net with a coverage gate on the pure modules. `App.css` went from 3,070 lines to a per-concern `@import` manifest. Nothing material remains open on the front end. See section 5.

---

## 1. Bottom line

The early architectural decisions have compounded well, for a structural reason rather than luck: the invariants that matter are enforced by `tests/structural/boundary_test.go`, not left to discipline. Layer direction, domain purity, the 400-line module cap and the single composition root are executable rules, so the classes of debt they cover cannot accumulate. Beyond the enforced invariants the discipline is evident: named constants, a thin facade, immutable value objects, consistent `%w` error wrapping, no dead code to speak of and no TODO or FIXME markers anywhere in the Go tree.

The backend is now essentially clean: the near-duplicate blocks and magic numbers that were the original findings have been collapsed or named behind the coverage gate, leaving only one optional port-shape decision (the `FetchBody` tuple). None of it is structural rot.

The invariants reached every part of the code except one, which is where the material debt used to sit: **the React front end, almost entirely `App.tsx`**. The structural scan excluded `frontend/`, there was no 400-line cap there and there were no automated front-end tests, so the front end had grown a 2,519-line god component holding 66 pieces of state while the Go core stayed clean. That asymmetry, the single most useful finding in this document, has since been closed: the front end now carries its own Vitest test net (a v8 coverage gate on the pure modules plus a structural boundary test), and `App.tsx` was decomposed staged and test-guarded down to 986 lines. See section 5.

---

## 2. How to read the recommendations

- **Verdict** is one of: **Do** (clear win, low risk), **Consider** (real value but weigh the effort or risk), **Leave** (looks like debt, is not worth touching).
- Two constraints frame every recommendation. No feature is ever reverted and no UI or UX behaviour changes. The safety net was, at the time of the original review, asymmetric: the Go core is protected by the 100% domain and application coverage gate plus the structural tests, so backend refactors are verifiable and safe, while the front end had no automated tests, so any change there rested on manual verification alone. That asymmetry is why the safe work was nearly all in the backend and why `App.tsx` was the riskiest target. It has since been closed by adding the front-end test net first (see section 5), which is exactly how the `App.tsx` decomposition was then done safely.

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

## 5. The front end (RESOLVED, 2026-07-11)

This was the one material concentration of debt: the ungated, untested React front end, almost entirely a 2,519-line `App.tsx` holding 66 `useState` declarations and orchestrating every concern (data loading, account and folder CRUD, single and bulk message operations, multi-select, compose, search, the outbox, reader tabs, ~fifteen modal flags, a ~230-line global keyboard effect and a ~390-line render tree). It has since been paid down exactly as the original verdict prescribed, staged and test-first, with no UI or UX change.

#### A0. The prerequisite, done first: a front-end test net

`frontend/` gained a Vitest + jsdom harness with a v8 coverage gate on the pure logic modules and a structural boundary test that scans `src/*.ts` (the same role `boundary_test.go` plays for Go). Every decomposition step was preceded by a characterization test pinning the behaviour on the un-extracted code, so each move was behaviour-preserving by construction. This converted the riskiest refactor in the codebase into a verifiable one.

#### A1. `App.tsx`, 2,519 to 986 lines (RESOLVED)

The state and orchestration that had never been extracted were lifted into fourteen custom hooks (`useMessageStore`, `useSelection`, `useMessageActions`, `useBulkActions`, `useReaderTabs`, `useOutbox`, `useFolders`, `useAccounts`, `useSync`, `useTags`, `useComposeLauncher`, `useAppEvents`, `useMenus`, `useMessageListKeyboard`) plus pure modules (`replyDraft` and the earlier text/shortcut/print/focus-ring helpers), and the render tree into four more components (`TitleBar`, `WelcomeScreen`, `SelectionSummary`, `DraftRecoveryDialog`). The fan-out-to-lists and remove-from-lists duplications collapsed into `useMessageStore`. The one deliberate leaf is the modal stack (`AppModals`): it wires already-componentised modals to App's state and a wrapper would need ~90 props, which displaces coupling rather than reducing it, so App stays its composition root.

#### A2. `App.css`, 3,070 lines to a manifest (RESOLVED)

Split by concern into eleven files under `src/styles/`, imported by a thin `App.css` manifest in the exact original source order, so the cascade is unchanged. Chosen over the component-ownership split (murky ownership, order-sensitive, unverifiable). Proven byte-neutral three ways: the parts concatenate to the original, the build validates every file, and the bundled CSS content-hash is unchanged. See ARCHITECTURE.md (Styles) for the go-forward rule.

#### A3 and A4. The big components, also decomposed (RESOLVED)

The earlier phases split the calendar (`CalendarModal` into `EventFormModal`, `useEventInstances`, `useCalendars` / `CalendarsManager`, `useOpenFromReminder`), the sidebar (`AccountList`, `FolderTree`, `useAccountReorder`, `usePersistedFolderState`), the account-setup modal (`ProviderChooser`, `AccountDetailsForm`, `useAccountForm`, `RichTextField`, `useLinkEditor`), the compose window (`useDraftAutosave`, `useSeparatorCorrection`, the shared `useLinkEditor`) and the reader (`ReaderToolbar`, `ReaderAttachments`, `TagColourMenu` / `useTagMenu`), each behind its own characterization tests.

#### A5. The front-end test gap (CLOSED)

The gap that made A1 risky is closed: the harness plus characterization tests for the keyboard, selection, delete/move and compose flows went in first, as the prerequisite, and grew alongside every extraction.

**Verdict: DONE.** The value was high and the risk was real; both were handled by putting the test net in first and moving in small, independently-shippable, verified steps. The one asymmetry this document opened with is closed: the front end now has the same kind of executable safety net as the Go core.

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

The structural evidence is consistent: the decisions compounded because the important invariants were made executable, so they held under speed. The backend is not clean by accident; it is clean because the gates make the bad states unrepresentable. The actionable backend tidy-ups are done, and the front-end decomposition that was the one remaining concentration of debt has now landed; what is left is one optional backend port-shape decision.

The front end that was the one genuine concentration of debt, where the invariants never reached, has been handled exactly as prescribed: highest-value and highest-risk, so a front-end test net went in first and `App.tsx` was decomposed in small staged, verified steps rather than a big-bang rewrite. The front end now carries the same executable safety net as the Go core.
