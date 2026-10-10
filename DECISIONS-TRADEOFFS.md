# Decisions and trade-offs

The deliberate choices PigeonPost rests on: what was chosen, what was given
up for it and why. Each entry is the decision as the product makes it today.
The detail behind each one, with the tests that hold it, lives in
[ARCHITECTURE.md](ARCHITECTURE.md) and [TESTING.md](TESTING.md);
[FEATURES_PLAN.md](FEATURES_PLAN.md) holds what is parked or ruled out and
[TECH_DEBT.md](TECH_DEBT.md) what was weighed and deliberately left alone.

## The product as a whole

### A client over the provider's own mail

PigeonPost reads and sends through the provider's IMAP, POP3 and SMTP
servers. Folders, message summaries and every body once opened are cached on
the machine, so mail already fetched reads offline. There is no PigeonPost
server, account or relay anywhere in between.

- **Rather than:** a hosted account or relay, including as the cure for
  scheduled sends that need the app running.
- **Gains:** nothing of the user's passes through a third party; IMAP mail
  stays where it was, so leaving PigeonPost leaves the mailbox untouched.
- **Costs:** send later, snooze and calendar reminders act only while the
  app is running or at its next launch.

### Go, Wails and React

The core is Go; the window is a Wails shell hosting a React and TypeScript
front end in the system's own web engine.

- **Rather than:** Rust and Tauri.
- **Gains:** one family of Go libraries covers IMAP, SMTP, MIME, iCalendar,
  vCard and WebDAV, so the whole mail, calendar and contacts surface shares
  one lineage.
- **Costs:** the interface runs in three different web engines. Some reader
  and drag behaviour exists only because WebKit differs from the engine on
  Windows.

### Each account keeps its own inbox

Accounts are stored separately. The unified mailbox is a view that merges the
cached inboxes as they are read, never a combination in storage.

- **Rather than:** one combined store.
- **Gains:** turning the view on or off changes nothing stored; paging stays
  exact across accounts.
- **Costs:** move, copy and junk are not offered in the combined view, since
  each folder belongs to one account.

### What PigeonPost deliberately is not

No AI summarising or reordering of mail, no priority inbox, no social or
engagement surfaces. Mail that lives only on disk (mbox, Thunderbird Local
Folders, Outlook data files) is not imported.

- **Rather than:** the extras of the larger suites.
- **Gains:** the list is what the server holds in the order it arrived,
  with nothing interpreted on the user's behalf.
- **Costs:** a local-only archive needs another tool; nothing condenses a
  long thread.

## Accounts and sign-in

### App passwords; no sign-in that needs a paid assessment

Gmail, iCloud, Yahoo, Zoho, Fastmail and StartMail come as presets, each
saying which password to use; all but Zoho say plainly that the normal
password will not work. One-click "Sign in with Google" is not offered.

- **Rather than:** Google's OAuth, whose full-mail scope carries a paid
  annual security assessment.
- **Gains:** no recurring cost attached to supporting a provider.
- **Costs:** Google Workspace accounts, which accept OAuth only, are not
  covered; every user of those presets has to create an app password.

### Microsoft through OAuth in the browser

A Microsoft account signs in through the system browser and the mail
connections then present a token. The tokens, the refresh token among them,
are kept in the keychain in place of a password.

- **Rather than:** a password, which Microsoft no longer accepts for these
  accounts; leaving Microsoft out.
- **Gains:** Outlook.com, Hotmail, Live and Microsoft 365 accounts work. The
  client identifier built into the app is a public one that holds no secret.
- **Costs:** Microsoft is the one OAuth provider and its flow is code of its
  own. A newly created Outlook.com mailbox can be refused IMAP or sending for
  days, which PigeonPost can only explain.

### Credentials proved before anything is written

Adding an account signs in to the incoming server first. Only a password
that works is put in the keychain and only then is the account saved. An
edit verifies the same way and never disturbs the stored password on a
failure.

- **Rather than:** saving the account and finding out at the first sync.
- **Gains:** a failed sign-in leaves the keychain and the store exactly as
  they were.
- **Costs:** none recorded.

### Passwords only in the operating system's keychain

The database never holds a password or token. On Windows, uninstalling with
the option to delete mail data purges the keychain entries as well.

- **Rather than:** credentials in the database beside the mail.
- **Gains:** the database file carries no secret; a full removal on Windows
  leaves none behind.
