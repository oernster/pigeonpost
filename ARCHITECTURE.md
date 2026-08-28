# PigeonPost Architecture

## Invariant

`UI -> Application -> Domain <- Infrastructure`

Dependencies point inward. The Domain is the stable core and depends on nothing. Every rule below is
enforced by a test in `tests/structural/boundary_test.go`, not by convention.

| Invariant | Enforcing test |
|---|---|
| Domain imports nothing from application/infrastructure/ui/wails | `TestDomainHasNoOutwardImports` |
| Domain is pure: no net, os, database/sql, time.Now, math/rand | `TestDomainIsPure` |
| Application never imports infrastructure or wails | `TestApplicationDoesNotImportInfrastructure` |
| No Go source file exceeds the module-size limit | `TestNoFileExceedsLineLimit` |
| The composition root is the only place that wires concrete adapters | `TestCompositionRootIsWhitelisted` |

## Layers

- **Domain** (`internal/domain`): pure Go, standard library only. Immutable value objects validated
  on construction (`With*` copy methods for change). No IO, no wall-clock reads; time enters through
  the injected `Clock`. This is where correctness lives and where the 100% coverage gate applies.
- **Application** (`internal/application`): use cases plus the port interfaces they depend on
  (`AccountStore`, `CredentialStore`, `AccountVerifier`, `MailStore`, `MailSource`, `MailActions`,
  `MailTransport`, `FolderActions`, `DraftSaver`, `OutboxStore`, `TagStore`, `RuleStore`, `Clock`,
  plus the later feature ports for contacts, calendar, recurrence, scheduling, draft recovery,
  remote images, CalDAV, the folder display state and the update check's `ReleaseSource`, each
  introduced with its feature below). The
  `MailSource`, `MailActions` and `AccountVerifier` ports are satisfied by the `mailrouter` adapter,
  which dispatches to the IMAP or POP3 implementation per account protocol. Depends on Domain and the
  standard library only. Never imports Infrastructure or the Wails runtime.
- **Infrastructure** (`internal/infrastructure`): concrete adapters implementing the Application
  ports. Owns SQLite (`storage`), IMAP (`imap`), POP3 (`pop3`, a hand-rolled client), SMTP (`smtp`), the
  shared RFC 5322 MIME builder (`message`, used by both `smtp` for send and `imap` for draft append so
  the message-format logic is not duplicated), the shared message-body parser with HTML sanitising and
  image-blocking (`mailparse`, used by both the IMAP and POP3 read paths and by the MIME builder for
  the outgoing linkify and embedded-image extraction), the per-protocol dispatcher
  (`mailrouter`, which routes reads, verification and actions to the IMAP or POP3 adapter by account
  protocol), the reminder and unread surfaces (`taskbar`: the Windows taskbar unread-overlay badge and
  reminder flash, no-ops off Windows, plus the notification tray, a Windows tray icon that also carries
  the unread badge or a native desktop notification elsewhere), the notification chime (`sound`, a
  synthesised WAV played through `winmm` on Windows and a no-op elsewhere), the OS keychain (`keychain`), the
  calendar and contacts codecs (`ics`, `recurrence`, `vcard` and `csv`), the CalDAV sync client
  (`caldav`), the Microsoft OAuth token flow (`oauth`) and the SSRF-guarded remote-image fetcher
  (`remoteimage`). Never imported by Domain or Application. The
  separate `internal/installer` package holds the setup program's install logic and is consumed by the
  `installer/` Wails setup app.
- **UI**: the React front end plus the thin Wails facade in package `main` (`app.go` with one binding
  file per feature surface: accounts, mail, folders, send, draft recovery, outbox, snooze, tags, rules,
  templates, calendar, CalDAV, contacts, scheduling, export, `.eml` files, updates and About, plus the
  `dto.go` DTO mappers and the `clock.go` clock). The facade is a client of the Application use cases only; it
  maps domain results to DTOs and holds no business logic.

## Composition root

`main.go` is the single composition root. It constructs concrete infrastructure adapters and injects
them into the application use cases by constructor injection, then hands the assembled facade to
Wails. There are no global singletons, no service locator and no auto-wiring. The structural test
whitelists this file as the only one permitted to import both `application` and `infrastructure`.

## Dependency direction

```
             +-------------------+
   Wails/UI  |   app.go (facade) |
             +---------+---------+
                       | calls
             +---------v---------+
             |    application    |  ports (interfaces) + use cases
             +----+---------+----+
        depends on |         ^ implements
             +-----v----+    |
             |  domain  |    |
             +----------+    |
                       +-----+---------------+
                       |   infrastructure    |  sqlite, imap, ...
                       +---------------------+
```

## Execution flow

Sync and read:

1. `main.go` opens the SQLite store, builds the use cases (accounts, mailbox, sync, compose) and the
   Wails facade, injecting the concrete adapters.
2. The React UI asks the facade for accounts, folders and message summaries.
3. The sync use case pulls folders and message summaries from the mail source and persists them
   through the store; the UI reads from the store so it works offline. The `mailrouter` picks the
   adapter by account protocol: an IMAP account lists its server folders, while a POP3 account
   downloads into a single local inbox (POP3 has no server-side folders), deduped by its UIDL, with
   read and star marks kept locally since POP3 carries no server flags. The message server handle is an
   opaque string that holds an IMAP UID or a POP3 UIDL. Folder unread and total counts are
   computed from the cached messages, so the per-folder, per-account and total badges are populated
   without a separate server STATUS pass. On the front end, every message action that can change an
   unread count (mark read/unread, delete, junk, move, the bulk forms) refreshes the account badges and
   the folder tree together through one shared refresher, so no badge surface can go stale alone. Mail
   arriving refreshes both surfaces too, whether it is announced by the poller (`mail:new`) or brought
   in by the background folder poll: the counts and the folder list are separate reads, so refreshing
   only the counts badges the titlebar and the account picker while leaving the folder row bare. Every
   folder-list write is claimed by the account it was fetched for, so a fetch that outlives an account
   switch is discarded rather than putting the previous account's folders under the new selection.

Add account:

1. The UI submits the setup wizard payload (identity, password, incoming and outgoing servers) to the
   facade's `AddAccount`.
2. The facade maps the wire strings to domain enums and builds a validated `Account` (its id is the
   email address), then calls the `AccountSetupService`.
3. The setup use case verifies the credentials against the incoming server through the
   `AccountVerifier` port (IMAP login) *first*, then stores the password through the `CredentialStore`
   port (keychain) and persists the account through `AccountStore`. Because nothing is written until
   the password is known good, a failed verify leaves the keychain and store untouched.

Edit account:

1. The UI opens the wizard prefilled from the account (its email is fixed, so that field is locked)
   and calls the facade's `UpdateAccount`.
2. The `AccountSetupService.Update` use case verifies first: a blank password re-verifies with the
   existing keychain secret (read server-side, never sent to the UI); a new password is verified and
   (only if good) replaces the stored one. The account is then persisted. A failed verify never
   disturbs the working account's stored password.

Remove account:

1. The UI confirms the destructive action in a modal, then calls the facade's `RemoveAccount`.
2. The `AccountService` deletes the account row (so it leaves the UI at once), then clears its cached
   folders and messages through `MailStore` and finally deletes its keychain secret through
   `CredentialStore`. The account's mail on the server is never touched.

Read a message body:

