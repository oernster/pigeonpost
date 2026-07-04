// api centralises all access to the Wails-bound Go facade so React components depend on this thin
// module rather than the generated bindings directly.
import {
    About,
    AddAccount,
    Author,
    DeleteMessage,
    DeleteMessagePermanent,
    DeleteTag,
    GetMessageBody,
    ListAccounts,
    ListFolders,
    LicenceText,
    ListMessages,
    ListTags,
    MarkFlagged,
    MarkRead,
    MessageTags,
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
    ImportContactsFromFile,
    ImportEventsFromFile,
    ListCalendars,
    ListContactGroups,
    ListContacts,
    ListEvents,
    ListRules,
    SaveCalendar,
    SaveContact,
    SaveContactGroup,
    SaveEvent,
    MoveMessage,
    RenameFolder,
    SaveRule,
    CancelOutboxItem,
    ListOutbox,
    OpenExternal,
    OpenReleasesPage,
    OutboxCount,
    PickAttachments,
    RemoveAccount,
    ReplayOutbox,
    SaveDraft,
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
export type MessageBody = main.MessageBodyDTO
export type OutboxItem = main.OutboxItemDTO
export type UnreadCountsResult = main.UnreadCountsDTO
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

export interface CalendarInput {
    id: string
    name: string
    colour: string
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
    saveMessageAs: (messageId: string, suggestedName: string): Promise<void> =>
        SaveMessageAs(messageId, suggestedName),
    moveMessage: (messageId: string, destFolderId: string): Promise<void> => MoveMessage(messageId, destFolderId),
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
    send: (req: ComposeInput): Promise<void> => SendMessage(main.ComposeRequest.createFrom(req)),
    saveDraft: (req: ComposeInput): Promise<void> => SaveDraft(main.ComposeRequest.createFrom(req)),
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
    getEvent: (id: string): Promise<CalendarEvent> => GetEvent(id),
    saveEvent: (req: CalendarEventInput): Promise<void> => SaveEvent(main.EventRequest.createFrom(req)),
    deleteEvent: (id: string): Promise<void> => DeleteEvent(id),
    importEventsFromFile: (): Promise<number> => ImportEventsFromFile(),
    exportEventsToFile: (): Promise<boolean> => ExportEventsToFile(),
}