- **Costs:** removing the app on macOS or Linux leaves its keychain entries
  for the user to delete.

### POP3 through a small client of its own

POP3 is spoken by a hand-written client. A POP3 account downloads into one
local inbox, with read and star marks kept locally; folders, move, copy and a
Trash are not offered.

- **Rather than:** IMAP only; a third-party POP3 library.
- **Gains:** POP3-only mailboxes work, with no new dependency for a small
  protocol.
- **Costs:** the client is PigeonPost's to maintain; a POP3 delete removes
  the message from the server at once.

## Privacy and the network

### Every way out of the machine is named and held by a test

Beyond your own mail and calendar servers, the app reaches out in three ways
only: the update check, remote images you choose to load and Microsoft
sign-in. A structural test allows only the packages serving those to open a
connection and forbids the front end from making a request of its own.

- **Rather than:** a list kept by the documents alone, which a new outbound
  call would not have failed.
- **Gains:** a new way out of the machine cannot ship unnoticed; it fails the
  suite until it is named.
- **Costs:** the test reads source, so a request Wails or its web view makes
  on its own account is outside it.

### The message frame makes no request of its own

A message renders inside a sandboxed frame whose content policy grants no
script source and allows images and fonts only when they are carried in the
page itself. The sandbox does permit scripts, because WebKit will not deliver
the app's own link-click handling into a frame that forbids them; scripts in
the message are still stopped by the policy and by the sanitiser that runs
first.

- **Rather than:** rendering mail in the app's own page; a scriptless
  sandbox, which left every link in a message dead on macOS and Linux.
- **Gains:** opening a message cannot report that it was read; nothing a
  sender wrote runs.
- **Costs:** script denial rests on the policy and the sanitiser rather than
  on the sandbox flag.

### Remote images held back, then fetched by the app

Remote images and CSS backgrounds are parked until the reader asks. Loading
them goes through the app's own fetcher, which inlines each image into the
message. It connects only to public addresses, checked after the name is
resolved, accepts only images and bounds how far it follows and how much it
reads. Loading by default is a setting, off unless chosen.

- **Rather than:** letting the frame load images by address, which many
  senders' cross-origin rules block anyway.
- **Gains:** images that refuse to be embedded still show; a message cannot
  use the app to reach a machine on the local network.
- **Costs:** loading images is a request the app makes from the user's own
  address.

### Links leave through the browser

Every link opens in the default browser, never inside the app. Only http,
https and mailto addresses are handed over.

- **Rather than:** following links in the app's own window.
- **Gains:** a message cannot steer the app to a file or any other scheme.
- **Costs:** none recorded.

### An update check that names nobody

Shortly after launch and once a day while it runs, the app asks GitHub for
its latest published release. The request carries no identifier, account
detail or mail content. A tag pushed during development never prompts; a
version the user skips is offered again only when they check by hand.

- **Rather than:** no check; a check against tags.
- **Gains:** updates are found without nagging; nothing about the user
  leaves the machine.
- **Costs:** one unprompted request a day, from the user's address.

### No encryption at rest

The cache is an ordinary SQLite file, read through a pure-Go driver.
Encrypting it is parked.

- **Rather than:** SQLCipher or any of the mature encrypted SQLite options,
  all of which need a C toolchain in the build.
- **Gains:** the Windows build needs no C compiler.
- **Costs:** anyone who can read the account's files can read the cached
  mail and its search index; the operating system's disk encryption is the
  protection that applies.

### A record of the errors the interface hid

When the interface replaces a mail server's words with a sentence of its
own, the original is written to a log beside the database. An error passed
to the reader unchanged is not recorded, since its own detail reaches them.
Failures the interface deliberately does not show are recorded whatever
their wording: a sync, the new-mail check, a delivered message whose Sent
copy was lost and a snooze with no message left to return. The log stays
small, keeping one previous generation when it rolls over.

- **Rather than:** discarding the original; logging every error.
- **Gains:** a sentence that named the wrong cause can be shown to be wrong;
  a folder that stops refreshing leaves a reason behind; the log lists only
  the cases where something was hidden.
- **Costs:** it holds server responses and the addresses they concern in
  plain text.

### A run log beside the database

Each run writes its log lines and any crash report to a log file beside the
database, with a line at the start and end of every step a sync takes
against the server. No line names a password or token. The file starts
afresh once it grows past a set size.

