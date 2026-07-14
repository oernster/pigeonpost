import icon from '../assets/pigeonpost.png'
import {Account, Folder} from '../api'
import {AccountList} from './AccountList'
import {FolderTree} from './FolderTree'

interface SidebarProps {
    accounts: Account[]
    selectedAccount: string
    // The unified mailbox entry: shown while the View tick is on, highlighted while it is the open view,
    // badged with the cross-account unread total. Selecting it opens the combined all-inboxes list.
    unifiedEnabled: boolean
    unifiedSelected: boolean
    unifiedUnread: number
    onSelectUnified: () => void
    // syncingAccountIds holds the ids of accounts whose mailbox sync is in progress, so each row can show a
    // small syncing cue and stays independent of the others.
    syncingAccountIds: ReadonlySet<string>
    // unreadByAccount maps an account id to its unread message count. An account with no unread mail is
    // absent from the map.
    unreadByAccount: {[accountId: string]: number}
    folders: Folder[]
    selectedFolder: string
    onSelectAccount: (id: string) => void
    onSelectFolder: (id: string) => void
    onEditAccount: (account: Account) => void
    onDeleteAccount: (account: Account) => void
    // onReorderAccounts persists a new account order (the full list of account ids, top to bottom) after
    // a drag or an up/down move.
    onReorderAccounts: (orderedIds: string[]) => void
    onNewFolder: () => void
    onRenameFolder: (folder: Folder) => void
    // onReparentFolder moves the folder with folderId under newParentId (empty for the top level) on the
    // server; it backs the drag-and-drop reparenting. A same-level reorder is handled locally and never
    // calls this.
    onReparentFolder: (folderId: string, newParentId: string) => void
    onDeleteFolder: (folder: Folder) => void
    onDropMessage: (messageId: string, folderId: string) => void
    // canManageFolders is false for POP3 accounts, which have no server-side folders to create.
    canManageFolders: boolean
}

export function Sidebar(props: SidebarProps) {
    return (
        <aside className="pane sidebar">
            <img className="sidebar-brand" src={icon} alt="" aria-hidden="true"/>
            {props.accounts.length === 0 ? (
                <div className="empty-state">
                    <div className="empty-title">No accounts yet</div>
                    <p className="empty-body">
                        Use "Add account" to configure a mail account.
                    </p>
                </div>
            ) : (
                <SidebarContent {...props}/>
            )}
        </aside>
    )
}

function SidebarContent(props: SidebarProps) {
    const {selectedAccount, folders, selectedFolder} = props
    return (
        <>
            {props.unifiedEnabled && (
                <ul className="list" data-unified-entry="">
                    <li
                        className={'list-item folder unified' + (props.unifiedSelected ? ' selected' : '')}
                        tabIndex={0}
                        onClick={props.onSelectUnified}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                                e.preventDefault()
                                props.onSelectUnified()
                            }
                        }}
                    >
                        <span className="folder-name">
                            <span className="folder-icon">{'\u{1F4EC}'}</span>
                            All inboxes
                        </span>
                        {props.unifiedUnread > 0 && <span className="badge">{props.unifiedUnread}</span>}
                    </li>
                </ul>
            )}
            <AccountList
                accounts={props.accounts}
                selectedAccount={selectedAccount}
                syncingAccountIds={props.syncingAccountIds}
                unreadByAccount={props.unreadByAccount}
                onSelectAccount={props.onSelectAccount}
                onEditAccount={props.onEditAccount}
                onDeleteAccount={props.onDeleteAccount}
                onReorderAccounts={props.onReorderAccounts}
            />

            {selectedAccount && (
                <>
                    <div className="section-header">
                        <span className="section-label">Folders</span>
                        {props.canManageFolders && (
                            <button
                                className="section-action"
                                title="New folder"
                                aria-label="New folder"
                                onClick={props.onNewFolder}
                            >
                                &#43;
                            </button>
                        )}
                    </div>
                    {folders.length === 0 ? (
                        <p className="empty-body indented">No folders cached. Press Sync to fetch them.</p>
                    ) : (
                        <FolderTree
                            folders={folders}
                            selectedFolder={selectedFolder}
                            selectedAccount={selectedAccount}
                            onSelectFolder={props.onSelectFolder}
                            onRenameFolder={props.onRenameFolder}
                            onReparentFolder={props.onReparentFolder}
                            onDeleteFolder={props.onDeleteFolder}
                            onDropMessage={props.onDropMessage}
                        />
                    )}
                </>
            )}
        </>
    )
}
