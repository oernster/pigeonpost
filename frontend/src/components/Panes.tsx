import type {CSSProperties, Dispatch, ReactNode, SetStateAction} from 'react'
import type {Account, Folder, UnreadCountsResult} from '../api'
import {UNIFIED_FOLDER_ID} from '../unified'
import {SNOOZED_FOLDER_ID} from '../snooze'
import type {ElsewhereCue} from '../newMail'
import type {FolderPrompt} from '../hooks/useFolders'
import type {PaneWidthsControl} from '../hooks/usePaneWidths'
import {Sidebar} from './Sidebar'
import {PaneSplitters} from './PaneSplitters'

export interface PanesProps {
    // The layout state: previewEnabled is the reading pane, paneWidths drives the grid columns and the
    // draggable dividers that sit on their boundaries.
    previewEnabled: boolean
    paneWidths: PaneWidthsControl
    // readingFull is a message opened full-width, which only happens with the reading pane off.
    // readerAvailable says there is something for the reader to show: a selected message, else a
    // thread being viewed as a conversation. Both must hold for the reader to take the whole width.
    readingFull: boolean
    readerAvailable: boolean
    // The two panes App builds, passed as elements. Which of them is on screen is decided here; what
    // each contains is not this component's business.
    messageListEl: ReactNode
    readerEl: ReactNode
    // Everything below wires the sidebar. It takes the underlying values rather than a built props
    // object, so the wiring has one home instead of being written out again by the caller.
    accounts: Account[]
    selectedAccount: string
    unifiedEnabled: boolean
    unifiedSelected: boolean
    unreadCounts: UnreadCountsResult
    snoozedCount: number
    snoozedSelected: boolean
    syncingAccountIds: ReadonlySet<string>
    elsewhereCue: ElsewhereCue
    folders: Folder[]
    selectedFolder: string
    canManageFolders: boolean
    selectAccount: (id: string) => Promise<void>
    selectFolder: (id: string) => Promise<void>
    setAccountToEdit: Dispatch<SetStateAction<Account | null>>
    setAccountToDelete: Dispatch<SetStateAction<Account | null>>
    setFolderPrompt: Dispatch<SetStateAction<FolderPrompt | null>>
    setFolderToDelete: Dispatch<SetStateAction<Folder | null>>
    setFolderContextMenu: Dispatch<SetStateAction<{folder: Folder; x: number; y: number} | null>>
    reparentFolder: (folderId: string, newParentId: string) => Promise<void>
    dropMessageOnFolder: (messageId: string, folderId: string) => boolean
}

// Panes is the three-column body: the sidebar, then the message list and the reader. It owns which of
// the three are on screen and the widths they are laid out at, so the grid, its CSS variables, the
// splitters and the choice between the panes all sit together rather than being spread across App's
// JSX. There are three arrangements: both panes with the reading pane on, the reader alone when a
// message is open full-width, otherwise the list alone.
export function Panes(props: PanesProps) {
    const {previewEnabled, paneWidths, readingFull, readerAvailable, messageListEl, readerEl} = props
    // The widths reach the grid as CSS variables, so the layout is one declaration rather than inline
    // sizes on each column.
    const paneStyle = {
        ['--sidebar-w']: `${paneWidths.widths.sidebar}px`,
        ['--list-w']: `${paneWidths.widths.list}px`,
    } as CSSProperties
    return (
        <div className={'panes' + (previewEnabled ? '' : ' no-preview')} style={paneStyle}>
            <Sidebar
                accounts={props.accounts}
                selectedAccount={props.selectedAccount}
                unifiedEnabled={props.unifiedEnabled}
                unifiedSelected={props.unifiedSelected}
                unifiedUnread={props.unreadCounts.total}
                onSelectUnified={() => void props.selectFolder(UNIFIED_FOLDER_ID)}
                snoozedCount={props.snoozedCount}
                snoozedSelected={props.snoozedSelected}
                onSelectSnoozed={() => void props.selectFolder(SNOOZED_FOLDER_ID)}
                syncingAccountIds={props.syncingAccountIds}
                unreadByAccount={props.unreadCounts.byAccount}
                elsewhereCue={props.elsewhereCue}
                folders={props.folders}
                selectedFolder={props.selectedFolder}
                onSelectAccount={(id) => void props.selectAccount(id)}
                onSelectFolder={(id) => void props.selectFolder(id)}
                onEditAccount={(account) => props.setAccountToEdit(account)}
                onDeleteAccount={(account) => props.setAccountToDelete(account)}
                onNewFolder={() => props.setFolderPrompt({mode: 'create'})}
                onRenameFolder={(folder) => props.setFolderPrompt({mode: 'rename', folder})}
                onNewSubfolder={(folder) => props.setFolderPrompt({mode: 'create', parent: folder})}
                onReparentFolder={(folderId, newParentId) => void props.reparentFolder(folderId, newParentId)}
                onDeleteFolder={(folder) => props.setFolderToDelete(folder)}
                onDropMessage={props.dropMessageOnFolder}
                onFolderContextMenu={(folder, x, y) => props.setFolderContextMenu({folder, x, y})}
                canManageFolders={props.canManageFolders}
            />
            {previewEnabled ? (
                <>
                    {messageListEl}
                    {readerEl}
                </>
            ) : readingFull && readerAvailable ? (
                readerEl
            ) : (
                messageListEl
            )}
            <PaneSplitters control={paneWidths} showListSplitter={previewEnabled}/>
        </div>
    )
}