- **Rather than:** the error output alone, which a program started from a
  shortcut on Windows does not have, so a sync stalled for weeks with
  nothing anywhere to say where.
- **Gains:** a stalled sync shows which folder and step it waits on; a crash
  leaves its report.
- **Costs:** another file of folder names and server errors in plain text.

### Donations go through the browser

The donate button hands a payment page to the browser through the same
route a link in a message takes. Nothing is withheld behind a donation.

- **Rather than:** a request of the app's own; a paid tier.
- **Gains:** no new connection inside the app; every feature is in every
  copy.
- **Costs:** none recorded.

## Mail and the local cache

### Bodies fetched on first open

A sync brings folders and summaries. A message's body, with its
attachments, is fetched the first time it is opened, then cached.

- **Rather than:** downloading every body at sync.
- **Gains:** a sync stays light however large the mailbox.
- **Costs:** full-text search reaches a body only once it has been opened;
  an unopened message reads offline by its summary alone.

### A folder synced before is refreshed from its listing

A folder already in the cache is refreshed from a listing of every message's
UID, flags and tags without headers. Only messages the cache does not hold
are fetched in full; the rest keep their cached headers and take the
listing's marks. The listing is trusted only while the server reports the
numbering the folder was settled under. A folder never settled, one the
server has renumbered and every POP3 folder are fetched in full.

- **Rather than:** fetching every summary at every sync, measured at up to
  40 seconds for 410 messages against Outlook.com, so an account of 54
  folders never finished a sync.
- **Gains:** a sync of a large mailbox finishes; the cost of a refresh
  follows what arrived rather than what is held.
- **Costs:** a folder's first sync is still a full download.

### One login per sync; one idle connection kept per account

A full sync of an account runs over one IMAP login. Between operations one
idle connection per account is kept for the next action, checked with a
NOOP before it is reused and logged out after five minutes unused. Two
operations at once each hold a connection; when both finish only one is
kept.

- **Rather than:** a login for each operation, the shape that made dozens of
  logins a minute and had StartMail block an address; a hard limit of one
  connection per account, under which an action would wait behind a long
  sync.
- **Gains:** a sync and a run of clicks cost a login or two rather than one
  each.
- **Costs:** a connection stays open for up to five minutes after use; a
  kept connection can have died, which costs one check before a fresh dial.

### Outgoing mail queues offline; other actions do not

A send or a draft save that finds the server unreachable waits in a
per-account Outbox (attachments included) and goes on the next sync. So does
one the server answers by saying it is temporarily unavailable: that is a
statement about the provider, not about the message or the account. A reply
asking for a password change is not read that way, since waiting would never
cure it. A send that can never succeed is kept and marked failed with its
reason. Delete and move stay online actions; a connection gives up within
seconds and says the app is offline in plain words.

- **Rather than:** queueing every action; waiting on the operating
  system's default timeout; failing a send the provider has said it cannot
  take right now.
- **Gains:** writing mail never depends on the network or on a provider's
  outage; a failed send never vanishes without trace.
- **Costs:** deleting and moving need a connection. A provider that keeps
  saying it is unavailable leaves the message waiting in the Outbox rather
  than reporting a failure.

### Read, star and tag changes held until the server agrees

A change is written to the cache together with a record that it is pending.
Every sync replays what is unconfirmed and lays it over what the server
reports, letting go of a change only when the server agrees. The replay
sends the pending changes in one batch per folder and reads them back on the
same connection; a change the read-back confirms is cleared there and then.
Tags travel as IMAP keywords, each fixed when the tag is made so a rename
never rewrites it. An added keyword (a tag or the forwarded mark) is cleared
only where the server says it keeps keywords; a server that holds one for the
session alone would read it back and clear the record guarding a local tag.

- **Rather than:** writing to the server and trusting the next fetch.
  Outlook.com accepts a flag change and then reports the old value, which
  kept turning read mail unread again. Replaying each change on a login of
  its own while clearing none until its folder was next fetched, which turned
  thousands of pending changes into a login every few seconds and had
  StartMail block the address.
- **Gains:** a message read stays read; marks work offline; tags reach other
  installations on the same account; a backlog of changes costs a few logins.
- **Costs:** more local state to keep correct. POP3 carries neither, so its
  marks stay on the one machine. On a server that does not keep keywords a
  tag is pushed again once per folder at every sync.

