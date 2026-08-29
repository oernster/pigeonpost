import {useRef} from 'react'
import icon from '../assets/pigeonpost.png'
import {Account, Folder} from '../api'
import {icons} from '../icons'
import {AccountPicker} from './AccountPicker'
import {FolderTree} from './FolderTree'
import {useDragAutoScroll} from '../hooks/useDragAutoScroll'
import type {ElsewhereCue} from '../newMail'

interface SidebarProps {
    accounts: Account[]
    selectedAccount: string
    // The unified mailbox entry: shown while the View tick is on, highlighted while it is the open view,
    // badged with the cross-account unread total. Selecting it opens the combined all-inboxes list.
    unifiedEnabled: boolean
    unifiedSelected: boolean
    unifiedUnread: number
    onSelectUnified: () => void
    // The Snoozed entry: shown while any message is hidden (like the Outbox it appears only when it has
    // contents), badged with how many. Selecting it lists the hidden messages with their due times.
    snoozedCount: number
    snoozedSelected: boolean
    onSelectSnoozed: () => void
    // syncingAccountIds holds the ids of accounts whose mailbox sync is in progress, so the picker can
    // show a syncing cue against each one independently.
    syncingAccountIds: ReadonlySet<string>
    // unreadByAccount maps an account id to its unread message count. An account with no unread mail is
    // absent from the map.
    unreadByAccount: {[accountId: string]: number}
    // elsewhereCue summarises mail newly arrived on accounts other than the selected one, for the
    // closed picker's trigger badge.
    elsewhereCue: ElsewhereCue
    folders: Folder[]
    selectedFolder: string
    onSelectAccount: (id: string) => void
    onSelectFolder: (id: string) => void
    onEditAccount: (account: Account) => void
    onDeleteAccount: (account: Account) => void
    onNewFolder: () => void
    onRenameFolder: (folder: Folder) => void
    // onNewSubfolder creates a folder under an existing one, from the row's plus button.
    onNewSubfolder: (folder: Folder) => void
    // onReparentFolder moves the folder with folderId under newParentId (empty for the top level) on the
    // server; it backs the drag-and-drop reparenting. A same-level reorder is handled locally and never
    // calls this.
    onReparentFolder: (folderId: string, newParentId: string) => void
    onDeleteFolder: (folder: Folder) => void
    // onDropMessage moves a dragged message into the folder and reports whether the drop was taken, so
    // the row confirms only a move that is actually happening.
    onDropMessage: (messageId: string, folderId: string) => boolean
    // onFolderContextMenu opens the folder right-click menu (Paste and friends) at the cursor.
    onFolderContextMenu: (folder: Folder, x: number, y: number) => void
    // canManageFolders is false for POP3 accounts, which have no server-side folders to create.
    canManageFolders: boolean
}

export function Sidebar(props: SidebarProps) {
    // Only the folder tree scrolls. The brand icon, the cross-account entries, the account picker and the
    // Folders header all stay pinned above it, so the folders get the whole of the space below them and
    // the scrollbar spans the folders alone. The tree auto-scrolls while a message or a folder is dragged
    // near its top or bottom edge, so a folder below the fold is reachable without letting go of the drag.
    const scrollRef = useRef<HTMLDivElement | null>(null)
    useDragAutoScroll(scrollRef)
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
                <>
                    <SidebarHeader {...props}/>
                    <div className="sidebar-scroll" ref={scrollRef}>
                        {props.selectedAccount && <SidebarFolders {...props}/>}
                    </div>
                </>
            )}
        </aside>
    )
}

// SidebarHeader is the pinned part: the cross-account entries, the account picker and the Folders section
// header. None of it scrolls, so the folder tree below keeps the rest of the pane.
function SidebarHeader(props: SidebarProps) {
    return (
        <div className="sidebar-header">
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
                            <span className="folder-icon"><img src={icons.mail} alt="" draggable={false}/></span>
                            All inboxes
                        </span>
                        {props.unifiedUnread > 0 && <span className="badge">{props.unifiedUnread}</span>}
                    </li>
                </ul>
            )}
            {props.snoozedCount > 0 && (
                <ul className="list" data-snoozed-entry="">
                    <li
                        className={'list-item folder snoozed' + (props.snoozedSelected ? ' selected' : '')}
                        tabIndex={0}
                        onClick={props.onSelectSnoozed}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                                e.preventDefault()
                                props.onSelectSnoozed()
                            }
                        }}
                    >
                        <span className="folder-name">
                            <span className="folder-icon"><img src={icons.snooze} alt="" draggable={false}/></span>
                            Snoozed
                        </span>
                        <span className="badge">{props.snoozedCount}</span>
                    </li>
                </ul>
            )}
            <AccountPicker
                accounts={props.accounts}
                selectedAccount={props.selectedAccount}
                syncingAccountIds={props.syncingAccountIds}
                unreadByAccount={props.unreadByAccount}
                elsewhere={props.elsewhereCue}
                onSelectAccount={props.onSelectAccount}
                onEditAccount={props.onEditAccount}
                onDeleteAccount={props.onDeleteAccount}
            />
            {props.selectedAccount && (
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
            )}
        </div>
    )
}

// SidebarFolders is the one scrolling region: the folder tree of the selected account, else the prompt
// to sync when nothing is cached yet.
function SidebarFolders(props: SidebarProps) {
    const {selectedAccount, folders, selectedFolder} = props
    if (folders.length === 0) {
        return <p className="empty-body indented">No folders cached. Press Sync to fetch them.</p>
    }
    return (
        <FolderTree
            folders={folders}
            selectedFolder={selectedFolder}
            selectedAccount={selectedAccount}
            onSelectFolder={props.onSelectFolder}
            onRenameFolder={props.onRenameFolder}
            onNewSubfolder={props.onNewSubfolder}
            onReparentFolder={props.onReparentFolder}
            onDeleteFolder={props.onDeleteFolder}
            onDropMessage={props.onDropMessage}
            onFolderContextMenu={props.onFolderContextMenu}
        />
    )
}