1. Opening a message calls the facade's `GetMessageBody`.
2. The `MessageBodyService` serves the cached body when present; on a miss it resolves the message,
   its folder and account through the stores, fetches the full body from the `MailSource`, caches it
   (the `message_body` table) and returns it. The message therefore reads offline after its
   first open. The parser also extracts attachment parts, cached alongside the body (the
   `message_attachment` table) so a received attachment can be saved offline from the reader; the list
   shows a paperclip for a message whose fetched IMAP body structure (BODYSTRUCTURE) has an
   attachment-disposition part. A one-off migration clears the cached bodies so each re-fetches with the
   attachment-aware parser. Subjects and display names are RFC 2047 decoded (through a charset reader, so
   windows-1252 and the like decode) and HTML-entity unescaped in the mail-source mapping via
   `mailparse.DecodeHeader`, shared by the IMAP envelope path and POP3, so encoded-word and
   template-built headers read as text. The shared `mailparse` package (used by both the IMAP and POP3
   read paths) parses the MIME into plain-text and HTML parts; the HTML is sanitised there (bluemonday) so only safe markup ever
   enters the cache; an HTML-only message also gets a plain-text rendering derived from the HTML.
   The same pre-sanitise pass drops nodes the sender hid with inline CSS (a preheader / preview-text
   block): the sanitiser strips the style that hid them, so left in place they would surface and
   duplicate the visible content. Before sanitising, every remote `<img src>` is parked in
   a `data-pp-src` attribute (and `srcset` dropped) so images do not auto-load, which would leak that
   the reader opened the message; the UI shows a "Load images" action that restores the source on
   request. Between the prepare pass and the sanitiser, bare web addresses in the message text are
   linkified the way mainstream clients do (`mailparse` linkify: http, https, mailto and www hosts become
   anchors; markdown-style `[label](url)` links render as their label; text already inside an anchor,
   script, style or form controls is left alone) and an anchor standing alone on its own line, between
   `<br>`s or block edges, is marked `pp-solo-link` so the reader presents it as a call-to-action button;
   the sanitiser then applies its usual scheme policy to the new anchors. The UI renders the sanitised
   HTML when present (links open in the external browser via the
   facade, never the app's own webview) and falls back to the plain text otherwise; the plain-text view
   applies the same linkify rules through the `LinkifiedText` component, including the solo-line button
   presentation, themed via the app's accent tokens.
3. The sanitised HTML renders inside a sandboxed iframe (`EmailHtmlFrame`) rather than the app's own
   document: the frame is `sandbox="allow-same-origin allow-scripts"` (popups, top navigation, forms and
   downloads all stay blocked) under a strict content-security-policy (`default-src 'none'`, images and
   fonts restricted to `data:`), so no script in the message runs and the frame makes zero remote requests,
   meaning opening a message cannot leak that it was read. Script execution is denied by the CSP (no
   `script-src`) plus the sanitiser rather than the sandbox flag: WebKit (WKWebView, WebKitGTK) refuses to
   dispatch event listeners inside a scripts-disabled browsing context, including listeners the parent
   registered on the frame's document, so a scriptless sandbox left the reader's link-click interception
   dead on macOS and Linux. The parent also writes the frame's document itself
   (`contentDocument.open()/write()/close()` rather than the `srcdoc` attribute), because WebKit does not
   reliably fire the iframe load event for `srcdoc`, which left listeners bound to the dead initial
   document. Loading the parked images routes through a server-side proxy: the application
   `RemoteImageService` over the `remoteimage` resolver fetches each blocked image and inlines it as a
   `data:` URI, sidestepping the CORP/CORS rules that would otherwise stop the iframe embedding it by URL.
   The fetch is SSRF-guarded by a `net.Dialer.Control` hook that checks the real post-DNS connect IP and
   rejects private, loopback, link-local and CGNAT addresses, under size, timeout and redirect caps and an
   image-only content-type check. A remote CSS `url(...)` is parked too, behind an unfetchable
   `pp-blocked:` scheme with the target base64url-encoded, so a background image is restored by the same
   proxy on the same request. Encoding keeps the parked value inside the character set the sanitiser's CSS
   value check accepts, so a URL with a query string cannot take its whole declaration (background colour and
   all) down with it; a `@font-face` source is emptied rather than parked, since the proxy's image-only
   content-type check would reject a font anyway and parking one would only buy a futile outbound request.
4. The frame is pinned to the light colour scheme in both themes, so every message renders in its light
   design. The pin sits as `color-scheme: light` on the iframe ELEMENT, because an embedded document takes
   its colour-scheme preference from its embedder and the same declaration inside the frame's own document
   does not change it (measured in Chromium/WebView2); the print path pins its frame the same way. Without
   the pin the frame inherits the app's dark scheme and any `prefers-color-scheme: dark` rules the message
   carries switch themselves on; dark-mode support in HTML email is almost always partial, a media query
   recolouring a handful of elements while `bgcolor` attributes and inline styles keep the bulk of the page
   white, so deferring to it left the reader blinding in the dark theme.

   The theme is then applied per element by `emailDarkMode`, which walks the written document. A single
   document-wide invert cannot work, because one message routinely mixes light-designed and dark-designed
   regions: Steam's wishlist mail is a dark panel (`bgcolor #212429`, white text) inside a white wrapper with
   a white footer; inverting the lot renders half of it upside down. Classifying the whole message either
   way fails for the same reason. So the walk inverts at the root and inverts a second time wherever the
   sender already designed dark, which returns that region to its authored colours. A `filter` on an element
   and one on its descendants compound, so what governs is the PARITY of the filters above a node: the walk
   carries it down and flips only where parity disagrees with the background the author gave that element.
   The invariant is uniform, every region ends up rendering dark. Media (`img`, `picture`, `video`, `svg`,
   `canvas`) is forced to an even parity so a photo or logo always shows true colour; media flipped back
   inside an inverted region carries a mid-grey hairline (with `box-sizing: border-box` so it does not resize
   the image) so a genuinely dark image keeps an edge against the now-dark surround.

   The same walk repairs text that cannot be read against its own background, in both themes. The cause is
   usually a background image that did not render: ClearScore's hero colours its heading `#ffffff` for a
   remote photo and leaves `#EAF5F5` as the fallback, so with the image absent the heading is white on
   near-white, which the invert turned into black on black. Text under a contrast ratio of 2 is reset to the
   paper ink; the repair is skipped wherever a background image is actually painting, since the sender's
   choice is then the right one. What counts as painting is a `data:` image or a gradient, not merely a
   computed `background-image` that is not `none`: a parked or emptied `url()` still computes as a url (an
   empty one resolves against the document itself) while painting nothing, so testing for `none` would
   suppress the repair on exactly the elements that lost their backdrop.

Printing a message reuses the same sanitised HTML. The message's parked remote images are restored for the
printed copy; the document is rendered into a hidden iframe that is invoked through the browser's print
dialog, so only the message prints rather than the whole app window. The frame is parked far off-screen but
given a real page-sized layout box (a zero-size frame has no viewport for the engine to lay the document into
and prints blank) and is pinned to a light colour scheme so it does not inherit the app's dark scheme and
prints as dark text on white paper. Its `srcdoc` is set before the frame is inserted so its only load is the
print document rather than the empty `about:blank` a fresh iframe momentarily holds. The print fires only once
a marker element from the print document is present, so the dialog never captures a blank page.

Send (also reply, reply-all and forward, which just pre-fill the same compose window before the
identical send path runs: reply pre-fills the sender; reply-all pre-fills the sender plus the original
To and Cc with the reader's own address and duplicates removed; forward pre-fills the quoted original;
all set a `Re:`/`Fwd:` subject). Reply-all is possible because the cached message summary now stores the
original To and Cc:

1. The UI submits a compose request (recipients, subject, body) to the facade.
2. The facade parses the addresses into domain value objects and calls the compose use case.
3. The compose use case loads the account, builds a validated `OutgoingMessage` (sender taken from
   the account) and hands it to the SMTP transport, which authenticates using the keychain password
   and delivers it. The compose editor is TipTap rich text: the draft carries both a plain-text body
   and an optional HTML body; when HTML is present the shared `message` MIME builder emits a
   `multipart/alternative` message (plain text first, HTML second) so plain-text clients still render.
   The builder linkifies bare and markdown-labelled URLs in the outgoing HTML (so a pasted or
   mailto:-prefilled link reaches recipients in any client as a real anchor) and encodes text parts
   quoted-printable rather than 8bit, so a long line (a URL, an unwrapped paragraph) folds losslessly
   instead of being hard-folded mid-content by a relay.
   An image pasted or dropped into the composer lives in the editor HTML as a `data:` URI, the same
   representation the reader produces when it resolves a received message's `cid:` images for display,
   so autosave, draft recovery, the outbox row and a forwarded message's quoted images all ride the
   existing `html_body` plumbing with no extra state. The conversion to wire shape happens only in the
   builder: `mailparse.ExtractDataImages` lifts each embedded image out of the HTML, rewrites its src
   to a `cid:` reference (the id a hash of the image bytes, so a repeated image becomes one part) and
   the builder emits `multipart/alternative(text/plain, multipart/related(text/html, image parts))`,
   the nesting that keeps the plain-text variant image-free. The symmetry closes the round trip:
   incoming cid becomes data: for display, outgoing data: becomes cid on the wire.
   Bcc recipients are added to the SMTP envelope (de-duplicated with To and Cc) but never
   written to the headers. Attachments turn the body into `multipart/mixed`: files chosen
   from disk, files pasted or dropped into the compose window (carried as bytes over the bridge
   where the engine hands the page File objects or by path where a paste exposes only file://
   URIs, as WebKit does for Finder-copied files) plus, optionally, an existing message fetched
   as a `message/rfc822` part, bounded by a total-size cap in the facade that counts embedded images
   too.

Recipient autocomplete: the To, Cc and Bcc fields suggest matching addresses from the address book
as an address fragment is typed. The matching, ranking and text-splicing logic is the gated pure
`recipientSuggest` module; the shared suggestion pool loads lazily through the contacts use case on
the first touch of a recipient field, so an untouched compose makes no call. Accepting a suggestion
only inserts text into the ordinary input, so the field stays freely editable and the backend
remains the address validator. Acceptance by Enter or click also appends the canonical separator
("; ") so the next address can be typed immediately (Tab does not, since focus is leaving the
field) and a space typed at the end of a complete address inserts the separator the same way; both
are pure `recipientSuggest` functions sharing the separator constant with the separator-correction
helper in `composeAddresses`. The suggestion pool treats a display name that is the contact's own
address as no name, so such a contact (auto-collected ones often are) is offered as the bare
address rather than the address twice. In the stylesheet the fixed-width label column of a compose
row is scoped to the field's direct child span, keeping it off the suggestion rows, whose name and
address spans stay on one line and ellipsise rather than wrap or overflow.

Save draft: Compose > Save draft calls the compose use case, which resolves the account's Drafts
mailbox from the cached folders and (through the `DraftSaver` port) renders the message with the shared
`message` builder and appends it to that mailbox on the server (IMAP APPEND, flagged `\Draft \Seen`).
Unlike a send, a draft may be incomplete (no recipients, empty body), so it is built with the lenient
`NewDraftMessage`.

Edit draft: opening a message that lives in a Drafts folder routes to the composer rather than the
reader (double-click or Enter on the row, else the reader toolbar's Edit draft action, which
replaces reply, reply-all and forward there). The front end fetches the stored body and rebuilds the compose
fields from the draft itself through the gated pure `draftEdit` module: no signature is seeded and
nothing is quoted, both being already in the saved text, so a reopen never duplicates them. The
draft's own id rides the compose state as `draftId`; once the replacement has been sent or saved,
that superseded Drafts copy is deleted permanently (routing every intermediate save through Trash
would fill it with versions of one message) and a failed delete leaves a duplicate draft rather than
losing anything. Two stated limits: Bcc does not round-trip, since a saved draft carries no Bcc
header; and the route reads the selected account's folder list, so a draft row belonging to another
account seen in the unified mailbox still opens in the reader.

The compose window is movable by its title bar, so a long reply can be pushed aside to re-read the
message underneath. The clamp geometry is the gated pure `modalDrag` module (a strip of the window
always stays reachable at the sides and foot; the top edge never leaves the screen, because the
header is the only handle); `useModalDrag` owns the pointer capture and applies the offset as a
translation on top of the backdrop's flex centring, so every compose opens centred, the offset is
re-clamped on a window resize and a closed window forgets where it was put. A press on a control
sitting on the bar (the close cross) reaches that control rather than starting a drag.

Compose discard guard: every discard path out of the compose window (backdrop click, Escape, the
close cross, Cancel) routes through one guard in `ComposeModal`: a compose the user has actually
edited (the autosave's dirty flag) that still holds content confirms "Discard message?" before
closing, so a stray click, such as the one that refocuses the app window onto the backdrop, can
never silently lose a message. An untouched or emptied-out compose closes at once. Send and Save
draft close directly, having preserved the message. The modal's render in `App` is gated on the
composing flag alone (not the resolved account), so an account-state change can never unmount a
compose in progress; the neutral-focus anchor declines to take focus while any dialog is open.

Draft recovery: separately from the server-side Save draft, the compose window autosaves its in-progress
content (debounced, once the user has edited it) to a single-row local slot through the
`DraftRecoveryStore` port (the `draft_recovery` table). It is local only and never sent to the
server; on the next launch the UI offers to restore it; sending, saving a server draft or discarding
it clears the slot.

Offline outbox: the SMTP, IMAP and POP3 adapters wrap a failed dial with the `ErrOffline` sentinel. When the
compose use case sees `ErrOffline` from a send or a draft append, instead of failing it queues the
operation through the `OutboxStore` port (the `outbox` table, which also carries Bcc and
attachments so a queued message keeps them on replay) and returns success; the UI surfaces the
queue as a per-account outbox folder where the waiting messages can be reviewed or cancelled. After the
next successful sync the UI calls replay, which drains the queue oldest-first: each item is re-sent or
re-appended, removed on success, left in place if still offline and dropped (with its error reported)
if it can never succeed. A replayed send keeps the same best-effort Sent copy a direct send leaves. The
queue covers outgoing mail only; message flag/delete/move actions remain online-only by design. Every
connection attempt is bounded by a short dial timeout (DNS plus the TCP and TLS handshake), so an action
taken while offline fails within seconds rather than blocking on the client library's or the operating
system's default wait. Because the online-only actions cannot be queued, their `ErrOffline` is translated
once at the Wails facade into a plain, user-facing message (the technical dial detail never reaches the
interface); a batched action carries an offline flag alongside its error so the front end shows that
message on its own rather than wrapping it in a "N of M could not be ..." line. Whatever the source,
an error surfaces in the main window's `ErrorBar` (a banner under the toolbar carrying an explicit
dismiss control), where it persists until dismissed or replaced by a later action's error.

Undo send: every send passes through `ComposeService.HoldSend` with the user's undo window (a Mail-menu
choice of 0 to 30 seconds, default 10, persisted locally). A positive window queues the message in the
same outbox with a hold instant (`hold_until_ms`) and returns the queued id; the front end
shows a countdown toast whose Undo cancels the item and reopens the composer exactly as it was, with a
reply's answered-flag marking deferred to the window's expiry so an undone reply never flags its
original. A held item is invisible to the ordinary replay (no path may send it early); once the hold
elapses, a small dispatcher goroutine in the composition root (`runOutboxDispatcher`, woken by a short
tick and gated on the store's earliest hold) sends it and announces the change over the `outbox:changed`
event. An undo that loses the race is told so: cancel reports whether the item was still queued. A due
item that finds the server unreachable has its hold cleared, degrading it to an ordinary offline-queued
item for the next sync rather than being retried every tick; a hold outlasting an app restart sends
on the next launch.

Send later: a scheduled send is the same hold with a chosen instant. The composer's Send later control
(presets plus a date-time field) passes `sendAtMs` on the compose request; `ComposeService.ScheduleSend`
validates the instant is in the future and queues the message exactly as an undo hold, so everything
above carries over: the Outbox shows it with its send time and offers Cancel send, no replay may send it
early, the dispatcher delivers it when due, a due-but-offline item degrades to the ordinary queue and a
schedule outlasting a restart sends on the next launch. There is no undo toast (the Outbox is the
cancel surface) and a scheduled reply does not flag its original (a schedule cancelled days later must
not have already marked it); the composer states the local-first constraint plainly: the message leaves
while the app runs or at the next launch after the chosen time.

Snooze: a message can be hidden until a chosen moment (context-menu or Mail-menu presets or a
pick-a-time dialog). Snooze is local-only state, one row per message in `message_snooze`:
nothing reaches the server and read/flag state is untouched. The visible listings
(`MailStore.ListMessagesVisible`, `ListMessagesPageVisible` and the snooze-aware `UnreadByAccount`
and `NewestUnreadByAccount`)
exclude a hidden message until its instant passes, while the plain listings the sync and the new-mail
notifier read see everything, so known-message sets and POP3 flag carry-over are unaffected; search also
still finds hidden messages. The Snoozed view is the synthetic folder `__snoozed__` (the Outbox
pattern), listing every hidden message across accounts with its due time, an Unsnooze action and the
same per-account dots and cross-account rules as the unified mailbox (rows compose from their own
account; Move, Copy and Junk live in the real folder). A scheduler goroutine (`runSnoozeScheduler`,
gated on the store's earliest snooze) pops due snoozes in one transaction, raises a desktop notification
(a snooze is an alarm the user set) and announces `snooze:changed`; a snooze missed while the app was
closed pops on the first tick after launch; a snooze orphaned by its message's deletion or move is
swept rather than resurfacing as a ghost.

Junk, conversations and list order: marking a message as junk moves it to the account's Junk folder
through the same online path as Move (`MessageActionService.MarkJunk`, resolving the Junk folder by kind);
Not junk (`MarkNotJunk`, offered on a message sitting in Junk) rescues it back to the Inbox the same way.
Both record the spam verdict on the server first as best-effort IMAP keywords (the `$Junk`/`Junk` pair set
and the `$NotJunk`/`NonJunk` pair cleared or the reverse), so other clients reading either keyword
convention agree; the folder move stays the authoritative action since keyword support varies by server.
Conversation grouping and list order are read-side concerns over the same cached summaries the flat list
uses: the domain `GroupThreads` groups a folder's summaries into conversations by normalised subject
(reply/forward prefixes stripped), exposed through `MailboxService.Threads`; the UI sorts the list by
date in either direction.

`MailboxService.Conversation` answers the question that grouping raises but cannot settle: a folder's
threading shows that a message has replies, while the replies themselves live elsewhere, above all the
ones the user sent, which are in Sent. It gathers the open message's thread across every folder of its
account. Two surfaces read it. The reader lists it under an open message's header as a strip, each entry
naming its folder and opening in its own reader tab. The conversation header row in the list opens the
thread whole in the reader (`ThreadView`), where each message expands in place, its body fetched the
first time it is opened; that row is the mouse route and the Mail menu's Open conversation is the
keyboard one, so the list keeps its single roving tab stop. Both surfaces are gated on the conversation
view setting along with the grouping itself: the tick is one switch over the whole feature, so turning it
off flattens the list, withholds the strip's lookup and closes any thread being read. Membership is `domain.ThreadKey` equality, the exported form of the grouping's own
normalisation, so a message cannot thread one way in the list and another way in the reader. The store's
`ThreadMessages` narrows the candidates with a subject-suffix LIKE (the key is always a suffix of the raw
subject; SQL cannot apply the domain's stripping rule), capped, with the exact comparison done in the
service. Real RFC threading over `In-Reply-To` and `References` would need those headers fetched and
stored, which the cache does not hold today; subject threading is what both surfaces already agree on. The desktop list mirrors the grouping client-side so it updates instantly with
optimistic changes, keeping the domain function as the single tested definition.

Large folders: the message list is fully virtualised (`@tanstack/react-virtual`) so only on-screen rows
exist in the DOM and it loads in pages of 200 through keyset pagination. `Store.ListMessagesPage`, exposed
as `MailboxService.MessagesPage`, walks an indexed `(folder_id, date_ms, id)` order (the
`idx_message_folder_date` index) and resumes strictly after the last row returned, its `(date_ms, id)` tie-break a
total order so no row is skipped or repeated. Toggling a message read or unread mutates the row in place and
refreshes only the unread counts rather than refetching the folder, so a folder of tens of thousands of
messages never reloads every row.

Unified mailbox: a View tick shows an All-inboxes entry in the sidebar whose list merges every account's
inbox, newest first. It is read-side aggregation only: `UnifiedMailboxService` fans the same keyset
cursor out to each inbox folder through the existing `MailStore.ListMessagesPage`, merges the returned
pages in the store's `(date_ms, id)` order and keeps the first page-worth, so the walk stays total and
no storage changes. In the UI the view is the synthetic folder `__unified__` (the Outbox pattern): the
api module routes its listing, paging and sync calls to the unified endpoints, so pagination, the
conversation view, sorting and the background poll (which refreshes every inbox via
`SyncService.SyncInboxes`) all work on it unchanged. Each row carries its owning account: a colour dot
labels it in the list and a reply or forward composes from that account, not the sidebar selection.
Move, copy and junk are unavailable in the combined view (the folder targets belong to one account) and
a drag onto a folder of a different account is filtered out; the message's real folder offers all of
them.

Delete a message: after a confirmation modal, the UI calls the facade, routed through the
`MessageActionService`. It resolves the message's folder and account, then via the `MailActions` port
moves the message to the account's Trash folder when one exists or deletes it permanently (mark
`\Deleted` and expunge) when the message is already in Trash or the account has no Trash folder. The
cached message and everything derived from it (body, tags, index row) are then removed locally.

Move a message: the UI offers the account's other folders; choosing one routes through the
`MessageActionService`, which checks the destination is in the same account, moves the message on the
server via the `MailActions` port and removes the local copy (the destination folder re-lists it, with
its new server UID, on the next sync). Copy is the same path without removing the original.

Every move-shaped action (move, delete to Trash, junk and its rescue, copy, the bulk forms) also
reports where the message landed: the IMAP adapter reads the server's COPYUID reply (RFC 4315),
pairing each source UID with its destination UID; the service maps that to the id the message
carries in its destination (`domain.MessageIDFor`, the single spelling of the folder-plus-UID
identity). The facade returns it in `MoveResultDTO`/`BulkResultDTO`; a server without UIDPLUS
reports nothing and the id is empty rather than guessed.

The reader is a fixed frame with a scrolling middle, not one long scrolling column. `.reader-scroll`
carries the header and the body; `.reader-footer` is its sibling and holds whatever the message offers at
its base, so that stays on screen while the email scrolls behind it. As one column the foot came after the
body, so on a long reply chain the attachment's Save button sat below every quoted round and could only be
reached by scrolling past the lot. The base is capped at a share of the pane and scrolls internally past
that, so a message with twenty attachments cannot squeeze the email off the screen; it is not rendered
at all when the message has nothing to put in it. The message popout hosts the same reader and therefore
stops scrolling it as one block; otherwise the foot would be below the fold again. Adding a new bottom-of-message
control means extending the footer's condition, not the layout.

Dragging a message onto a folder is optimistic and says so. An IMAP move is a live server round trip
that can take seconds, so `useBulkActions` takes the dropped rows out of every on-screen list at the
moment of the drop rather than when the server answers: a list that does not visibly change reads as a
drop that missed; the reflex is to drag again. The rollback is what makes the optimism safe.
`optimisticList.ts` (a gated pure module) lifts each row with the id of the nearest row before it that
STAYED in the list; a refused or partial move splices the untouched rows back into those gaps. The
anchor is an id and not an index on purpose: a partial move leaves some of the batch gone for good,
which shifts every index after them. The same handler holds the ids whose move is still open, so a
repeat drop of a message already in flight is dropped rather than issued twice; it reports whether
it took the drop at all, which is what the sidebar's landing flash is gated on (a drop skipped for being
same-folder, cross-account or in flight must not be confirmed on screen).

A dragged row that belongs to the selection carries the whole selection, whichever gesture built it: the
drop handler takes the marked ids when the dropped id is one of them and that one id otherwise, so a
Ctrl-built and a Shift-built selection are the same thing by the time they reach it. What differed was
upstream, at the drag start. A Shift-click extends the document text selection across every row it spans;
the browser then drags that text rather than the messages, so the row used to cancel the mousedown
default when Shift was held. Cancelling that default cancels the native drag the browser would have begun
from the same mousedown, which left a Shift-built range selectable but undraggable. The rows are marked
`user-select: none` in CSS instead: nothing to smear; the mousedown reaches the browser intact. A row
is a selection target rather than a text surface, so the suppression belongs on the surface as a standing
property, not on the event as a special case.

`user-select: none` stops the smear but Blink goes further: it classifies a Shift-held mousedown as
"extend the selection" and refuses to start a native drag from it at all, so the range could be built
with Shift down yet only dragged after letting go, which reads as drag-and-drop being broken. The
engine's one escape hatch is the computed `-webkit-user-drag: element` of the pressed node (the
`draggable` attribute is never consulted and the check runs on the deepest hit-tested node, where the
property does not inherit), so the rows and all their descendants carry it in `list-rows.css`. That
makes the pressed child the drag source, so the row's `onDragStart` resets the drag image to the whole
row at the grab point and the drag looks the same whichever child the press lands on. The rule is
pinned by a stylesheet-scan test in `App.test.tsx` beside the mousedown-default test above.