### Copies of one message kept in step

A server can show one message in several mailboxes; Gmail does it for every
label. A read, star or flag change reaches every cached copy in the account,
recognised by Message-ID, date, sender and subject together.

- **Rather than:** Gmail's own message identity, which the mail library
  cannot request without being forked; Message-ID alone, which many rows in
  real accounts share.
- **Gains:** reading a message clears it from bold everywhere it appears.
- **Costs:** two different messages agreeing on all four values would share
  their marks. It has not been observed.

### The archive never badges

No surface counts unread mail in the archive, neither its own folder nor the
account's totals.

- **Rather than:** counting it like any folder; deduplicating the totals by
  Message-ID, which would have undercounted every account.
- **Gains:** archiving puts a message out of the way. On Gmail, whose
  archive is All Mail, an arriving message is no longer counted twice.
- **Costs:** unread mail filed in the archive is never called for. No action
  marks a whole folder read, because on Gmail doing so to the archive would
  mark the whole mailbox read.

### Permanent deletion on Gmail goes through the Bin

Elsewhere a purge marks a message deleted and expunges it where it stands.
On Gmail, where an expunge archives rather than deletes, the message is
moved to the Bin and expunged from there.

- **Rather than:** one route for every provider, under which a rule set to
  destroy mail was found to have kept every message it reported destroying.
- **Gains:** delete permanently means it on Gmail too.
- **Costs:** the providers that behave this way are a list kept in the
  code; another one would need adding.

### One folder for each role; stray Sent folders merged

Inbox, Sent, Drafts, Trash, Junk and Archive each go to exactly one folder,
with the server's own markings taking precedence over names. Before each
account sync, stray sent folders have their mail moved into the one Sent and
are then deleted on the server.

- **Rather than:** treating every folder named Sent as Sent.
- **Gains:** a sent copy always lands in one known place.
- **Costs:** PigeonPost changes the server's folder tree. A stray is deleted
  only after its messages have moved.

### PigeonPost files its own copy of sent mail

After a send, a copy is appended to the account's Sent folder, except on
Gmail, which files one itself. The copy is best effort and never fails a
send.

- **Rather than:** relying on the server to file one, which most providers
  do not do for mail a client sends.
- **Gains:** sent mail is in the user's own account on every provider.
- **Costs:** the providers that save their own copy are a list in the code.

### Large selections and large folders

Deleting, moving or marking read a selection is grouped by folder. Each
folder is selected once and its messages sent in chunks, the folders taken
one after another over the account's kept connection, so a selection
spanning many folders normally costs one login. The list reads a folder a
page at a time and draws only the rows on screen.

- **Rather than:** a login per message, which ran past Gmail's limit on
  simultaneous connections; reading a folder whole, which froze the window
  on a real Trash of tens of thousands of messages.
- **Gains:** large selections complete; large folders stay responsive.
- **Costs:** select all has to load the rest of the folder first;
  conversation view and search still load their whole set.

### A search index that keeps its own text

Search is a full-text index over the subject, preview, sender, recipients,
the cached body and attachment names, with an operator grammar that never
refuses input: what it cannot parse is searched as plain text. Folder, flag,
date and account conditions stay in ordinary queries. Results are capped.

- **Rather than:** an index that reads its text from the source tables,
  which would need every delete to reproduce exactly what was indexed.
- **Gains:** every path that keeps the index right is a delete or reinsert
  of one message, a move included; flipping a flag needs no index work.
- **Costs:** the index holds its own copy of the text.

### Conversations by subject

Messages thread by subject with reply and forward prefixes stripped. The
same rule groups the list and gathers a conversation across the account's
folders, so the reply sent from Sent sits beside the message it answers.

- **Rather than:** threading on reply headers, which the cache does not
  hold.
- **Gains:** the list and the reader agree about what a thread is.
- **Costs:** unrelated messages with the same subject group together; a
  conversation lookup considers a bounded number of messages. The rule is
  written twice (once for the list, once for the reader); the two have to be
  kept alike.

### Folder order and collapse kept in the database

The order of custom folders and which are collapsed live in the database,
with the web engine's storage only a warm copy for the first paint. A
same-level reorder is local, since IMAP has no folder order; nesting a folder
is a real move on the server.

- **Rather than:** the web engine's local storage alone, which did not
  survive an update or a reinstall.
