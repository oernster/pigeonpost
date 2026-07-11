// api centralises all access to the Wails-bound Go facade so React components depend on this thin
// module rather than the generated bindings directly.
import {
    About,
    AddAccount,
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
    ListTags,
    MarkFlagged,
    MarkJunk,
    MarkRead,
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
    SyncAccount,
    SyncFolder,
    UnreadCounts,
    UpdateAccount,
    UpdateAccountProfile,
    Version,
} from '../wailsjs/go/main/App'
import {main} from '../wailsjs/go/models'

export type Account = main.AccountDTO
export type Folder = main.FolderDTO
// Wails returns plain JSON at runtime (no class methods), and the UI spreads messages for optimistic
// updates, so Message is the data-only shape. Omitting convertValues keeps this valid even after the
// generated MessageDTO regains that helper method on a `wails generate`.
export type Message = Omit<main.MessageDTO, 'convertValues'>
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
export type Contact = main.ContactDTO
export type ContactGroup = main.ContactGroupDTO

export interface ContactEmailInput {
    label: string
    address: string
}

export interface ContactPhoneInput {
    label: string
    number: string
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
    emails: ContactEmailInput[]
    phones: ContactPhoneInput[]
}

export interface ContactGroupInput {
    id: string
    name: string
    members: string[]
}

export type Calendar = main.CalendarDTO
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
    attachmentMessageIds: string[]
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
    listMessages: (folderId: string): Promise<Message[]> => ListMessages(folderId),
    searchMessages: (query: string): Promise<Message[]> => SearchMessages(query),
    messageBody: (messageId: string): Promise<MessageBody> => GetMessageBody(messageId),
    openExternal: (url: string): Promise<void> => OpenExternal(url),
    syncAccount: (accountId: string): Promise<void> => SyncAccount(accountId),
    syncFolder: (folderId: string): Promise<void> => SyncFolder(folderId),
    markRead: (messageId: string, read: boolean): Promise<void> => MarkRead(messageId, read),
    markFlagged: (messageId: string, flagged: boolean): Promise<void> => MarkFlagged(messageId, flagged),
    deleteMessage: (messageId: string): Promise<void> => DeleteMessage(messageId),
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
    saveAllAttachments: (messageId: string): Promise<void> => SaveAllAttachments(messageId),
    moveMessage: (messageId: string, destFolderId: string): Promise<void> => MoveMessage(messageId, destFolderId),
    markJunk: (messageId: string): Promise<void> => MarkJunk(messageId),
    copyMessage: (messageId: string, destFolderId: string): Promise<void> => CopyMessage(messageId, destFolderId),
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
    send: (req: ComposeInput): Promise<void> => SendMessage(main.ComposeRequest.createFrom(req)),
    saveDraft: (req: ComposeInput): Promise<void> => SaveDraft(main.ComposeRequest.createFrom(req)),
    saveDraftRecovery: (req: DraftRecoveryInput): Promise<void> =>
        SaveDraftRecovery(main.DraftRecoveryRequest.createFrom(req)),
    draftRecovery: (): Promise<DraftRecoveryResult> => DraftRecovery(),
    clearDraftRecovery: (): Promise<void> => ClearDraftRecovery(),
    outboxCount: (): Promise<number> => OutboxCount(),
    listOutbox: (): Promise<OutboxItem[]> => ListOutbox(),
    cancelOutboxItem: (id: string): Promise<void> => CancelOutboxItem(id),
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
    importContactsFromFile: (): Promise<number> => ImportContactsFromFile(),
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
}
