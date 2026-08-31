# <img width="64" height="64" alt="pigeonpost" src="https://github.com/user-attachments/assets/fcc90cad-786e-4d04-a7a9-6f5d82be309d" /> PigeonPost

A cross-platform desktop email, calendar and address book client. Go core, React front end,
local-first. A calmer, more predictable alternative to Thunderbird.

![Released](https://img.shields.io/badge/status-released-brightgreen)

## Who it is for

- IMAP/POP3 mailbox users (self-hosted, ISP, Fastmail, corporate) who want a fast, native,
  local-first desktop client.
- Gmail, iCloud, Yahoo, Zoho, Fastmail and StartMail users who connect with an app password (the
  setup wizard fills in the servers).
- Microsoft users (Outlook.com, Hotmail, Live, Microsoft 365) who sign in through Microsoft OAuth in
  the browser; the refresh token is kept in the OS keychain. Microsoft ships a new Outlook.com or
  Hotmail mailbox with IMAP switched off, so turn it on first. See "Turning on IMAP for a Microsoft
  account" below, which is worth reading before you start if the mailbox is new.

## Who it is not for

- Webmail-only users who never want a desktop install.
- Google Workspace (work/school) accounts: OAuth-only, so an app password will not work. Personal
  Gmail works via an app password; only the one-click "Sign in with Google" is declined, because its
  full-mail scope carries a paid annual assessment.

## Turning on IMAP for a Microsoft account

PigeonPost reads Microsoft mail over IMAP. Microsoft switches IMAP off on every new Outlook.com and
Hotmail mailbox, so a sign-in can succeed and the account still refuse to add. Turn it on at
outlook.com: open Settings (the gear, top right), then Mail, then either "Sync email" or "Forwarding
and IMAP" depending on which Outlook you have, then switch on "Let devices and apps use IMAP" and
save.

A new mailbox puts three obstacles in the way of that; none of them announces itself.

**The page may show no switches.** Instead you get an advert for the Outlook mobile app and a Sign in
button. Microsoft documents this for the same panel: "You might be prompted to Sign in on the
Forwarding and IMAP page. If so, you will need to complete authentication before the ... settings
become available." Press Sign in, authenticate, then the switches appear.

**The switch may revert.** On a mailbox only a day or two old it accepts the change, saves it, then
puts itself back to off. This is Microsoft holding IMAP down on new accounts while they build
reputation. Nothing at your end clears it: browser changes, private windows and cache clearing are
all reported not to help. The only reported cure is time, usually 10 to 24 hours from account
creation and occasionally longer. Leave it, switch IMAP on again the next day, confirm it has stayed
on, then add the account.

**A new mailbox may be refused anyway.** With IMAP on and staying on, Microsoft can still accept the
sign-in and then refuse the session, answering "User is authenticated but not connected". This is a
fault at Microsoft's end and there is nothing PigeonPost can do about it: the endpoint, the port and
the scopes are the ones Microsoft documents; mailboxes more than a few days old connect normally
on the same build. The same failure is reported publicly against consumer Outlook.com accounts with an
identical configuration and has been unanswered by Microsoft since December 2024. If you have just
created the mailbox, give it a few days and try again. An established Outlook.com or Hotmail account
is unaffected.

You need only IMAP. Leave "Let devices and apps use POP" off, because PigeonPost never uses POP for a
Microsoft account.

If an account still will not add, `mail-errors.log` beside the database holds what the mail server
actually said, one failure per line. PigeonPost replaces a server's own words with a message naming a
setting and the steps, which is easier to act on but asserts a cause that can be wrong; the log is the
copy kept so a wrong reading can be seen for what it is. It records only the errors that were
replaced, it is created on the first one and it holds mail server responses and the address they
concern, so read it before attaching it to anything.

## Capabilities

- **Accounts**: IMAP and POP3 from a two-step wizard (presets for Gmail, iCloud, Yahoo, Zoho, Fastmail
  and StartMail, plus a manual host/port/security path), credentials verified before saving. A preset
  whose provider needs an app password says so plainly (a normal login password will not work) and links
  to the page that creates one. Microsoft accounts
  via one-click OAuth. Each account keeps its own inbox, with an optional unified mailbox (a View tick)
  that merges every inbox into one list, each row dotted with its account's colour; replies from it send
  from the row's own account. Send-as addresses. The accounts section is a dropdown holding the account
  you are in, badged with its unread count; picking an account opens its inbox, including the account you
  are already in; the app reopens on the account you left. Mail arriving on an account other than
  the one you are in lights a second, outlined badge on the closed dropdown: it counts only mail newer
  than your last visit to that account, so a standing backlog never lights it; opening the dropdown
  shows which accounts it means. POP3 downloads into one local mailbox with read
  and star marks kept locally.
- **Mail**: folders and summaries cached to local SQLite and read offline; bodies fetched on open and
  cached. HTML renders faithfully in a sandboxed frame that keeps the sender's own styles while running
  no scripts and making no remote requests; remote images are blocked by default until you load them
  (per message or a "Load images by default" toggle), which also restores the backgrounds a message styles
  with CSS. In the dark theme the reader darkens a message region by region rather than all at once: a block
  its sender already designed dark is left in that design and a light block is darkened, so a message that
  mixes the two (a dark brand panel above a white footer, say) reads correctly throughout, with photos and
  logos keeping their true colour. Text a sender coloured for a background image that has not loaded is
  corrected so it stays readable rather than disappearing into its own background. Bare web addresses in a message are clickable (opening in
  your browser, never inside the app), markdown-style `[label](url)` links show their label and a link
  standing alone on its own line is presented as a button. Attachments save from the reader (cached for offline). Sync runs per account with its own
  progress cue.
- **Compose**: TipTap rich text, To/Cc/Bcc, reply, reply-all and forward (Ctrl+R, Ctrl+Shift+R and
  Ctrl+L), file or message attachments
  (25 MB), reusable message templates, per-account signatures and server Drafts. A saved draft
  reopens for editing from the Drafts folder (double-click or Enter on it, else Edit draft in the
  reader) exactly as it was saved; finishing it, by sending or saving again, replaces the stored
  copy rather than leaving a stale one behind. The compose window can be dragged by its title bar
  to uncover the message beneath it; it always opens centred. Typing in To, Cc
  or Bcc suggests matching addresses from your contacts (accept with Enter, Tab or a click); a
  suggestion just inserts text, so you can edit it freely afterwards; accepting with Enter or a
  click adds the separator too, so you can type the next address straight away. Typing a space after
  a complete address does the same, so a list flows without punctuation. Each suggestion shows the
  contact's name beside its address on one line (long values shorten with an ellipsis) and a contact
  whose name is just its own address shows it once. Paste an image
  (a screenshot, a copied picture, an image file) and it embeds in the body at the cursor, keeping
  its original bytes and sent as a proper inline image every mail client renders; paste or drop any
  other kind of file and it attaches. The one rule: images embed, files attach; the 25 MB limit
  counts embedded images too and forwarding a message carries its embedded images along. In-progress writing autosaves locally and is
  offered back after a crash; closing a message you have edited (by any route, including a click
  outside the window) asks before discarding it. Send later schedules a message for a chosen moment (presets or an
  exact date and time); it waits in the Outbox with Cancel send and leaves while the app is running or
  at the next launch after the chosen time. Sends made offline queue in a per-account outbox and deliver on the next sync.
  URLs you type or paste go out as real links in any recipient's client and long lines are encoded so
  no mail server can fold and corrupt them in transit.
- **Organise**: mark read/star, delete to Trash or purge, junk and not-junk (a wrongly junked message
  moves back to the inbox with the server told the verdict), instant offline full-text search with
  operators (from:, to:, has:attachment, dates and more) and colour tags
  that sync across devices as IMAP keywords. Snooze hides a message until a chosen time then returns it
  untouched, announced in the window as a banner you can click to open it and on the desktop as a
  notification (while the app is running or at the next launch); hidden messages wait in a Snoozed view
  with their due times and an Unsnooze. Move or copy messages by menu or by dragging onto a folder:
  drag one message or a whole selection, however you picked it (Ctrl-click for scattered rows, Shift-click
  for a run); dragging any row of that selection takes all of it. A
  dragged message leaves the list the moment you drop it rather than when the server has finished, the
  folder that took it flashes twice so you can see where it went, dragging onto a collapsed folder springs
  it open and holding a drag near the top or bottom of the folder pane scrolls it. Undo and redo (Ctrl+Z, Ctrl+Y) unwind the mail
  actions: delete, move, junk and its rescue, their bulk forms and the read, star and tag toggles,
  with each menu entry naming what it will unwind. Cut, copy and paste messages file-manager style
  (Ctrl+X/C/V, the Edit menu or a right-click): cut or copy a selection, then paste it into the
  folder being viewed or straight onto a right-clicked folder; cut rows dim until pasted and pasted
  rows appear instantly. Create, rename and delete folders: the plus beside the Folders heading makes
  one at the top level and New subfolder on a folder's right-click menu makes one inside it. Reorganise the
  tree by dragging a folder to nest it, move it out or reorder its siblings; the order you choose and
  the folders you keep collapsed are remembered per account and survive an update or reinstall.
  One folder each holds Inbox, Sent, Drafts, Trash, Junk and Archive, leading a collapsible tree with
  unread badges per folder, account and total. The archive is the one folder that never badges: archiving
  puts a message out of the way, so it stops asking for attention. On Gmail the archive is All Mail, which
  holds a copy of every message you have, so a count there would be the whole mailbox rather than anything
  about archived mail. Reading a message marks it read everywhere it appears, which on Gmail is every
  label it carries. On-arrival rules combine several conditions (all fields at
  once; or From, To, Cc, any recipient, Subject, sender domain) with all-or-any matching and a per-condition
  match-case switch, then mark read, flag, move to a folder of your choice or delete permanently. A rule
  runs on every account unless you limit it to the ones you pick. Delete permanently means exactly that: removed on the server,
  never cached and not recoverable. On most providers the message is expunged where it stands and never
  touches Trash. Gmail is the exception: it treats its folders as labels and answers an expunge by
  archiving rather than deleting, so there PigeonPost moves the message to the Bin and empties it from
  the Bin, which is the only route Gmail honours as a deletion. Rules run on the Inbox and
  only on mail arriving after the rule exists, so adding one never reaches back over the mail you have.
- **Read**: an optional reading pane (mark-on-view, F8 toggle) whose attachments stay pinned at the
  foot of the message, so Open and Save are one click away however long the thread is rather than below
  every quoted round, with draggable pane dividers
  (widths remembered, double-click to reset), a right-click context menu, in-app
  reader tabs, a double-click (or Enter) that pops the message out into its own dialog, mouse and
  keyboard multi-select with bulk actions, plus full keyboard control through an explicit focus ring;
  after ten seconds without input the app settles back on the active account's Inbox: the Inbox becomes
  the selected folder with keyboard focus on its row (never while a dialog is open, while you are
  mid-entry in a text field or while a message is open in the reader), so the window always resumes
  from a known place. The list stays fluid in folders of tens of thousands of messages. A Date sort and
  an optional threaded conversation view. In that view the header above a grouped conversation opens it
  whole: every message of the exchange gathered across the account's folders rather than the open one, so
  the reply you sent sits with the message it answers instead of staying out of sight in Sent. Each
  message names the folder it lives in, opens in place for reading and can be lifted into its own reader
  tab. Mail > Open conversation (Ctrl+Shift+T) does the same for the message you are on, so a thread is
  reachable without a mouse. An open message also lists its conversation under the header as a way
  through it. The setting
  governs the whole feature: switch it off and the list goes flat, the strip goes and any open thread
  closes.
  A `.eml` file opens in an in-app viewer; on Windows and macOS
  PigeonPost can be set as the default `.eml` handler. PigeonPost registers as a system mail handler on
  Windows, macOS and Linux, so it can be chosen as the default email client (Windows Default apps, the
  macOS default email reader, GNOME Default Apps) and a clicked mailto: link anywhere opens a pre-filled
  compose window. Print a message through the system print dialog.
- **Notifications**: new mail raises a native desktop notification and updates a Windows taskbar badge.
  Each IMAP account is watched by a persistent IDLE connection with a 60-second poll backstop (and for
  POP3); an account's first sync is silent. On Windows PigeonPost sounds its own chimes rather than the
  shell's default; each of the three things it announces has its own: new mail, a calendar reminder
  and a snoozed message coming back. They differ in how many notes sound and in their rhythm, so they
  are told apart by ear from each other and from every other app's notification; elsewhere the sound is
  the one your desktop theme chooses.
- **Calendar**: month, week and day views (a multi-day event is drawn as one bar across its days), recurring events with per-event time zones, nine
  emoji-labelled event categories, on-screen reminders at a lead you choose (from the moment the event
  starts out to a week before) and ICS import/export (RFC 5545) that round-trips
  with Outlook and Thunderbird. Every date field in the app (event times, repeat-until, a contact's
  birthday, send later, snooze) opens a themed calendar picker, with direct typing still first-class. Meeting
  invites over iTIP/iMIP (accept, decline, cancel, reply) with clickable join links (Teams, Meet, Zoom,
  Webex). Answering an invitation leaves a proper trail: the reply is saved to Sent, the invitation
  message gains the replied arrow and the invite card shows everyone's current response, updated as
  replies arrive on meetings you organise and as the organiser's updated invitations arrive on
  meetings you attend (queued through the outbox if you answer while offline). Clicking the
  answer you already gave warns before resending it. Re-saving a meeting you organise emails the
  attendees an update only when something they can
  see changed; a reminder or calendar tweak saves locally without emailing anyone and the save button
  says which it will be. Early two-way CalDAV sync: a calendar-server account (app password) syncs events both ways,
  server-wins on conflict with the losing local edit kept as a copy.
- **Contacts**: an address book with postal addresses and birthdays, plus vCard (.vcf) and CSV
  import/export that round-trips with Outlook and Thunderbird. People you email are added to the
  address book automatically (a minimal contact per new recipient, ready to flesh out or delete),
  with a toggle on the Contacts page to turn the collection off. CSV import reads both exporters'
  column conventions (including UK and US wording for regions and postcodes) and the encodings they
  actually write, so accented names survive. Importing the same export twice updates the contacts it
  matches instead of duplicating them; a match is merged so an import never overwrites what you
  have already recorded.
- **Trust**: a dark theme with a light toggle, passwords held in the OS keychain (never the database)
  and external links opened in your browser. The app checks GitHub's releases shortly after launch and
  once a day while running; only a formally published release can prompt and the request carries
  nothing about you or your mail. A newer release offers the download for your platform, with Skip
  This Version remembered and Later; Help > Check for Updates runs the same check on demand and also
  reports up to date or unreachable. An action that fails (a move the server refused, say) is
  reported in a banner under the toolbar with its own dismiss control, so a stale error never lingers. Closing the window while something is still open (a
  half-written message, say) surfaces the keep-in-tray-or-quit choice on top at once and warns that
  unsaved work may be lost, with Go back as the default so nothing is lost silently.

Planned: OS-delivered calendar alarms, two-way CardDAV contact sync. The
candidates parked beyond these are triaged with their rationale in
[FEATURES_PLAN.md](FEATURES_PLAN.md).

## Stack

| Concern | Choice |
|---|---|
| Shell | Wails v2 (Go + system WebView) |
| Backend | Go 1.25+ |
| Front end | React 18 + TypeScript (Vite) |
| Mail | emersion go-imap / go-smtp / go-message |
| Storage | modernc.org/sqlite (pure Go) + FTS5 |
| Credentials | OS keychain (zalando/go-keyring) |
| Delivery | Windows installer (a Wails app, same theme), macOS DMG (Apple Silicon), Linux Flatpak |

## Documentation

- [DEVELOPMENT-README.md](DEVELOPMENT-README.md): prerequisites, running, building and packaging.
- [ARCHITECTURE.md](ARCHITECTURE.md): the clean-architecture invariants and how they are enforced.
- [TESTING.md](TESTING.md): the test strategy, the coverage gate and how to run everything.
- [TECH_DEBT.md](TECH_DEBT.md): the standing technical-debt reference.
- [FEATURES_PLAN.md](FEATURES_PLAN.md): the triaged feature backlog (parked candidates and confirmed
  won't-dos, each with its rationale).

## Quick start

```
wails dev        # run the app in development
go test ./...    # run the Go test suite
cd frontend && npx vitest run   # run the front-end test suite
./build.ps1              # build the app exe and the installer (Windows)
bash builddmg.sh         # build the signed, notarized DMG (macOS, Apple Silicon)
bash build_flatpak.sh    # build and install the Flatpak (Linux)
```

## Supporting the project

A tray at the foot of the window carries a donate button at its left. It opens a PayPal payment page
in your browser; the app itself sends nothing and asks for nothing. Donations support maintenance and
continued development. Nothing in PigeonPost is withheld behind one: there is no paid tier, no licence
key and no feature that a donation unlocks.

## Licence

GPL-3.0. See [LICENSE](LICENSE); the full text is also in the app under Help > Licence.

Credit to the original author (Oliver Ernster) must be retained in all copies and derivative works,
under all circumstances. Removing or omitting this attribution is not permitted. The requirement is
stated in the LICENSE file's own licensing notice as a GPLv3 section 7(b) additional term and repeated
in Help > About.