- **Gains:** the folder tree comes back as the user left it.
- **Costs:** none recorded.

### One unreadable message costs paperclips, not mail

The summaries a sync fetches come in one request: a whole folder the first
time, then only the messages new since. If one message's structure cannot be
decoded the request fails, so it is made again without structures: every
message arrives and the paperclip is lost for the messages in that request.

- **Rather than:** fetching in batches to confine the loss.
- **Gains:** one malformed message no longer empties a folder. Measured
  across every folder of several real accounts, the fallback fired on none.
- **Costs:** where it does fire, none of the messages it fetched shows an
  attachment, a whole folder's worth on a first sync. Later syncs keep those
  summaries as cached, so the paperclips do not come back by themselves.

### Server errors become sentences that claim no more than they know

A mail failure the interface shows is translated: offline, refused sign-in,
an app password asked for, a server saying it is temporarily unavailable, a
reply that cannot be read and a few others. A refused sign-in says it was
refused rather than that the password is wrong; an unavailable server is
reported as a problem at the provider, never as one with the password.

- **Rather than:** the server's own words; messages that assert a cause the
  failure does not carry.
- **Gains:** a failure reads as something a person can act on, at the point
  it happens.
- **Costs:** not every action routes through it yet: creating, renaming,
  moving and deleting folders and sending meeting invitations, replies and
  cancellations still show the raw error. A single read or star mark shows
  no server failure at all; it is left for the next sync to replay.

## Filter rules

### Rules decide; a separate step acts

Evaluating rules is a pure function that says what should happen to each
message. A separate step carries it out during the sync, before the messages
reach the cache, so a destroyed message never enters it.

- **Rather than:** rules that act as they are evaluated.
- **Gains:** every rule's verdict is testable with no server; a destroyed
  message leaves nothing to tidy up.
- **Costs:** none recorded.

### Unattended rules touch only new arrivals in the Inbox

During a sync, rules run on the Inbox only and only on messages the cache
has not seen. Moves and deletions wait until the folder's first pass has
been recorded as done.

- **Rather than:** inferring a first pass from an empty cache, which also
  matched an inbox the user keeps at zero and silently exempted all its mail
  from destructive rules.
- **Gains:** adding a rule or an account never reaches back over existing
  mail of its own accord.
- **Costs:** mail already filed is untouched until the rule is applied by
  hand. Mail another program moves into the Inbox comes back under a new UID
  and counts as new (TECH_DEBT.md item 2).

### Exclusions always count; a new rule narrows

Each condition can be negated, which gives every comparison its opposite. A
negated condition is an exclusion and is required whatever the match mode,
while the plain conditions combine as the mode says. A new rule starts on
"all of these" and the editor prints the joining word between rows.

- **Rather than:** treating an exclusion as one more alternative, which
  turned a rule for two senders into one that filed a whole mailbox.
- **Gains:** "any of these, never those" can be written and means what it
  says.
- **Costs:** the match mode is no longer the whole story; the editor has to
  show how rows combine.

### A rule naming no account covers every account

A rule can be limited to chosen accounts. One that names none applies to
every account, including any added later.

- **Rather than:** "unscoped" meaning the accounts that existed when the
  rule was written.
- **Gains:** a rule never quietly stops covering a new address.
- **Costs:** none recorded.

### A rule applied on demand: previewed, batched and stoppable

The Now button applies one rule across every folder of the accounts it
covers. It first counts what the rule would do and acts only once agreed.
Moves and deletions go in bounded batches. Cancel is read between batches,
never inside one; the report then states what had already happened.

- **Rather than:** cancelling inside a batch, which left a batch moved on
  the server while the cache still listed it where it had been.
- **Gains:** the cache and the server cannot drift apart mid-run; the
  confirmation quotes measured counts.
- **Costs:** a cancel waits for the batch in flight to finish.

### Rule sets travel by name

Export writes the rules as readable JSON with no local ids: a destination is
an account and a mailbox path. On import a rule replaces the stored one of
the same name. A rule arrives switched off and is named in the report when
this installation lacks its destination folder or every account it names.

- **Rather than:** rewriting or dropping what cannot act here; leaving it
  switched on, where it would silently do nothing on every sync.
- **Gains:** importing twice updates rather than duplicates; an imported
  rule works once its folder or account appears.
- **Costs:** two rules with one name cannot be told apart across machines.

