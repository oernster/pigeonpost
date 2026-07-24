// api centralises all access to the Wails-bound Go facade so React components depend on this thin
// module rather than the generated bindings directly.
import {
    About,
    AddAccount,
    AddCalDAVAccount,
    ListCalDAVAccounts,
    SyncCalDAV,
    RemoveCalDAVAccount,
    Author,
    DeleteMessage,
    DeleteMessagePermanent,
    DeleteMessages,
    DeleteMessagesPermanent,
    DeleteTag,
    GetMessageBody,
    ListAccounts,
    ListFolders,
    LicenceText,
    ListMessages,
    ListMessagesPage,
    ListSnoozedMessages,
    ListUnifiedMessages,
    ListUnifiedMessagesPage,
    ListTags,
    LoadRemoteImages,
    MarkFlagged,
    MarkForwarded,
    MarkJunk,
    MarkNotJunk,
    MarkRead,
    MarkReplied,
    MessageTags,
    MinimiseToTray,
    RequestQuit,
    CopyMessage,
    CreateFolder,
    DeleteCalendar,
    DeleteContact,
    DeleteContactGroup,
    DeleteEvent,
    DeleteFolder,
    DeleteRule,
    DeleteTemplate,
    ExportContactsToFile,
    ExportEventsToFile,
    GetContact,
    GetEvent,
    GetInvitation,
    RespondToInvitation,
    RemoveCancelledMeeting,
    ApplyMeetingReply,
    SendMeetingRequest,
    SendMeetingCancel,
    ImportContactsFromFile,
    ImportEventsFromFile,
    ListCalendars,
    ListContactGroups,
    ListContacts,
    ListEvents,
    ListEventInstances,
    ListRules,
    ListTemplates,
    SaveCalendar,
    SaveContact,
    SaveContactGroup,
    SaveEvent,
    SaveEventScoped,
    DeleteEventScoped,
    MoveFolder,
    MoveMessage,
    MoveMessages,
    RenameFolder,
    SaveRule,
    SaveTemplate,
    CancelOutboxItem,
    ListOutbox,
    OpenExternal,
    OpenReleasesPage,
    ClearDraftRecovery,
    DraftRecovery,
    OutboxCount,
    PickAttachments,
    RemoveAccount,
    ReorderAccounts,
    ReplayOutbox,
    SignInMicrosoft,
    SnoozeMessage,
    SnoozedCount,
    UnsnoozeMessage,
    SaveDraft,
    SaveDraftRecovery,
    OpenAttachment,
    OpenEmailAttachment,
    SaveAllAttachments,
    SaveAttachment,
    SaveMessageAs,
    SaveTag,
    SearchMessages,
    SendMessage,
    SetMessageTag,
    ShowDefaultAppSettings,
    ShowDefaultMailAppSettings,
    SyncAccount,
    SyncAllInboxes,
    SyncFolder,
    UnreadCounts,
    UpdateAccount,
    UpdateAccountProfile,
    Version,
} from '../wailsjs/go/main/App'
import {ClipboardGetText} from '../wailsjs/runtime'
import {main} from '../wailsjs/go/models'
import {isUnifiedFolder} from './unified'
import {isSnoozedFolder} from './snooze'

