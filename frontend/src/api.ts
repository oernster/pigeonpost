// api centralises all access to the Wails-bound Go facade so React components depend on this thin
// module rather than the generated bindings directly.
import {
    About,
    Author,
    ListAccounts,
    ListFolders,
    LicenceText,
    ListMessages,
    MarkRead,
    OpenReleasesPage,
    SendMessage,
    SyncAccount,
    Version,
} from '../wailsjs/go/main/App'
import {main} from '../wailsjs/go/models'

export type Account = main.AccountDTO
export type Folder = main.FolderDTO
export type Message = main.MessageDTO
export type AboutInfo = main.AboutDTO

export interface ComposeInput {
    accountId: string
    to: string[]
    cc: string[]
    subject: string
    body: string
}

export const api = {
    listAccounts: (): Promise<Account[]> => ListAccounts(),
    listFolders: (accountId: string): Promise<Folder[]> => ListFolders(accountId),
    listMessages: (folderId: string): Promise<Message[]> => ListMessages(folderId),
    syncAccount: (accountId: string): Promise<void> => SyncAccount(accountId),
    markRead: (messageId: string, read: boolean): Promise<void> => MarkRead(messageId, read),
    about: (): Promise<AboutInfo> => About(),
    licence: (): Promise<string> => LicenceText(),
    version: (): Promise<string> => Version(),
    author: (): Promise<string> => Author(),
    openReleases: (): Promise<void> => OpenReleasesPage(),
    send: (req: ComposeInput): Promise<void> => SendMessage(main.ComposeRequest.createFrom(req)),
}