## Compose and sending

### Send sends

Pressing Send delivers the message. There is no window in which it can be
pulled back; Send later remains for choosing a moment.

- **Rather than:** holding every message for a few seconds with an Undo.
- **Gains:** a message sent while the server is reachable has left when the
  compose window closes.
- **Costs:** a message sent in error cannot be recalled.

### Rich text with a plain-text part always

The composer is rich text throughout. Every message with formatting is sent
with a plain-text version first and the formatted one second.

- **Rather than:** a plain-text composing mode.
- **Gains:** one editor; plain-text clients still read every message.
- **Costs:** no way to write a message that is plain text only, apart from
  writing no formatting.

### Images embed, files attach

A pasted or dropped image is held in the message as its own bytes and sent
as an inline image part; any other file attaches. The message text is encoded
so no relay can fold a long line. Files, attached messages and embedded images
share one size limit.

- **Rather than:** linking images; sending long lines unencoded, which let
  relays fold a link in half.
- **Gains:** every mail client shows the image; long lines arrive intact.
- **Costs:** embedded images count against the size limit.

### A template keeps its files, not their paths

A template stores the bytes of the files it carries, held to the same limit
as one message.

- **Rather than:** remembering paths, which would leave a template looking
  complete while attaching nothing once a file moved.
- **Gains:** a template still does what it says long after it was made.
- **Costs:** the files are duplicated in the database.

### A reopened draft comes back with its files or not at all

A saved draft reopens in the compose window with its recipients, subject,
text and attached files, the files held exactly as a pasted file is held.
If its files cannot be fetched the draft does not open. Finishing it
replaces the stored copy.

- **Rather than:** opening the draft without its files, which sent or
  scheduled it with nothing attached and nothing on screen to say so.
- **Gains:** what leaves is what was saved.
- **Costs:** Bcc is not brought back, since a saved draft carries none; a
  draft not yet opened on this machine needs the server to reopen.

### A local recovery slot, apart from server drafts

While a message is written it is saved now and then to one local slot,
offered back after a crash and cleared by a send, a save or a confirmed
discard. Closing an edited message asks first.

- **Rather than:** relying on server drafts alone.
- **Gains:** a crash or a stray click does not lose a message.
- **Costs:** one slot, so only the most recent unsent message is kept.

### People emailed join the address book

When a message is sent or scheduled to send later, each recipient not
already in the address book is added as a minimal contact. A setting on the Contacts page turns it off; it starts on.

- **Rather than:** an address book filled only by hand.
- **Gains:** suggestions in the address fields grow from use.
- **Costs:** the address book gathers contacts the user may not want.

## Calendar and contacts

### What the app does not model is kept, not dropped

An imported event keeps the iCalendar properties PigeonPost does not model
and they are written back on export. To-dos, journal entries and alarms of
kinds the app does not handle are kept verbatim.

- **Rather than:** modelling only what is shown and losing the rest.
- **Gains:** an export from PigeonPost carries what the import brought.
- **Costs:** data is held that the app neither shows nor edits.

### Each event carries its own time zone

An event stores a named zone, so a recurring event keeps its local time
across daylight saving. The zone database is built in. Windows zone names in
Outlook files are translated on import; export writes zone definitions for
the zones it uses.

- **Rather than:** fixed offsets; the system's own zone database.
- **Gains:** a 9am meeting stays at 9am; Outlook events are not silently
  dropped on import.
- **Costs:** the zone definitions written on export are derived by probing
  each zone, which is code of PigeonPost's own.

### Reminders fire while the app runs

Reminders are checked while the app is running. At launch, one missed while
the app was closed fires only if its event has not yet started.

- **Rather than:** reminders delivered by the operating system.
- **Gains:** no backlog of stale alerts on opening the app.
- **Costs:** with the app closed, no reminder fires.

### Only the organiser emails a meeting's attendees

An account sends invitations and cancellations only for meetings it
organises; an attendee's copy saves locally. Re-saving a meeting emails an
update only when something attendees can see has changed. An organiser's
update folds in other attendees' responses, never the meeting's content.

- **Rather than:** any copy of a meeting being able to email everyone on it.
- **Gains:** no attendee can invite the others to someone else's meeting
  from the wrong address; a reminder tweak sends nobody mail.
- **Costs:** another attendee's reply reads "Not known" until the organiser
  sends an update.