export type Account = main.AccountDTO
export type Folder = main.FolderDTO
// Wails returns plain JSON at runtime (no class methods), and the UI spreads messages for optimistic
// updates, so Message is the data-only shape. Omitting convertValues keeps this valid even after the
// generated MessageDTO regains that helper method on a `wails generate`.
export type Message = Omit<main.MessageDTO, 'convertValues'>
// MessagePage is one keyset-paginated slice of a folder's flat listing: the page's messages, whether an
// older (or newer, when ascending) page exists plus the opaque cursor to fetch it. The cursor is passed
// straight back to listMessagesPage; it is never constructed by the caller.
export interface MessagePage {
    messages: Message[]
    hasMore: boolean
    nextCursorDateMs: number
    nextCursorId: string
}
// MESSAGE_PAGE_SIZE is how many rows the flat folder view loads per page. A folder of tens of thousands
// of messages (a real Trash) would freeze the render if every row were loaded at once, so the list loads
// one page and fetches the next as the user scrolls.
export const MESSAGE_PAGE_SIZE = 200
// SearchHit is one search result: the matched message plus a snippet of the matched text with each
// matched term wrapped between SEARCH_MATCH_START and SEARCH_MATCH_END. The snippet is empty for a
// purely structural query (one with no search text, such as "is:unread").
export interface SearchHit {
    message: Message
    snippet: string
}
// SearchResult carries one search's hits, most relevant first, plus whether the query text failed
// structural parsing and was searched as plain text (so the UI can hint that operators were ignored).
export interface SearchResult {
    hits: SearchHit[]
    degraded: boolean
}
// The control characters the backend wraps matched terms in. The UI splits on them to render
// highlights, so message content is never interpreted as markup.
export const SEARCH_MATCH_START = '\u0001'
export const SEARCH_MATCH_END = '\u0002'
export type AboutInfo = main.AboutDTO
export type Tag = main.TagDTO
export type Rule = main.RuleDTO
export type Template = main.TemplateDTO
// MessageBody drops the generated convertValues helper so an outbox message's body can be built as a
// plain object literal; the nested AttachmentDTO array carries no helper of its own.
export type MessageBody = Omit<main.MessageBodyDTO, 'convertValues'>
export type Attachment = main.AttachmentDTO
export type OutboxItem = main.OutboxItemDTO
export type UnreadCountsResult = main.UnreadCountsDTO
export type BulkResult = main.BulkResultDTO
// MoveResult reports where a move-shaped action (move, delete to Trash, junk, rescue) put the
// message: the id it will carry in its destination folder, or empty when the server did not say.
// Undo entries are built from it.
export type MoveResult = main.MoveResultDTO
export type Contact = main.ContactDTO
export type ContactGroup = main.ContactGroupDTO
// ContactImportResult separates records stored as new contacts from those merged into existing ones,
// so the UI can say what an import actually changed rather than implying every row was new.
export type ContactImportResult = main.ContactImportResult

export interface ContactEmailInput {
    label: string
    address: string
}

export interface ContactPhoneInput {
    label: string
    number: string
}

export interface ContactAddressInput {
    label: string
    street: string
    locality: string
    region: string
    postalCode: string
    country: string
}

export interface ContactInput {
    id: string
    uid: string
    formattedName: string
    givenName: string
    familyName: string
    organization: string
    title: string
    note: string
    birthday: string
    emails: ContactEmailInput[]
    phones: ContactPhoneInput[]
    addresses: ContactAddressInput[]
}

export interface ContactGroupInput {
    id: string
    name: string
    members: string[]
}

export type Calendar = main.CalendarDTO
// CalDAVAccount is a configured remote CalDAV account. The password is never part of this view; it lives
// in the OS keychain, exactly as for a mail account.
export type CalDAVAccount = main.CalDAVAccountDTO
export type CalendarEvent = main.EventDTO
export type CalendarEventInstance = main.EventInstanceDTO
export type Invitation = main.InvitationDTO
export type MeetingAttendee = main.AttendeeDTO

// PartStat is the ICS PARTSTAT reply value the reader sends when answering a meeting request.
export type PartStat = 'ACCEPTED' | 'DECLINED' | 'TENTATIVE'

// EventScope mirrors the Go application.EventScope: how far an edit or delete of a recurring occurrence
// reaches. The integer values must match the Go constants.
export enum EventScope {
    This = 0,
    Future = 1,
    All = 2,
}

export interface CalendarInput {
    id: string
    name: string
    colour: string
}

// MeetingOrganizerInput is the organizer written onto an event when it is a meeting. An empty address
// marks an ordinary (non-meeting) event.
export interface MeetingOrganizerInput {
    address: string
    commonName: string
}

// MeetingAttendeeInput is one invited party written onto a meeting event. role and status accept the ICS
// ROLE and PARTSTAT values; empty strings take the domain defaults (REQ-PARTICIPANT and NEEDS-ACTION).
export interface MeetingAttendeeInput {
    address: string
    commonName: string
    role: string
    status: string
    rsvp: boolean
}

export interface CalendarEventInput {
    id: string
    uid: string
    calendarId: string
    summary: string
    description: string
    location: string
    // category is the optional short lowercase category value (the primary iCalendar CATEGORIES value);
    // empty means no category.
    category: string
    start: string
    end: string
    allDay: boolean
    recurrence: string
    timeZone: string
    // reminders are lead times in whole minutes before the event start (0 means at the start).
    reminders: number[]
    extra: string
    // organizer and attendees carry the meeting scheduling data. organizer.address is empty and attendees
    // is empty for an ordinary calendar entry.
    organizer: MeetingOrganizerInput
    attendees: MeetingAttendeeInput[]
}

export interface RuleInput {
    id: string
    name: string
    field: string
    operator: string
    contains: string
    action: string
}