Two drag ergonomics live in the sidebar, both as a pure geometry module plus a hook holding the event
and frame plumbing. `dragScroll.ts` sizes the edge hot zone that auto-scrolls the folder pane during a
drag: the browser's own is a couple of pixels deep at the pane's edge and runs only while the pointer
moves, so `useDragAutoScroll` widens it to a band (capped at a quarter of the pane's height, so a short
pane keeps a neutral middle), ramps the speed with depth into the band and drives it from an animation
frame loop keyed off the last pointer position, so resting in the band keeps scrolling. `autoScroll.ts`
is the self-reading cycle for long help content, at the pace the desktop apps use: hold still on open,
read down a pixel every second tick, hold at the tail, rewind fast, repeat. `useAutoScroll` adds what is
a property of the page rather than the cycle: any manual reading input suspends it for a stillness
window and it resumes from wherever the reader left it (never switches off); a surface underneath
another modal is frozen rather than suspended so its phase and position survive the modal above. Both
panes put the cycle on their inner body and pin their action row beneath it (`.modal.pinned-actions`, a
flex column whose body is the scroller), so Close never drifts off as the content reads itself. That
layout is no longer particular to these two: every dialog carrying an action row now wears it, so a
tall dialog scrolls its body instead of taking its buttons off the bottom of a short window. The
furniture around the body (a title, an intro, a toolbar above it, the action row below) is held at
`flex: none` by one rule rather than one per element; `modalLayout.test.ts` scans the source so a
new dialog that forgets the class fails on the day it is written.
Neither this nor the drop flash is gated on `prefers-reduced-motion`: on Windows that query follows the
general "Animation effects" switch, which people turn off for performance rather than motion
sensitivity, so gating on it silently removed both features on a machine that had it off. Stopping the
cycle is what touching the pane is for. The self-reading cycle, unlike the pinned layout above, is worn
by the About and Licence panes only; every other scrollable surface in the app is a work or decision
surface, where content that moves on its own would fight the user.