### Calendar sync: the server wins, the local edit is kept

A CalDAV account syncs both ways. A local edit is pushed only if the server's
copy is unchanged; where the server changed meanwhile, the server's version
is kept and the local one saved beside it as a copy.

- **Rather than:** the local version winning; a one-way pull.
- **Gains:** a conflicting edit is never silently lost.
- **Costs:** the conditional writes are PigeonPost's own code, since the
  WebDAV library cannot send them; the sync has not been exercised against a
  live server, so how real providers behave is unproven.

### A contact import merges

A re-import matches each contact by id, then a shared email address, then
the display name for one with no email. A match is merged into what is
stored. Outlook and Thunderbird CSV files are read in the encodings those
programs actually write.

- **Rather than:** overwriting or duplicating on every import.
- **Gains:** importing the same export twice changes nothing; details added
  by hand survive.
- **Costs:** two different people with no email and the same name merge.

### Two contact formats

Contacts import and export as vCard and as CSV.

- **Rather than:** vCard alone.
- **Gains:** vCard round-trips with Thunderbird; CSV covers Outlook's bulk
  export, which does not write vCard.
- **Costs:** two codecs to keep correct.

## The interface

### Drawn artwork rather than emoji

Every control on the title bar and every folder mark is a picture the
repository owns, generated from masters.

- **Rather than:** emoji, drawn by whatever font each platform ships.
- **Gains:** the same marks at the same weight on Windows, macOS and Linux.
- **Costs:** every new control needs a drawing.

### One run of controls in a window wide enough for it

The menus and working controls run from the left of the title bar; the theme
toggle and Help sit at the far end. The window cannot be made narrower than
the width at which all of them fit.

- **Rather than:** a smaller window, at which the bar overran; centring the
  controls or gathering them against the right edge, each tried and dropped.
- **Gains:** the controls stay together and in reach at any width the app
  allows.
- **Costs:** the window has a floor on its width.

### Headers and actions stay put; the content scrolls

The reader keeps its tabs, sender and subject above the body and its
attachments below it. Every dialog with buttons keeps them pinned, with the
title and leading fields held above a scrolling body.

- **Rather than:** dialogs and messages that scroll as one block.
- **Gains:** Save and Open stay in reach on a short window and a long
  thread.
- **Costs:** none recorded.

### Stacked editors ignore a click beside them

An editor opened over a list (a contact, a group, a template, a remote
calendar or an event) closes on Escape but not on a click on its backdrop.

- **Rather than:** dismissing on any click outside.
- **Gains:** a stray click cannot drop what was typed.
- **Costs:** one way out fewer.

### The keyboard reaches everything

A focus ring takes Tab and the arrow keys through every control; menus carry
accelerators wired from the same definitions they draw.

- **Rather than:** mouse-first controls.
- **Gains:** the app works without a mouse; a shortcut hint cannot drift
  from its key.
- **Costs:** every new control needs its place in the ring.

### Closing asks, on Windows; one copy runs

On Windows the close button offers minimise to tray or quit in a dialog of
the app's own. Elsewhere it quits. A second launch brings the running copy
forward, without changing whether it is maximised.

- **Rather than:** a native dialog with Yes and No; closing always
  quitting.
- **Gains:** sync and reminders can carry on with the window hidden; the
  question is worded for what it asks.
- **Costs:** one more question at every close.

### A drop answers at once

A message dragged onto a folder leaves the list on the drop and the folder
flashes. A refused or partial move puts the rows back beside the
neighbours they left.

- **Rather than:** waiting for the server, which made a slow drop read as a
  miss.
- **Gains:** a drag that worked looks like it worked.
- **Costs:** the list can briefly show a move the server then refuses.

### Motion only where it helps, whatever the animation setting

The guide, About and Licence scroll gently on their own. They pause the
moment the reader takes over and carry on from there once left alone; no
other surface moves by itself. Neither that nor the
drop flash follows the reduced-motion setting, which on Windows tracks the
general animation switch people turn off for speed.

- **Rather than:** static help; self-reading everywhere; honouring the
  setting.
- **Gains:** long help reads hands free without any work surface fighting
  the user; neither feature silently disappears on a machine with animations
  off.
- **Costs:** someone who wants no motion can only pause a pane, which moves
  again once left alone.

### Idle returns to the Inbox

