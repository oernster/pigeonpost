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
    SaveCalendar,
    SaveContact,
    SaveContactGroup,
    SaveEvent,
    SaveEventScoped,
    DeleteEventScoped,
    MoveMessage,
    RenameFolder,
    SaveRule,
    CancelOutboxItem,
    ListOutbox,
    OpenExternal,
    OpenReleasesPage,
    ClearDraftRecovery,
    DraftRecovery,
    OutboxCount,
    PickAttachments,
    RemoveAccount,
    ReplayOutbox,
    SaveDraft,
    SaveDraftRecovery,
    SaveAttachment,
    SaveMessageAs,
    SaveTag,
    SearchMessages,
    SendMessage,
    SetMessageTag,
    SyncAccount,
    SyncFolder,
    UnreadCounts,
    UpdateAccount,
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
// MessageBody drops the generated convertValues helper so an outbox message's body can be built as a
// plain object literal; the nested AttachmentDTO array carries no helper of its own.
export type MessageBody = Omit<main.MessageBodyDTO, 'convertValues'>
export type Attachment = main.AttachmentDTO
export type OutboxItem = main.OutboxItemDTO
export type UnreadCountsResult = main.UnreadCountsDTO
export type BulkDeleteResult = main.BulkDeleteResultDTO
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

export interface ComposeInput {
    accountId: string
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
}

export const api = {
    listAccounts: (): Promise<Account[]> => ListAccounts(),
    addAccount: (req: AccountSetupInput): Promise<void> => AddAccount(main.AccountSetupRequest.createFrom(req)),
    removeAccount: (accountId: string): Promise<void> => RemoveAccount(accountId),
    updateAccount: (req: AccountSetupInput): Promise<void> => UpdateAccount(main.AccountSetupRequest.createFrom(req)),
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
    // deleteMessages / deleteMessagesPermanent delete the whole selection in one batched backend call
    // (grouped by folder, one server connection per folder) rather than one round trip per message.
    deleteMessages: (ids: string[]): Promise<BulkDeleteResult> => DeleteMessages(ids),
    deleteMessagesPermanent: (ids: string[]): Promise<BulkDeleteResult> => DeleteMessagesPermanent(ids),
    saveMessageAs: (messageId: string, suggestedName: string): Promise<void> =>
        SaveMessageAs(messageId, suggestedName),
    saveAttachment: (messageId: string, index: number): Promise<void> => SaveAttachment(messageId, index),
    moveMessage: (messageId: string, destFolderId: string): Promise<void> => MoveMessage(messageId, destFolderId),
    markJunk: (messageId: string): Promise<void> => MarkJunk(messageId),
    copyMessage: (messageId: string, destFolderId: string): Promise<void> => CopyMessage(messageId, destFolderId),
    createFolder: (accountId: string, name: string): Promise<void> => CreateFolder(accountId, name),
    renameFolder: (folderId: string, newName: string): Promise<void> => RenameFolder(folderId, newName),
    deleteFolder: (folderId: string): Promise<void> => DeleteFolder(folderId),
    listRules: (): Promise<Rule[]> => ListRules(),
    saveRule: (req: RuleInput): Promise<void> => SaveRule(main.RuleRequest.createFrom(req)),
    deleteRule: (ruleId: string): Promise<void> => DeleteRule(ruleId),
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
    pickAttachments: (): Promise<string[]> => PickAttachments(),
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