Undo, redo and the message clipboard (front end): the reported destination ids are what make undo
possible. `undoStack.ts` (a gated pure module) models the undo and redo stacks: entries for the
move-shaped actions plus the read, star and tag toggles, capped at a fixed depth, each labelled with
what it will unwind ("Undo delete"). `useUndoRedo` executes an entry through the same api actions and
rebinds the entry's message ids from each execution's own COPYUID reply, so undo and redo can
ping-pong indefinitely; an action whose server reported no id is simply never recorded, so the menu
never promises what it cannot address. `useMessageClipboard` is the file-manager cut/copy/paste:
cut or copy takes the selection onto an internal clipboard (cut rows dim in the list via `cutIds`)
and paste files it into a folder. A pasted cut is optimistic: rows join the open folder at once,
the batched server move settles behind them and each row is re-pointed at its reported new id
(refused rows roll back; a wholly failed paste restores the clipboard). A pasted copy inserts each
duplicate's row the moment the server reports the id its copy carries, cloned from the original;
without a reported id the copy appears on the destination sync instead, so a row is never shown
under an invented identity. `editClipboard.ts` (also gated pure) decides the text-versus-message
context, so Cut, Copy and Paste act on a text selection first and messages otherwise. Menu
accelerators are wired once in `useMenus` from the same item definitions the menus render (submenu
children flattened in), so a hint and its key can never drift.

Folder operations: the `FolderService` creates, renames and deletes mailboxes on the server through the
`FolderActions` port. Each cached `Folder` records the server's mailbox hierarchy delimiter,
captured from the IMAP `LIST` response, so the leaf name and a rename's destination path are
derived with the real separator ("." on StartMail, not the default "/"); a folder with an unknown
delimiter falls back to "/". A folder is created either way round: `Create` takes an account and a
path and lands at the top level, while `CreateChild` takes a parent folder id and a LEAF name and
joins the two with that parent's own delimiter (`Folder.ChildPath`), so the caller never has to know
what the server's delimiter is. A leaf holding the delimiter is refused
(`ErrFolderNameHasSeparator`) rather than quietly nesting deeper than the name asked for; the parent
also supplies the account, so a subfolder cannot be created against the wrong one. `FolderService.Move` reparents a folder, moving it under a new parent through
the same path-to-path rename (an empty parent is the top level) and rejecting a move across accounts or
into the folder's own subtree; the sidebar's folder drag-and-drop calls this, while a same-level reorder is
a local per-account display order, since IMAP has no folder order. That display state (the custom folders'
order and the collapsed folder paths) persists durably in the database via `FolderUIStateService` and the
`folder_ui_state` table, so it survives an application update; the WebView's localStorage holds only a
warm cache of it for the first paint. localStorage lives in the WebView2 browser profile (by default
`%APPDATA%\PigeonPost.exe`, a directory named after the executable and managed by the browser runtime,
not by PigeonPost), which sits outside the application's own data directory and has proven not to
survive an update or reinstall, so nothing durable may live only there. Classification
gives each well-known role to exactly one folder: the server's RFC 6154 special-use attributes are
authoritative, otherwise the well-known leaf name is used; a name match nested under a different
special folder is rejected, so a stray "Sent" under Drafts never becomes the account Sent. Any stray sent
folders are reconciled into one top-level Sent at the start of each sync.