export interface TagInput {
    id: string
    name: string
    colour: string
}

export interface TemplateInput {
    id: string
    name: string
    subject: string
    body: string
}

export interface ComposeInput {
    accountId: string
    // from is the chosen sender address: empty means the account's primary address, otherwise it must be
    // one of the account's identities. The backend validates it.
    from: string
    to: string[]
    cc: string[]
    bcc: string[]
    subject: string
    body: string
    htmlBody: string
    attachmentPaths: string[]
    // attachmentData carries files pasted or dropped into the compose window, where the webview holds
    // name and bytes but no filesystem path. content is base64 (AttachmentDataEntry in send.go).
    attachmentData: {name: string; contentType: string; content: string}[]
    attachmentMessageIds: string[]
    // holdSeconds is the undo-send window: greater than zero queues the send for that long (send returns
    // the queued item's id, cancellable until it elapses) and zero sends immediately (send returns '').
    holdSeconds: number
    // sendAtMs is send-later: a Unix-millisecond instant queues the send held until then (cancellable
    // from the Outbox; send returns the queued id) and takes precedence over holdSeconds. Zero means no
    // schedule.
    sendAtMs: number
}

// DraftRecoveryInput is a local snapshot of the compose window, autosaved for crash and
// accidental-close recovery. The recipient fields are the raw text as typed, not parsed lists.
export interface DraftRecoveryInput {
    accountId: string
    to: string
    cc: string
    bcc: string
    subject: string
    bodyHtml: string
}

// DraftRecoveryResult is the stored compose snapshot. present is false when none is held, in which case
// the other fields are empty and there is nothing to restore.
export interface DraftRecoveryResult {
    present: boolean
    accountId: string
    to: string
    cc: string
    bcc: string
    subject: string
    bodyHtml: string
    savedMs: number
}

// Identity is one alternate sender address on an account: an email address with an optional display name.
export interface Identity {
    name: string
    address: string
}

export interface AccountSetupInput {
    displayName: string
    email: string
    password: string
    protocol: string
    inHost: string
    inPort: number
    inSecurity: string
    outHost: string
    outPort: number
    outSecurity: string
    signature: string
    identities: Identity[]
}

// AccountProfileInput edits only an account's profile (display name, signature and send-as identities),
// leaving its servers and credentials untouched. It is the payload for an OAuth account's edit and for
// any edit that changes no server setting. email identifies the account and is not editable.
export interface AccountProfileInput {
    email: string
    displayName: string
    signature: string
    identities: Identity[]
}

// EmailView is a parsed .eml attachment shown in the in-app viewer: its key headers and its sanitised HTML
// and plain-text bodies.
export interface EmailView {
    subject: string
    from: string
    to: string
    date: string
    html: string
    plain: string
}

