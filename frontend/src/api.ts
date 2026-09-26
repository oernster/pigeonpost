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
    Conversation,
    CopyMessage,
    CreateFolder,
    CreateSubfolder,
    DeleteFolder,
    FolderUIState,
    SaveFolderUIState,
    MoveFolder,
    MoveMessage,
    RenameFolder,
    CheckForUpdates,
    OpenExternal,
    OpenReleasesPage,
    RemoveAccount,
    SignInMicrosoft,
    SnoozeMessage,
    SnoozedCount,
    UnsnoozeMessage,
    OpenAttachment,
    OpenEmailAttachment,
    SaveAllAttachments,
    SaveAttachment,
    SaveMessageAs,
    SaveTag,
    SearchMessages,
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
// The filter-rule calls and their types live in their own module; they are re-exported and spread
// into the api object below, so callers still reach them as api.* and import their types from here.
import {rulesApi} from './apiRules'
export type {Rule, RuleAction, RuleBackfill, RuleBackfillProgress, RuleCondition, RuleInput} from './apiRules'
import {templatesApi} from './apiTemplates'
import {bulkApi} from './apiBulk'
export type {BulkResult} from './apiBulk'
import {calendarApi} from './apiCalendar'
export type {
    CalDAVAccount, Calendar, CalendarEvent, CalendarEventInput, CalendarEventInstance, CalendarInput, Invitation,
    MeetingAttendee, MeetingAttendeeInput, MeetingOrganizerInput, PartStat,
} from './apiCalendar'
// EventScope is an enum, a value rather than only a type, so it is re-exported as one.
export {EventScope} from './apiCalendar'
import {contactsApi} from './apiContacts'
export type {
    Contact, ContactAddressInput, ContactEmailInput, ContactGroup, ContactGroupInput, ContactImportResult,
    ContactInput, ContactPhoneInput,
} from './apiContacts'
import {composeApi} from './apiCompose'
export type {ComposeInput, DraftRecoveryInput, DraftRecoveryResult, OutboxItem} from './apiCompose'
export type {Template, TemplateAttachment, TemplateFile, TemplateInput} from './apiTemplates'

export type Account = main.AccountDTO
export type Folder = main.FolderDTO
// Wails returns plain JSON at runtime (no class methods); the UI spreads messages for optimistic
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
export type UpdateStatus = main.UpdateStatusDTO
export type Tag = main.TagDTO
// MessageBody drops the generated convertValues helper so an outbox message's body can be built as a
// plain object literal; the nested AttachmentDTO array carries no helper of its own.
export type MessageBody = Omit<main.MessageBodyDTO, 'convertValues'>
export type Attachment = main.AttachmentDTO
export type UnreadCountsResult = main.UnreadCountsDTO
// FolderUIStateResult is an account's persisted folder display state: the custom folders' local order
// and the collapsed folder paths.
export type FolderUIStateResult = main.FolderUIStateDTO
// ConversationEntry is one message of a thread with the folder it sits in, so the reader can say which
// of them you received and which you sent.
export type ConversationEntry = main.ConversationEntryDTO

// MoveResult reports where a move-shaped action (move, delete to Trash, junk, rescue) put the
// message: the id it will carry in its destination folder; empty when the server did not say.
// Undo entries are built from it.
export type MoveResult = main.MoveResultDTO
export interface TagInput {
    id: string
    name: string
    colour: string
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

// MicrosoftSignInInput adds a Microsoft account. It carries no password, servers or email address: the
// OAuth flow supplies the address and the token. The name, signature and send-as addresses are collected
// before the browser sign-in, so the account is complete the moment it is added.
export interface MicrosoftSignInInput {
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
    updateAccount: (req: AccountSetupInput): Promise<void> => UpdateAccount(main.AccountSetupRequest.createFrom(req)),
    // updateAccountProfile changes only the name, signature and send-as identities of an existing account,
    // without re-verifying its credentials. It is the edit path for an OAuth account.
    updateAccountProfile: (req: AccountProfileInput): Promise<void> =>
        UpdateAccountProfile(main.AccountProfileRequest.createFrom(req)),
    // signInMicrosoft runs the interactive OAuth flow (opens the browser, waits for consent) and resolves
    // with the signed-in address so the caller can select the new account. The name, signature and
    // send-as addresses are collected by the wizard first, so the account is complete when it is added.
    signInMicrosoft: (req: MicrosoftSignInInput): Promise<string> =>
        SignInMicrosoft(main.MicrosoftSignInRequest.createFrom(req)),
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
    // conversation returns every cached message threading with this one, across the account's folders,
    // oldest first. The list's own grouping sees a single folder, so this is the only view that pairs a
    // message with the answer you sent to it.
    conversation: (messageId: string): Promise<ConversationEntry[]> => Conversation(messageId),
    createFolder: (accountId: string, name: string): Promise<void> => CreateFolder(accountId, name),
    // createSubfolder creates a folder named name directly under parentFolderId. The parent supplies the
    // account and the server's hierarchy delimiter, so name is a leaf name and never a path.
    createSubfolder: (parentFolderId: string, name: string): Promise<void> =>
        CreateSubfolder(parentFolderId, name),
    renameFolder: (folderId: string, newName: string): Promise<void> => RenameFolder(folderId, newName),
    deleteFolder: (folderId: string): Promise<void> => DeleteFolder(folderId),
    // moveFolder reparents a folder under newParentId on the server (an empty newParentId moves it to
    // the top level). Same-level reordering is a local display concern and does not call this.
    moveFolder: (folderId: string, newParentId: string): Promise<void> => MoveFolder(folderId, newParentId),
    // folderUIState and saveFolderUIState persist the account's local folder display state (the custom
    // folders' order and the collapsed paths) in the backend database, so it survives an application
    // update; the WebView's localStorage is only a warm cache of the same state.
    folderUIState: (accountId: string): Promise<FolderUIStateResult> => FolderUIState(accountId),
    saveFolderUIState: (accountId: string, order: string[], collapsed: string[]): Promise<void> =>
        SaveFolderUIState(accountId, order, collapsed),
    ...rulesApi,
    ...templatesApi,
    ...bulkApi,
    ...composeApi,
    ...contactsApi,
    ...calendarApi,
    about: (): Promise<AboutInfo> => About(),
    licence: (): Promise<string> => LicenceText(),
    version: (): Promise<string> => Version(),
    author: (): Promise<string> => Author(),
    openReleases: (): Promise<void> => OpenReleasesPage(),
    checkForUpdates: (skippedVersion: string): Promise<UpdateStatus> => CheckForUpdates(skippedVersion),
    minimiseToTray: (): Promise<void> => MinimiseToTray(),
    requestQuit: (): Promise<void> => RequestQuit(),
    // clipboardText reads the system clipboard through the Wails runtime for Edit > Paste, which
    // the webview cannot do itself (execCommand('paste') is blocked and navigator.clipboard.readText
    // may prompt). An empty clipboard reads as an empty string.
    clipboardText: (): Promise<string> => ClipboardGetText(),
}