After a short spell without input the active account's Inbox is selected
again, never while a dialog or a menu is open, a field is being typed in or a
message is open.

- **Rather than:** leaving the window wherever it was last left.
- **Gains:** the window always resumes from a known place.
- **Costs:** a folder left open unattended does not stay open.

### Chimes of its own

On Windows the shell's notification sound is silenced and PigeonPost plays
one of three chimes synthesised in code: new mail, a reminder and a returning
snoozed message differ by note count and rhythm. Elsewhere the desktop
chooses.

- **Rather than:** the shell's one sound for everything; recorded sound
  files.
- **Gains:** an alert can be told apart by ear; no audio file to ship.
- **Costs:** none recorded.

## Building and installing

### A setup program of its own, for one user

On Windows a bespoke setup program installs, updates, repairs and removes
the app under the user's own folders and registry. If the app is running it
offers to close it, which it does by ending the process.

- **Rather than:** a generic installer; a machine-wide install.
- **Gains:** no administrator prompt; the setup program wears the app's own
  look.
- **Costs:** the setup program is PigeonPost's to maintain; each account on
  a machine installs separately.

### Each platform built on itself

Windows builds an executable and setup program, macOS a signed DMG for Apple
Silicon and Linux a Flatpak. The macOS build always notarises unless told
explicitly to skip it for a local test build, which is never released.

- **Rather than:** cross-compiling; shipping an unnotarised Mac build.
- **Gains:** each package is built by the tools of its platform; a Mac
  download opens on any Mac.
- **Costs:** a machine of each kind and an Apple developer account.

### What the build depends on is declared once in the repository

The version lives in one file read by the app and stamped into each package.
The icons and glyphs are generated from masters; every release build runs
the generator and its output is committed. Line endings are declared by the
repository rather than left to each machine.

- **Rather than:** versions written where needed; generated artwork kept
  out of the repository; each machine's own line-ending setting, which had
  git reporting files modified with nothing changed.
- **Gains:** nothing drifts; a plain build of a fresh clone needs no
  generator run; a real change cannot hide among phantom ones.
- **Costs:** a changed master and its regenerated output have to be
  committed together; a checkout made before the line-ending rule may hold
  old endings until refreshed.

### GPL-3.0 with an attribution term

The app is GPL-3.0 with an additional term, as section 7(b) permits,
requiring the author's credit to be kept. A commercial licence for the
author's own code is offered separately.

- **Rather than:** a permissive licence; no commercial route.
- **Gains:** derivatives stay open and credited; closed-source users have a
  way in.
- **Costs:** two licensing routes to explain.

## Engineering

### Layers held by tests

The code is split into domain, application, infrastructure and interface,
each depending only inward, wired in one composition root. Structural tests
fail the Go suite on a forbidden import, a domain that reads the clock or the
network, a second place that wires both layers and a source module over the
size limit. The front end's tests hold its pure modules to their imports and
every module to the same size limit. Its one composition file is exempt from
that limit by decision and cannot grow.

- **Rather than:** convention alone; a dependency injection framework;
  splitting that composition file for the count's sake.
- **Gains:** the rules about mail, rules and calendars are tested with no
  server, disk or clock; modules split at real seams.
- **Costs:** more packages, more small files and explicit wiring; one
  exemption to keep honest.

### Complete coverage where the logic lives

The domain and application layers are held at full coverage by the test
script, which also checks formatting and vets first. The Go code has no
mocking library; its fakes are written by hand. The front end gates a named
list of its pure modules the same way, each added as it is extracted with
its test.

- **Rather than:** one figure over everything, which would mean mocking the
  operating system.
- **Gains:** anything short of complete in the core is a gap somebody has to
  explain.
- **Costs:** network, Win32 and window code relies on targeted tests; a
  bare test run applies no gate.

### A test is trusted once it has been seen to fail

A new guard is proved by planting the violation it exists for. A migration
that rewrites data is tested on a database from the version it upgrades. The
front end's stand-in for the backend is built from the real one, so a call
nobody stubbed fails by name. A large refactor is pinned first by tests
written against the code as it stood.

- **Rather than:** guards assumed to work; tests that pass vacuously or for
  the wrong reason; refactoring checked by eye.
- **Gains:** each check is known to bite; a behaviour-preserving move is
  shown to change nothing.
- **Costs:** a deliberate failure for every guard; every call a test reaches
  has to be stubbed.