export const api = {
    listAccounts: (): Promise<Account[]> => ListAccounts(),
    addAccount: (req: AccountSetupInput): Promise<void> => AddAccount(main.AccountSetupRequest.createFrom(req)),
    removeAccount: (accountId: string): Promise<void> => RemoveAccount(accountId),
    // reorderAccounts persists the sidebar order of accounts as the given full list of ids, most
    // preferred first, after a drag or an up/down move.
    reorderAccounts: (orderedIds: string[]): Promise<void> => ReorderAccounts(orderedIds),
    updateAccount: (req: AccountSetupInput): Promise<void> => UpdateAccount(main.AccountSetupRequest.createFrom(req)),
    // updateAccountProfile changes only the name, signature and send-as identities of an existing account,
    // without re-verifying its credentials. It is the edit path for an OAuth account.
    updateAccountProfile: (req: AccountProfileInput): Promise<void> =>
        UpdateAccountProfile(main.AccountProfileRequest.createFrom(req)),
    // signInMicrosoft runs the interactive OAuth flow (opens the browser, waits for consent) and resolves
    // with the signed-in address so the caller can select the new account.
    signInMicrosoft: (displayName: string): Promise<string> => SignInMicrosoft(displayName),
    listTags: (): Promise<Tag[]> => ListTags(),
    saveTag: (req: TagInput): Promise<void> => SaveTag(main.TagRequest.createFrom(req)),
    deleteTag: (tagId: string): Promise<void> => DeleteTag(tagId),
    messageTags: (messageId: string): Promise<Tag[]> => MessageTags(messageId),
    setMessageTag: (messageId: string, tagId: string, assigned: boolean): Promise<void> =>
        SetMessageTag(messageId, tagId, assigned),
    listFolders: (accountId: string): Promise<Folder[]> => ListFolders(accountId),
    unreadCounts: (): Promise<UnreadCountsResult> => UnreadCounts(),
    // listMessages routes the synthetic folders: the unified id to the merged cross-account inbox
    // listing and the snoozed id to the hidden-message listing, so the folder-driven callers (the
    // conversation view's whole-set load) work on them unchanged.
    listMessages: (folderId: string): Promise<Message[]> =>
        isUnifiedFolder(folderId) ? ListUnifiedMessages()
            : isSnoozedFolder(folderId) ? ListSnoozedMessages()
                : ListMessages(folderId),
    // listMessagesPage fetches one keyset page of a folder's flat listing. The first call passes
    // hasCursor false (the cursor arguments are ignored); each later call passes hasCursor true with the
    // previous page's nextCursorDateMs and nextCursorId to walk to strictly older (or newer, when
    // ascending) rows. ascending matches the list's sort direction. The synthetic unified folder routes
    // to the merged cross-account page with the identical cursor mechanics; the snoozed view is small,
    // so it arrives whole as a single page.
    listMessagesPage: async (
        folderId: string, hasCursor: boolean, cursorDateMs: number, cursorId: string, limit: number, ascending: boolean,
    ): Promise<MessagePage> => {
        if (isUnifiedFolder(folderId)) {
            return ListUnifiedMessagesPage(hasCursor, cursorDateMs, cursorId, limit, ascending)
        }
        if (isSnoozedFolder(folderId)) {
            const messages = await ListSnoozedMessages()
            return {messages, hasMore: false, nextCursorDateMs: 0, nextCursorId: ''}
        }
        return ListMessagesPage(folderId, hasCursor, cursorDateMs, cursorId, limit, ascending)
    },
    // searchMessages runs the operator-grammar search. folderId and accountId scope it to the UI's
    // selection (empty strings for all mail).
    searchMessages: (query: string, folderId: string, accountId: string): Promise<SearchResult> =>
        SearchMessages(query, folderId, accountId),
    messageBody: (messageId: string): Promise<MessageBody> => GetMessageBody(messageId),
    // loadRemoteImages returns the message HTML with its blocked remote images fetched server-side and inlined
    // as data: URIs, so the reader can show images a browser cannot load cross-origin (a sender's
    // Cross-Origin-Resource-Policy, CORS or hotlink protection).
    loadRemoteImages: (html: string): Promise<string> => LoadRemoteImages(html),
    openExternal: (url: string): Promise<void> => OpenExternal(url),
    syncAccount: (accountId: string): Promise<void> => SyncAccount(accountId),
    // syncFolder routes the synthetic folders: the unified id refreshes every inbox (so opening it and
    // the background poll refresh what the combined list actually shows) and the snoozed id does
    // nothing (snooze is local state with no server side to sync).
    syncFolder: (folderId: string): Promise<void> =>
        isUnifiedFolder(folderId) ? SyncAllInboxes()
            : isSnoozedFolder(folderId) ? Promise.resolve()
                : SyncFolder(folderId),
    // snoozeMessage hides a message until the given instant (Unix milliseconds, must be in the future);
    // unsnoozeMessage brings it back at once; snoozedCount sizes the sidebar entry's badge.
    snoozeMessage: (messageId: string, untilMs: number): Promise<void> => SnoozeMessage(messageId, untilMs),
    unsnoozeMessage: (messageId: string): Promise<void> => UnsnoozeMessage(messageId),
    snoozedCount: (): Promise<number> => SnoozedCount(),
    markRead: (messageId: string, read: boolean): Promise<void> => MarkRead(messageId, read),
    markFlagged: (messageId: string, flagged: boolean): Promise<void> => MarkFlagged(messageId, flagged),
    // markReplied / markForwarded record that a message has been replied to (\Answered) or forwarded
    // ($Forwarded) on the server and in the local cache, so its row shows the indicator. The composer calls
    // them after a successful reply / forward.
    markReplied: (messageId: string): Promise<void> => MarkReplied(messageId),
    markForwarded: (messageId: string): Promise<void> => MarkForwarded(messageId),
    deleteMessage: (messageId: string): Promise<MoveResult> => DeleteMessage(messageId),
    deleteMessagePermanent: (messageId: string): Promise<void> => DeleteMessagePermanent(messageId),
    // deleteMessages / deleteMessagesPermanent / moveMessages act on the whole selection in one batched
    // backend call (grouped by folder, one server connection per folder) rather than a round trip per
    // message, which is what keeps a large Gmail selection under its simultaneous-connection cap.
    deleteMessages: (ids: string[]): Promise<BulkResult> => DeleteMessages(ids),
    deleteMessagesPermanent: (ids: string[]): Promise<BulkResult> => DeleteMessagesPermanent(ids),
    moveMessages: (ids: string[], destFolderId: string): Promise<BulkResult> => MoveMessages(ids, destFolderId),
    saveMessageAs: (messageId: string, suggestedName: string): Promise<void> =>
        SaveMessageAs(messageId, suggestedName),
    saveAttachment: (messageId: string, index: number): Promise<void> => SaveAttachment(messageId, index),
    openAttachment: (messageId: string, index: number): Promise<void> => OpenAttachment(messageId, index),
    openEmailAttachment: (messageId: string, index: number): Promise<EmailView> => OpenEmailAttachment(messageId, index),
    // showDefaultAppSettings opens Windows' Default apps settings so the user can make PigeonPost the default
    // for .eml files (Windows does not let an app claim the default silently).
    showDefaultAppSettings: (): Promise<void> => ShowDefaultAppSettings(),
    // showDefaultMailAppSettings re-registers the mailto: handler then opens the same settings page, so
    // the user can make PigeonPost the default email client (the MAILTO link type).
    showDefaultMailAppSettings: (): Promise<void> => ShowDefaultMailAppSettings(),
    saveAllAttachments: (messageId: string): Promise<void> => SaveAllAttachments(messageId),
    moveMessage: (messageId: string, destFolderId: string): Promise<MoveResult> => MoveMessage(messageId, destFolderId),
    markJunk: (messageId: string): Promise<MoveResult> => MarkJunk(messageId),
    markNotJunk: (messageId: string): Promise<MoveResult> => MarkNotJunk(messageId),
    // copyMessage duplicates a message into destFolderId; the result carries the id the duplicate
    // holds there when the server reported it (COPYUID), so a pasted copy can show up instantly.
    copyMessage: (messageId: string, destFolderId: string): Promise<MoveResult> => CopyMessage(messageId, destFolderId),
    createFolder: (accountId: string, name: string): Promise<void> => CreateFolder(accountId, name),
    renameFolder: (folderId: string, newName: string): Promise<void> => RenameFolder(folderId, newName),
    deleteFolder: (folderId: string): Promise<void> => DeleteFolder(folderId),
    // moveFolder reparents a folder under newParentId on the server (an empty newParentId moves it to
    // the top level). Same-level reordering is a local display concern and does not call this.
    moveFolder: (folderId: string, newParentId: string): Promise<void> => MoveFolder(folderId, newParentId),
    listRules: (): Promise<Rule[]> => ListRules(),
    saveRule: (req: RuleInput): Promise<void> => SaveRule(main.RuleRequest.createFrom(req)),
    deleteRule: (ruleId: string): Promise<void> => DeleteRule(ruleId),
    listTemplates: (): Promise<Template[]> => ListTemplates(),
    saveTemplate: (req: TemplateInput): Promise<void> => SaveTemplate(main.TemplateRequest.createFrom(req)),
    deleteTemplate: (templateId: string): Promise<void> => DeleteTemplate(templateId),
    about: (): Promise<AboutInfo> => About(),
    licence: (): Promise<string> => LicenceText(),
    version: (): Promise<string> => Version(),
    author: (): Promise<string> => Author(),
    openReleases: (): Promise<void> => OpenReleasesPage(),
    minimiseToTray: (): Promise<void> => MinimiseToTray(),
    requestQuit: (): Promise<void> => RequestQuit(),
    send: (req: ComposeInput): Promise<string> => SendMessage(main.ComposeRequest.createFrom(req)),
    saveDraft: (req: ComposeInput): Promise<void> => SaveDraft(main.ComposeRequest.createFrom(req)),
    saveDraftRecovery: (req: DraftRecoveryInput): Promise<void> =>
        SaveDraftRecovery(main.DraftRecoveryRequest.createFrom(req)),
    draftRecovery: (): Promise<DraftRecoveryResult> => DraftRecovery(),
    clearDraftRecovery: (): Promise<void> => ClearDraftRecovery(),
    outboxCount: (): Promise<number> => OutboxCount(),
    listOutbox: (): Promise<OutboxItem[]> => ListOutbox(),
    // cancelOutboxItem resolves true when the item was still queued and is now stopped; false means the
    // message had already been sent, so an undo that lost the race can say so.
    cancelOutboxItem: (id: string): Promise<boolean> => CancelOutboxItem(id),
    // A cancelled file dialog returns a Go nil slice, which arrives as null; coalesce it to an empty array
    // so callers can always read .length and filter it.
    pickAttachments: async (): Promise<string[]> => (await PickAttachments()) ?? [],
    replayOutbox: (): Promise<number> => ReplayOutbox(),
    listContacts: (): Promise<Contact[]> => ListContacts(),
    getContact: (id: string): Promise<Contact> => GetContact(id),
    saveContact: (req: ContactInput): Promise<void> => SaveContact(main.ContactRequest.createFrom(req)),
    deleteContact: (id: string): Promise<void> => DeleteContact(id),
    listContactGroups: (): Promise<ContactGroup[]> => ListContactGroups(),
    saveContactGroup: (req: ContactGroupInput): Promise<void> =>
        SaveContactGroup(main.ContactGroupRequest.createFrom(req)),
    deleteContactGroup: (id: string): Promise<void> => DeleteContactGroup(id),
    importContactsFromFile: (): Promise<ContactImportResult> => ImportContactsFromFile(),
    exportContactsToFile: (format: string): Promise<boolean> => ExportContactsToFile(format),
    listCalendars: (): Promise<Calendar[]> => ListCalendars(),
    saveCalendar: (req: CalendarInput): Promise<void> => SaveCalendar(main.CalendarRequest.createFrom(req)),
    deleteCalendar: (id: string): Promise<void> => DeleteCalendar(id),
    listEvents: (): Promise<CalendarEvent[]> => ListEvents(),
    listEventInstances: (from: string, to: string): Promise<CalendarEventInstance[]> =>
        ListEventInstances(from, to),
    getEvent: (id: string): Promise<CalendarEvent> => GetEvent(id),
    // saveEvent returns the saved event's id (freshly generated for a new event), so a newly created
    // meeting can send its invitations without a reload.
    saveEvent: (req: CalendarEventInput): Promise<string> => SaveEvent(main.EventRequest.createFrom(req)),
    saveEventScoped: (req: CalendarEventInput, scope: EventScope, occurrence: string): Promise<void> =>
        SaveEventScoped(main.EventRequest.createFrom(req), scope, occurrence),
    deleteEvent: (id: string): Promise<void> => DeleteEvent(id),
    deleteEventScoped: (scope: EventScope, seriesId: string, occurrence: string): Promise<void> =>
        DeleteEventScoped(scope, seriesId, occurrence),
    importEventsFromFile: (): Promise<number> => ImportEventsFromFile(),
    exportEventsToFile: (): Promise<boolean> => ExportEventsToFile(),
    getInvitation: (messageId: string): Promise<Invitation> => GetInvitation(messageId),
    respondToInvitation: (messageId: string, status: PartStat): Promise<void> =>
        RespondToInvitation(messageId, status),
    removeCancelledMeeting: (messageId: string): Promise<void> => RemoveCancelledMeeting(messageId),
    applyMeetingReply: (messageId: string): Promise<void> => ApplyMeetingReply(messageId),
    sendMeetingRequest: (accountId: string, eventId: string): Promise<void> =>
        SendMeetingRequest(accountId, eventId),
    sendMeetingCancel: (accountId: string, eventId: string): Promise<void> =>
        SendMeetingCancel(accountId, eventId),
    // CalDAV remote calendars. listCalDAVAccounts returns the configured DAV accounts; addCalDAVAccount stores
    // an account and its keychain password; removeCalDAVAccount deletes both; syncCalDAV runs the two-way sync
    // for an account (pushes local changes, then reconciles the server's calendars into the local store).
    listCalDAVAccounts: (): Promise<CalDAVAccount[]> => ListCalDAVAccounts(),
    addCalDAVAccount: (displayName: string, baseUrl: string, username: string, password: string): Promise<void> =>
        AddCalDAVAccount(displayName, baseUrl, username, password),
    removeCalDAVAccount: (id: string): Promise<void> => RemoveCalDAVAccount(id),
    syncCalDAV: (id: string): Promise<void> => SyncCalDAV(id),
    // clipboardText reads the system clipboard through the Wails runtime for Edit > Paste, which
    // the webview cannot do itself (execCommand('paste') is blocked and navigator.clipboard.readText
    // may prompt). An empty clipboard reads as an empty string.
    clipboardText: (): Promise<string> => ClipboardGetText(),
}
