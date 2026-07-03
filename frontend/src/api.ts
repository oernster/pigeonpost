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
    DeleteFolder,
    DeleteRule,
    ListRules,
    MoveMessage,
    RenameFolder,
    SaveRule,
    OpenExternal,
    OpenReleasesPage,
    OutboxCount,
    RemoveAccount,
    ReplayOutbox,
    SaveDraft,
    SaveMessageAs,
    SaveTag,
    SearchMessages,
    SendMessage,
    SetMessageTag,
    SyncAccount,
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

export interface RuleInput {
    id: string
    name: string
    field: string
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
}

export interface AccountSetupInput {
    displayName: string
    email: string
    password: string
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
    listMessages: (folderId: string): Promise<Message[]> => ListMessages(folderId),
    searchMessages: (query: string): Promise<Message[]> => SearchMessages(query),
    messageBody: (messageId: string): Promise<MessageBody> => GetMessageBody(messageId),
    openExternal: (url: string): Promise<void> => OpenExternal(url),
    syncAccount: (accountId: string): Promise<void> => SyncAccount(accountId),
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
    replayOutbox: (): Promise<number> => ReplayOutbox(),
}