Mark read/unread and star/flag: the UI calls the facade, which routes through the
`MessageActionService`. It writes the flag (`\Seen` or `\Flagged`) to the IMAP server first (via the
`MailActions` port) and only then updates the local cache, so the change is durable: a later sync
mirrors server state back and preserves it rather than overwriting a local-only flag. The unread
(bold) state and the star follow the cached flags. `UnreadCounts` is the single derived-total choke
point: it reflects the cross-account total onto both the taskbar overlay badge and the tray icon (the
tray icon composites the app icon with the same red count badge, so the count stays visible even when
the window is hidden to the tray). Beside the counts it carries each account's newest unread message
date (`NewestUnreadByAccount`, the same snooze-aware visible set); the front end compares those dates
to per-account "last looked" watermarks it keeps in localStorage (`newMail.ts`, stamped on every
account switch) to light the account picker's elsewhere badge only for mail that arrived after you
last had that account open, so a standing unread backlog never lights a permanent cue.

Search: local, offline, operator-grammar full-text search over the cached mail. The grammar is a
domain concept: `domain.ParseSearchQuery` turns raw input into a modelled `SearchQuery` (bare prefix
words, exact `"phrases"`, `OR` groups, `-negation`, the field operators `from:` `to:` `subject:`
`filename:` and the structural predicates `has:attachment`, `is:unread`/`is:read`/`is:flagged`,
`in:<folder>`, `account:<name>` and `before:`/`after:`/`on:` ISO dates). The parser never fails: an
unclosed quote degrades the whole input to plain free text (flagged so the UI can hint), while an
unknown operator or operand stays literal search text. `MailboxService.Search` applies policy (the
UI's folder or account scope, the result cap) and hands the modelled query to the `MailStore`.

The index is `message_search` (FTS5): subject, snippet, sender, recipients, the plain
body and the attachment filenames. Bodies are cached lazily on first open, so body text becomes
searchable as messages are read (headers and the snippet cover everything from sync); the body-cache
save re-indexes its message in the same transaction. The `message_searchable_text` view is the single
definition of a message's searchable text: every insert site and the schema backfill select from it,
so the indexed shape cannot drift. The index is deliberately self-contained rather than FTS5
external-content: the text spans three tables (`message`, `message_body`, `message_attachment`);
external content requires every delete to reproduce the exact values as indexed, which cross-table
mutation ordering cannot guarantee; self-contained keeps every consistency path an idempotent DELETE
or reinsert by message id, at the cost of the index holding its own copy of the text. Folder, account,
flag and date predicates stay relational (joined in SQL, never indexed), so moves, flag flips and
scoping need no index maintenance at all. Ranking is BM25 with subject and sender boosted over the
body; each hit carries a `snippet()` of the matched text with matches wrapped in control-character
markers the UI splits on, so message content is never interpreted as markup. The consistency contract
(insert, body cache, folder replace, delete, account removal, flag changes) is pinned by the
`search_store_test.go` suite; a future index format change ships as a migration that drops and refills
from the view, the pattern that built the current index. The UI runs the query debounced with a scope selector (all mail, this
folder, this account), highlights matches in the result rows and is reachable via Edit > Search
(Ctrl+K).

Coloured tags: the `TagService` use case manages user-defined tags (a name plus a validated `#rrggbb`
`Colour`) and their many-to-many association with messages, through the `TagStore` port. Tags and the
`message_tag` link table have their own migration; migrations apply incrementally from the recorded
`user_version`. Tags now round-trip onto IMAP keywords so an assignment made on one device reconciles on
another. Each `Tag` carries a frozen keyword, `$PPtag_` followed by the lowercase hex of the tag name's
UTF-8 bytes (domain `KeywordForName`, backfilled into a new `tag.keyword` column by a migration), so the same
tag derives the same keyword everywhere and a rename never rewrites it. Every assign or unassign writes the
local `message_tag` row and a row in the `message_tag_pending` intent table in one SQLite
transaction, so the assignment and its sync intent can never drift. The application `TagSyncService` flushes
each pending intent to the server through the `MailActions.SetKeyword` port (an IMAP STORE of the custom
keyword, retried best-effort until it lands); when a folder is fetched it reconciles the server's own tag
keywords back into local assignments, clearing a pending intent once the server agrees. POP3 accounts skip
all of this by design, since POP3 messages carry no keywords.

Flag changes (read, starred, answered, forwarded) round-trip the same way, through the
`message_flag_pending` intent table and the application `FlagSyncService`. A mark action writes the cache
flag and its pending intent in one transaction (`MailStore.SetFlag`), then pushes to the server
best-effort: a push that fails, offline or against a server that accepts the STORE and drops it, leaves
the intent for every sync to replay (`FlushPending`, which re-resolves the message's current UID at push
time). Before a sync saves fetched summaries it overlays each unconfirmed intent onto them
(`ReconcileFetched`), so a fetch that still reports the old value, which Outlook.com does routinely
because it applies flag STOREs lazily or sheds them, cannot regress a change the user just made; the
intent is cleared only once a fetch shows the server agreeing. This is why viewing a message in the
reader marks it read durably: without the guard, the IDLE-triggered inbox re-fetch would write the
server's stale unseen flag straight back over the cache. POP3 accounts record no intents: their flags
are purely local and the sync's `preserveFlags` carries them across fetches whole.

Filter rules: the `RuleService` use case manages user-defined rules through the `RuleStore` port and the
`RuleExecutor` carries out what they decide. A domain `Rule` is ordered, can be switched off, can be limited
to named accounts and holds several conditions combined by a match mode (all or any) plus several
actions. A rule naming no account covers every account, including one added after the rule was written:
"unscoped" has to mean all, never "the accounts that existed at the time"; otherwise a rule would
quietly stop covering a user's mail the moment they add an address. `RulesForAccount` narrows the set before
evaluation, so scope is enforced in one place rather than checked per message. A condition matches one
field (all fields, From, To, Cc, any recipient, Subject or sender domain) against a value with an operator
(contains, is, starts with, ends with or does not contain), case-insensitively unless the condition sets
its case-sensitivity flag. The all-fields option reaches the sender, every recipient, the subject and the
sender's domain at once; it is what a new condition starts on, alongside an any-of-these match mode,
because narrowing a rule is easier than knowing to widen it. Bcc is deliberately absent: the sending
server strips it, so a received message never carries one and such a condition could never fire. The
actions are mark read, flag, move to a named folder and delete permanently.

Evaluation stays pure: `EvaluateRules` returns one `RuleOutcome` per message describing what should
happen (flags applied, a destination folder, a destruction) and performs no I/O, because a move and a
destruction act on a remote server. Rules are tried in position order; flag actions accumulate, the first
move wins, a destroy ends evaluation for that message and `StopProcessing` ends it after a match.

`RuleExecutor` executes the outcomes during the sync, on messages just fetched and not yet cached, so a
destroyed message never enters the local store: `MailActions.DeleteMany` with an empty trash path marks
`\Deleted` and expunges where the message stands, with no Trash copy and nothing to tidy up afterwards.
Moves are batched per destination through `MoveMany`. Three guards bound the destruction, each pinned by
a test: rules run on the **Inbox only**, so mail the user has already filed by hand is never touched;
they act on **arrivals only** (an id the local store does not hold), so adding a rule never reaches back
over existing mail; and destructive actions are held back until the folder has been **baselined**, the
one pass that records what a folder already holds, which is what stops a newly added account being
emptied by mail that arrived long before the rule. A batch the server refuses leaves its messages in
place and reports the failure.

The baseline is a stored fact (`folder_baseline`, keyed by folder id), read through
`MailStore.FolderBaselined` and written by `MarkFolderBaselined` only after the fetched messages are
safely saved: a folder marked ahead of a failed save would have its whole backlog read as arrivals on
the next pass. It is deliberately not inferred from the cached messages. That inference, "the local
store holds no messages for this folder", is equally true of an inbox the user has simply emptied, so
it exempted every message arriving into a filed-clean mailbox from the destructive actions, silently
and every time; for anyone who keeps their inbox at zero a destroying rule then essentially never ran.
It also cannot live on the folder row, because `SaveFolders` clears and rewrites every folder for an
account on each sync and would take the mark with it, re-arming the exemption forever.

Because a rule runs unattended, the confirmation for a destructive action moves to rule-creation time:
the UI warns before saving a rule that moves or destroys mail and marks a destroying rule in the list.
Conditions and actions live in the `rule_condition` and `rule_action` child tables keyed by rule id and
ordered by position (`schemaV50`); rules written before that carry over verbatim as one-condition,
one-action rules, their stored field, operator and action integers unchanged. `schemaV51` adds the
per-condition case-sensitivity flag, defaulting to 0 so an existing condition keeps comparing
case-insensitively, which is all any of them ever did. `schemaV52` adds the `rule_account` scope table;
a rule with no row there applies everywhere, so nothing needed backfilling. `schemaV53` adds
`folder_baseline` and marks every folder already in the database, since those installations established
their baseline through ordinary use; a folder written after that step, which is what a newly added
account produces, has no row and so still gets its one protected pass.

The editor keeps the scope and a move destination consistent: a scoped rule is offered only folders in
the accounts it covers; narrowing the scope clears a destination that falls outside it. Without
that, a rule could name a folder it can never reach and the move would silently do nothing on every
sync; clearing it instead blocks the save until a reachable folder is chosen.

**Update check.** The application `UpdateService` compares the embedded VERSION against the newest
published GitHub release through the `ReleaseSource` port, implemented by
`infrastructure/update.GitHubReleaseSource` (a 5 second `net/http` GET of the latest-release
endpoint, which reports published, non-draft, non-prerelease releases only, so a tag pushed
mid-development can never prompt; the HTTP client is injected for tests). The service picks the
download asset for the running OS by filename suffix (`.exe` / `.dmg` / `.flatpak`), reports an
unreachable source as no update and treats a version the user skipped as seen but not available.
The facade binding (`updatesapi.go`, `CheckForUpdates`) runs the check off the main thread as every
Wails bound call is; the front end owns the triggers (`useUpdateCheck`: a launch check shortly
after start, a daily re-check and the manual Help / tray path, which ignores the skip) and renders
the outcome in `UpdateModal` (Download via the existing `OpenExternal` scheme allowlist, Skip This
Version persisted in localStorage with the other presentation preferences, Later). Automatic checks
surface nothing on failure or when up to date; the manual check reports both.

## Errors

Wrapped with `fmt.Errorf("...: %w", err)` and matched with `errors.Is` against sentinel errors. No
custom error types beyond sentinels.

## Quality enforcement

- `internal/domain` and `internal/application` at 100% test coverage, enforced by `./test.ps1`, which
  fails the run when either drops below it. A bare `go test ./...` does not apply the gate.
- Application use cases tested against hand-written fakes (no mock libraries).
- Infrastructure tested against a real SQLite database in a temp directory.
- Structural AST tests enforce layering, domain purity, the module-size limit and the composition
  root whitelist.
- The React front end has its own Vitest and jsdom suite: a coverage gate on the pure logic modules and
  a structural boundary test that keeps them pure.

## Styles (frontend)

`frontend/src/App.css` is a manifest: nothing but `@import` lines pointing at per-concern files under
`frontend/src/styles/`, listed in the order the sections had in the original single stylesheet so the
cascade is unchanged. The split is by concern, not by component, because shared globals (`.btn`, `.modal`,
`.icon-btn`, the theme variables and the focus/hover rules) belong to no single component.

Rule for new styles: add a file under `frontend/src/styles/` and `@import` it from `App.css` in the right
place. Keep `App.css` a manifest (only `@import` lines); never inline component rules back into it or let a
per-component file own a shared global. Split a concern file over ~500 lines again at a top-level comment
boundary, keeping the import order intact.

One sidebar layout rule: `.pane.sidebar` disables the pane's own overflow and scrolls an inner
`.sidebar-scroll` region holding the folder tree alone, so the brand icon, the cross-account entries, the
account picker and the Folders header (all inside the pinned `.sidebar-header`) stay put while the folders
scroll beneath them. The accounts section is one dropdown rather than a list precisely so the folders keep
that space. A new sidebar section belongs in `.sidebar-header` unless it is meant to scroll with the
folders. `Sidebar.test.tsx` pins this structure.

That dropdown is a listbox built in `AccountPicker.tsx`, not a native `select`, for one behavioural
reason: a native select is silent when the option already showing is re-picked, so choosing the account
you are already in did nothing, when it should take you back to that account's inbox. Every pick reports
its account and App opens that account's inbox from there, current one included. It remains a single
focus-ring stop as the select was: focus rests on the trigger and never enters the popup, with Up and Down
walking the options through `aria-activedescendant`, Enter picking, Escape dismissing and Tab or the
horizontal arrows closing it as the ring steps away. The closed trigger also carries the elsewhere
badge (outlined, matching the folder tree's rolled-up badge: an outline means "not here") summing the
unread on accounts with mail newer than their watermarks, with the per-account breakdown one click
away in the open list.

## Calendar and contacts

This section records the shape of the address book and calendar; each piece is held to
the same layer rules and tests as the body above.
The invariant is unchanged: `UI -> Application -> Domain <- Infrastructure`, same layer rules, same
composition root, same 100% domain gate. The address book is built before the calendar because it is
the simpler half (no recurrence, timezones or RRULE) and it exercises the shared import/export seam the
calendar then reuses.

**Domain.** New pure value objects, immutable and validated on construction like the mail entities.
Address book first: `Contact` (id, vCard UID for lossless round-trip, formatted name, given/family
name, organisation, title, note and slices of `ContactEmail` and `ContactPhone`, each a labelled
value) and `ContactGroup` (id, name, member contact ids, with `With*` copy methods for membership).
Calendar: `Calendar` and `Event` (id, ICS UID, summary, start/end, all-day flag, location,
description and an optional recurrence rule), with time entering only as already-resolved values, the
domain still reads no wall clock.

**Application.** New ports mirroring the mail stores: `ContactStore` (list, get, save, delete contacts
and groups) and `CalendarStore` (calendars, events and preserved passthrough components). Import and export sit behind a codec seam
so the use case is format-agnostic: a `ContactCodec` interface with `Decode([]byte) ([]domain.Contact,
error)` and `Encode([]domain.Contact) ([]byte, error)`, implemented once per format and a
`CalendarCodec` likewise. An `ImportContacts` / `ExportContacts` use case selects the codec by the
chosen format and reconciles by UID so a re-import updates rather than duplicates.

**Infrastructure.** New adapters implementing the ports: `storage` gains `contact`, `contact_email`,
`contact_phone`, `contact_group` and `contact_group_member` tables, plus `calendar` and
`event` tables. Codec adapters: `vcard` (emersion/go-vcard) and `csv`
(stdlib `encoding/csv`) for contacts, plus `ical` (emersion/go-ical) for calendar. Two contact codecs
exist deliberately: vCard covers Thunderbird and single-contact Outlook; CSV covers Outlook's bulk
contact export/import (Outlook exports the address book as CSV, not vCard; Thunderbird reads CSV too).
The pure decode/encode logic lives in these packages and is covered to 100%; only genuine file or OS
edges are excluded.

**Automatic collection.** `ContactService.CollectAddresses` adds a minimal contact (the address as
its display name) for each given address not already anywhere in the address book, case-insensitively
and best-effort: a malformed address is skipped, never an error. The front end calls it fire-and-forget
after a successful send with the message's recipients (the sender's own addresses filtered out by the
gated pure `autoCollect` module), gated by a locally persisted on-by-default setting toggled on the
Contacts page. Collection is a side effect of sending, so it can never fail a send that succeeded.

The `csv` package is split by concern because the two exporters agree on almost nothing: `mapping.go`
holds the column-alias tables and the row-to-contact rules, `encoding.go` normalises input to UTF-8
(neither exporter reliably writes it; a byte-order mark left in place binds to the first header
and silently drops that column), `dates.go` normalises birthdays to the ISO form the editor accepts
and `csv.go` orchestrates. Reconciling a re-import is deliberately NOT the codec's job: CSV carries no
stable per-contact id, so matching is a policy over the whole address book and lives in
`ContactService.ImportContacts`, which matches on id or shared email address and merges through the
pure `Contact.MergedWith` in the domain.

**UI.** A contacts dialog and calendar month, week and day views, both clients of the Application
use cases only. The contacts dialog is wide, laying the address book out as a three-column card
grid; each card opens its contact on click or via its edit pencil; deletion lives at the end of
the open contact's editor beside Save (still behind the confirm-before-delete rule) rather than on
the list rows. A postal address in the editor is a three-column field grid with its remove control
alongside, so it cannot be squeezed out of view. Date entry
everywhere in the app (a contact's birthday, an event's start and end, repeat-until, send later and
the snooze picker) is the shared `DateField` component: the native input stays for typing, while its
calendar button opens the themed `DatePickerDialog` instead of the engine's minimal native picker;
the dialog's month-grid model is the gated pure `datePicker` module and picking a day merges only
the date, so a datetime field keeps the time already typed. The week and day
views are an hour time-grid: an all-day strip, timed events sized by start and end, clashing events in
side-by-side lanes. The month view lays each week's events out as bars: a multi-day event spans the day
columns it covers as one continuous bar, squared where a week boundary clips it and stacked in lanes above
the single-day chips, with a "+N more" when a day overflows; the span and lane placement is pure and tested
in `calendarModel`.

**Interop acceptance.** A real export from Outlook and from Thunderbird imports cleanly into PigeonPost;
a PigeonPost export imports back into both without loss, for calendar (ICS) and contacts (vCard and CSV).

**Calendar recurrence (RFC 5545 expansion).** The `Event` now models the whole recurrence set: the raw
RRULE plus RDATE and EXDATE occurrence lists and a RECURRENCE-ID for an override event, all as
already-resolved values so the domain still reads no wall clock (the date lists are stored as
comma-separated Unix milliseconds and the recurrence id as milliseconds). Expansion needs an RRULE
parser, which the domain must not depend on, so it lives behind a new Application port,
`RecurrenceService` (`Expand` an event into `EventInstance` occurrences within a window; `TruncateBefore`
rewrite a rule to end before a time), implemented in `infrastructure/recurrence` over the pure-Go
`teambition/rrule-go` library. `CalendarService.ListEventInstances(from, to)` groups events by series
(UID or id when absent), expands each master, suppresses the generated occurrence an override replaces
and merges one-off events, all sorted by start; a malformed rule degrades to a single instance rather
than losing the event. Editing or deleting a recurring occurrence carries a scope (this, this-and-future,
all): `this` writes a single-occurrence override, `future` truncates the master with UNTIL and starts a
new series from the split (migrating later overrides) and `all` rewrites the master. When the split
leaves the recurrence unchanged, `SplitCountForward` reduces a COUNT-based rule by the occurrences before
the split so the forward series carries the remaining count and the two halves keep the original total
(an open-ended or UNTIL rule needs no adjustment; a rule the user changed is honoured as given). The `ics` codec
extracts and re-emits RDATE, EXDATE and RECURRENCE-ID alongside the existing opaque `Extra`
pass-through, so the round-trip stays lossless.

**Event timezones.** An `Event` also carries an IANA zone (the `time_zone` column), so a recurring event
keeps its local wall-clock time across daylight-saving changes: its Start and End stay absolute instants;
the zone says how they are shown and expanded. The expander anchors DTSTART in that zone before
generating, so a 9am daily event stays 9am local while its UTC instant shifts across the DST boundary;
the IANA database is embedded (`time/tzdata`) so `LoadLocation` resolves on Windows. The `ics` codec reads
the `TZID` parameter on import and writes `DTSTART;TZID=...` on export (the IANA name, which Google,
Outlook and Thunderbird resolve from their own databases); a UTC or all-day event carries no zone. On the
front end a zone picker sets the event zone, the form interprets and shows its wall-clock times in that
zone; occurrences render in the browser's local zone. Export also emits a `VTIMEZONE` for every zone
the events use, so the file defines the zones its `TZID` parameters reference rather than relying on the
reading application's own database. Each is generated by probing the zone across the earliest event's
year to find its standard and daylight offsets and the transitions between them, then writing STANDARD
and DAYLIGHT sub-components with an RRULE derived from each transition date (a zone without daylight
saving gets a single STANDARD). RDATE, EXDATE and RECURRENCE-ID are written as UTC instants.

**To-dos and journals.** The `ics` codec models only VEVENTs but a VTODO or VJOURNAL is preserved
verbatim as a `domain.CalendarPassthrough` (UID, kind, the component re-serialised as a standalone
VCALENDAR) rather than dropped. `Decode` returns passthrough alongside the events; `ImportEvents` stores
each in the `calendar_passthrough` table (keyed by UID so a re-import replaces); and
`ExportEvents` re-emits them. So an imported calendar's tasks and notes survive an import and export
round-trip even though PigeonPost does not yet display them.

**Reminders.** An `Event` carries a list of `Alarm` reminders, each a signed trigger offset from the start
(stored as comma-separated seconds; the facade exposes them to the UI as whole
minutes-before). The `ics` codec reads relative-trigger `VALARM` children into alarms and re-emits one
`DISPLAY VALARM` per modelled alarm with a friendly duration (`-PT15M`, not the library's `-PT900S`);
because it owns the property it strips existing VALARMs first, so an exotic imported alarm (an absolute
trigger, an email action) is not preserved.

**Reminder scheduling.** `CalendarService.DueReminders(since, now)` expands events and
returns the reminders whose trigger falls in that window; a scheduler goroutine in the composition root
polls it every thirty seconds and emits a Wails event that the front end shows as an on-screen banner. On
launch it first calls `PendingReminders(now)`, which fires reminders for still-imminent events (starting
at or after now) whose trigger lapsed while the app was closed, so a reminder for an upcoming event is not
missed; a reminder for an event already started or past is not resurrected; the catch-up and live
windows do not overlap.

**Alerting.** When a batch of reminders fires, the composition root also draws attention from
outside the window: it flashes the taskbar button through an injected `ReminderAlerter` (the `taskbar`
package's `Flasher`, a build-tagged no-op off Windows) and raises a notification through the `taskbar`
package's `Tray`. The tray notification is a Windows balloon on the tray icon or a native desktop
notification off Windows (a freedesktop D-Bus notification on Linux, an `osascript` notification on
macOS, a no-op on any other platform). Both alerts skip when the window is already in the foreground, so
an in-view reminder relies on its banner alone.

**The notification sound.** On Windows a balloon raised through `Shell_NotifyIconW` plays the shell's
one default notification sound, which is the same sound every other app and tool raising a stock
notification gets, so a PigeonPost alert cannot be told apart by ear from anything else on the machine.
The tray therefore sets `NIIF_NOSOUND` to silence the shell and plays its own chime through
`infrastructure/sound`, a soft falling pair pitched low and wooden where the system sounds are bright
and glassy. The chime is synthesised from named constants rather than shipped as an asset, so there is
no binary in the repository and nothing to resolve at runtime across the dev, Wails and packaged
builds; the synthesis is a pure function and is unit tested, leaving only the `winmm` `PlaySound` call
itself outside coverage. Playback is asynchronous and reads the buffer after the call returns, which is
safe because the rendering is cached once for the life of the process. Every tray notification shares
the one chime (new mail, a due reminder and a snoozed message returning), so PigeonPost has a single
recognisable voice. Off Windows the sound is left to the desktop's own notification service, which
chooses it from the user's theme, so there is nothing to override.

**Close to tray.** On Windows the `Tray` is a persistent, clickable
notification-area icon: left-clicking it reopens the window; its right-click menu mirrors the Help
menu (About, Licence, Check for Updates) plus Open and Quit. Where a restorable tray icon exists (only
Windows, gated by `Tray.CanHideToTray`), the window's close button does not quit: `OnBeforeClose` keeps
the window open and emits `app:close-request`; the front end shows its own dark-themed dialog
offering Minimise to tray or Quit. The dialog renders last in App's overlay list on a raised backdrop
(`modal-backdrop top`), so it surfaces above whatever dialog is open when the close button is pressed
rather than painting beneath a later-mounted sibling; the Escape stack still closes one layer at a
time. When another dialog is open as it appears (a message being written, a contact being edited), it
samples that fact once on open, states plainly that unsaved work may be lost on quit and offers Go
back as the focused default so the user can finish or save first; Quit stays one deliberate click
away. Minimise calls `MinimiseToTray` (which hides the window so the
scheduler and mail sync keep running in the background); Quit calls `RequestQuit`; dismissing the dialog
leaves the window open. A native dialog is deliberately avoided so the prompt matches the app theme.
Where no tray icon exists the close button simply quits. The tray menu's Quit sets a flag so it exits
without re-triggering that prompt, since it drives the same close path. To keep the `taskbar` package
free of any UI-framework dependency, the tray's Open and menu items invoke callbacks supplied by the
`App` facade, which reopen the window (`WindowShow`), quit or emit `menu:*` Wails events the front end
turns into the same dialogs the in-window Help menu opens.

**Meeting scheduling (iTIP / iMIP).** An event with attendees is a meeting; PigeonPost sends and
receives the RFC 5546 scheduling messages (REQUEST, REPLY, CANCEL) as RFC 6047 iMIP `text/calendar` mail
parts. New pure domain value objects carry the data: `Organizer` (a validated address plus an optional
common name) and `Attendee` (address, common name, a `Role` and a `ParticipationStatus` enum each parsed
leniently and an RSVP flag with a `WithStatus` copy method), with `Event` gaining an organiser and an
attendee list (stored as an `event.organizer` column and a JSON `event.attendees`
column). A `scheduling.go` domain file adds the `Method` enum, the `SchedulingMessage` (a method plus its
events) and the `CalendarPart` (a method plus the encoded bytes) that an outgoing message carries. These
value objects and their parse rules are held to the 100% domain gate.

The codec seam gains a `SchedulingCodec` port (`DecodeScheduling` reads a VCALENDAR's METHOD and events;
`EncodeRequest`, `EncodeReply` and `EncodeCancel` build the payloads), satisfied by the same `ics`
adapter over go-ical. The `SchedulingService` use case (application layer, 100% gated) drives the flows:
`Respond` saves an incoming REQUEST to the calendar with the chosen PARTSTAT and emails a REPLY to the
organiser; `ApplyReply` folds an incoming REPLY into the organiser's stored meeting, recording the
responder's status on every event the reply covers (the named occurrence or the series master plus
every override when it names none) and appending a responder the meeting does not list (a delegate or
a guest answering from a different address) rather than dropping the response; `ApplyCancellation`
removes the meeting a CANCEL withdraws; and `SendRequest` / `SendCancel` email a REQUEST or CANCEL to a
meeting's attendees from the organising account. A recurring meeting is matched as its series master
plus any overrides, keyed by UID and RECURRENCE-ID. Every scheduling send (`scheduling_send.go`) leaves
the same record an ordinary composed message does: the shared `saveCopyToSent` helper appends a
best-effort copy to the account's Sent mailbox (skipped for providers that save sent mail server-side),
an unreachable server queues the message in the offline outbox for the compose dispatcher to replay
(outbox rows persist the iMIP calendar part so the replayed message keeps its payload); a
successful response marks the invite message answered. `Invitation` resolves an invite for display by
overlaying attendee statuses from the stored calendar copy of the meeting, which is where `Respond`
records the recipient's answer and `ApplyReply` lands everyone else's, so the card shows the current
truth rather than the email's frozen ICS.

Arriving scheduling mail is folded in automatically by `ApplyIncoming` (`scheduling_apply.go`), fed by
the new-mail notifier for every fresh message. It distinguishes changed (the calendar moved, so the
front end reloads it) from resolved (the message needed nothing from the user, so it is marked read):
a REPLY and a CANCEL are both; an updated REQUEST for a meeting already held locally is changed but
never resolved, because it may carry alterations the user must still look at. The update path folds
only the organiser's attendee-status snapshot into the stored meeting (`applyRequestStatuses`), never
the meeting's content: statuses of attendees other than the recipient, matched by address, with a
NEEDS-ACTION snapshot value never downgrading a recorded response and the recipient's own row left to
their local answer. This is deliberate iTIP shape: an attendee's reply travels only to the organiser,
so the organiser's updated REQUEST is the sole channel through which one attendee learns another's
response. The organiser themselves need not appear in the attendee list; their participation is
implicit in the ORGANIZER property and no attendee row is invented for them.

Mail carries the invites both ways. Incoming: the shared `mailparse` parser diverts a `text/calendar`
part into a `ParsedBody.Invite`; the cached `MessageBody` gains an `invite` column with
`HasInvite` / `Invite`, so a message reading offline still shows its invitation. The `MailSource.FetchBody`
port and both the IMAP and POP3 adapters return the raw calendar bytes alongside the plain and HTML
parts. Outgoing: an `OutgoingMessage` carries an optional `CalendarPart`; the shared `message` MIME
builder writes it as a `text/calendar; method=...; charset=utf-8` part inside the `multipart/mixed` body,
so one sent message is both a readable email and a valid iMIP scheduling message.

The Wails facade (`schedulingapi.go`) exposes the flow through `OrganizerDTO`, `AttendeeDTO` and
`InvitationDTO` and the methods `GetInvitation`, `RespondToInvitation`, `RemoveCancelledMeeting`,
`ApplyMeetingReply`, `SendMeetingRequest` and `SendMeetingCancel`; `EventDTO` and `EventRequest` carry the
organiser and attendees so a meeting round-trips through the calendar editor. As with the rest of the
facade, these binding files are build-verified (they hold no logic beyond DTO mapping) rather than
unit-tested; the correctness lives in the domain and application layers behind them. In the UI the reader
shows an invite card (Accept, Tentative or Decline a request, remove a cancellation, apply a reply) and
the calendar event editor edits a meeting's attendee list and sends its invitations and cancellations.
Re-saving an existing meeting emails an update only when a field the attendees can see changed: the form
snapshots the attendee-visible slice of the event on open (title, times, zone, location, description,
category, recurrence and the attendee list, the fields the encoded REQUEST carries) and compares it on
save, so a reminder or local calendar tweak saves without emailing an update; the primary button and the
hint text state in advance whether the save will email the attendees. A
join link an invite carries in its location or description (Microsoft Teams, Google Meet, Zoom or Webex,
matched by host) surfaces as a Join button in the event editor; any other link in the description is
clickable; both open in the external browser through the existing `OpenExternal` facade method, so this
adds no new port. The invite card also guards against pointless resends: clicking the answer already on
record shows an inline confirmation naming the recorded response before an identical REPLY is sent
again, while a changed answer sends immediately.

**New-mail notifications and IMAP IDLE.** New mail is surfaced the moment it arrives. A
`runMailNotifier` goroutine in the composition root owns the flow: it primes a baseline first (an existing
inbox is cached, not announced), then feeds two detection paths through one serialised `checkMail`, so a
push and the backstop poll can never double-notify. `SyncInboxes` (application) refreshes every account's
inbox and returns the messages whose id is not already cached, keyed on arrival rather than read state, so
a message another client already marked read still counts while only a filter-rule read-on-arrival is
silenced. A new message raises a desktop notification through the same `taskbar` `Tray` the reminders use,
forced to show even when the window is focused because new mail has no in-window cue.

Instant delivery is an IMAP IDLE watcher. `infrastructure/imap`'s `Watcher` holds a persistent,
authenticated IDLE connection per IMAP account and invokes a callback the moment the server reports the
mailbox changed, reconnecting with capped exponential backoff and reissuing the IDLE inside the server's
timeout window; a server without the IDLE capability stops cleanly and is left to the poll. The watcher is
injected into the facade behind a `MailWatcher` port, so the application layer keeps no IMAP dependency. A
60-second poll is the backstop for a missed push and for POP3, which has no IDLE.

The watcher set is kept in step with the accounts, so an account added after launch gets instant push
without a restart. Each account's watcher runs under its own cancellable child of the app context, tracked
by id: `AddAccount` starts one, `UpdateAccount` restarts it so changed server settings take effect (and a
switch to POP3 drops the IMAP watcher) and `RemoveAccount` stops it so no stale connection is left.
Shutdown cancels the app context and stops them all. A fired reminder banner is clickable, opening the
calendar on that event through the existing calendar binding.

Only one PigeonPost runs per user, enforced by Wails' `SingleInstanceLock` (a named mutex on Windows).
A second launch does not open a new window: the running instance's `OnSecondInstanceLaunch` reveals its
window through the same `WindowShow`/`WindowUnminimise` path the tray uses, so relaunching an app hidden
in the tray simply brings it back.

A clicked `mailto:` link or opened `.eml` reaches the app by two platform routes into one shared
mechanism. On Windows the payload arrives as a command-line argument (of the cold launch or the second
instance); on macOS it arrives as an Apple Event through the Wails `Mac.OnUrlOpen` and `Mac.OnFileOpen`
handlers, never as argv. Both routes park a payload that beats the front end in the same mutex-guarded
pending slots that `domReady` flushes, so a cold start opens the compose or viewer exactly once the UI
can receive it. The macOS bundle declares the `mailto` scheme and the `eml` extension through
`wails.json` (`info.protocols`, `info.fileAssociations`), which the `build/darwin/Info.plist` template
renders as `CFBundleURLTypes` and `CFBundleDocumentTypes`, making PigeonPost selectable as the system
default mail reader. On Linux the Flatpak's desktop entry claims `x-scheme-handler/mailto` with a `%u`
field code, so GNOME's Default Apps offers PigeonPost as the email client and a clicked link arrives as
the same argv payload the Windows route uses (or through the single-instance forward when the app is
already running). Claiming `message/rfc822` (.eml) on Linux is deferred: a file-manager launch hands a
`file://` URI the `.eml` opener does not parse.

## Design decisions

The standing choices behind the stack and the product shape, recorded so they are not relitigated.
The feature backlog beyond these decisions (parked candidates and confirmed won't-dos) is triaged in
[FEATURES_PLAN.md](FEATURES_PLAN.md).

Go + Wails + React was chosen over Rust + Tauri because the Emersion Go mail suite covers the entire
email, calendar and contacts surface in one coherent family (including CalDAV/CardDAV via go-webdav),
it matches the proven native-Go-on-Wails delivery lineage (locus, focus-reader) and it avoids learning
async Rust under load on a protocol-heavy app. All chosen dependencies are permissive (MIT/BSD) and
GPL-3.0 compatible.

| Concern | Choice | Rationale |
|---|---|---|
| Shell | Wails v2 (WebView2/WebKit hosting React + TS) | Proven delivery lineage; single small binary. |
| Backend | Go 1.25+ | Second first-class language; native cross-platform. The floor is the `go` directive in `go.mod`. |
| IMAP | emersion/go-imap (v2, IDLE) | Async push, mature. |
| POP3 | small hand-rolled client | POP3 is a small protocol. |
| SMTP send | emersion/go-smtp | Pairs with the suite. |
| MIME parse/build | emersion/go-message | Production-tested in real clients. |
| SASL | emersion/go-sasl | SASL PLAIN for SMTP AUTH. |
| Calendar ICS | emersion/go-ical | RFC 5545 round-trip (Thunderbird and Outlook). |
| Contacts vCard | emersion/go-vcard | vCard 3/4 round-trip. |
| Contacts CSV | stdlib encoding/csv | Outlook bulk contact import/export (Outlook exports CSV, not vCard). |
| CalDAV / CardDAV | emersion/go-webdav | Two-way sync affordable in Go. |
| Storage | modernc.org/sqlite (pure Go, no CGO) + FTS5 | Local-first, single-writer/multi-reader. |
| Credentials | zalando/go-keyring | OS keychain; never in the DB. |
| Front end | React 18 + TypeScript (Vite) | Existing React/TS + Wails lineage. |
| List virtualisation | @tanstack/react-virtual | 100k-message folders scroll smoothly. |
| Drag/drop | native HTML5 drag-and-drop | Message-to-folder, folder reparent and reorder. The pane's edge auto-scroll and the drop confirmation are the app's own; the engine's are unusable or absent. |
| Rich-text compose | TipTap (ProseMirror) | Clean, sanitisable, email-safe HTML. |
| HTML mail render | sandboxed iframe + sanitiser | Untrusted HTML is the top security surface. |

Locked product decisions:

- Licence: GPL-3.0, whole app, with credit to the original author (Oliver Ernster) retained in all
  copies and derivative works (an author-attribution additional term of the kind GPLv3 section 7(b)
  permits). Removing or omitting the attribution is not permitted. The term is stated in the LICENSE
  file's own licensing notice (shown by Help > Licence, which renders that file) and repeated in
  Help > About.
- Auth: password (or app password) over generic IMAP/POP3 is the core path; XOAUTH2 OAuth is
  implemented and Microsoft uses it (authorization-code plus PKCE, loopback redirect, free Entra
  registration). Gmail personal accounts are supported via an app-password preset; one-click Google
  sign-in is declined (annual CASA fee) and Workspace accounts are OAuth-only, so they are not covered
  by the personal preset.
- Calendar/contacts: file ICS/vCard import/export and two-way CalDAV calendar sync are delivered;
  two-way CardDAV contact sync is planned. The delivered CalDAV sync has
  not yet been exercised against a live server, so its per-provider edges (ETag and href formats,
  whether 412 is the conflict status, whether a server accepts a client-chosen object name, CTag
  support) are unproven until a first real account exercises them.
- Compose: light TipTap rich-text plus a plain-text toggle; no full HTML-editor parity.
- Inboxes: each account keeps its own separate inbox in storage; the unified mailbox is a read-side
  merge of the cached inboxes (a synthetic folder in the UI, aggregation in gated application code),
  never a storage-level combination. Move, copy and junk stay per-account actions, so the combined view
  does not offer them.
