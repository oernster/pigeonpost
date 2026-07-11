import {useState, type CSSProperties} from 'react'
import icon from '../assets/pigeonpost.png'
import {Account, Folder} from '../api'
import {messageDragType} from './MessageList'
import {
    detectSeparator,
    leafName,
    ancestorPaths,
    nearestParentPath,
    orderFolders,
    placeAdjacent,
} from '../folderPaths'
import {dropZoneFor, folderDragType, resolveFolderDrop, type FolderDropZone} from '../sidebarDnd'
import {AccountList} from './AccountList'
import {usePersistedFolderState} from '../hooks/usePersistedFolderState'

interface SidebarProps {
    accounts: Account[]
    selectedAccount: string
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

const folderIcon: Record<string, string> = {
    inbox: '\u{1F4E5}',
    sent: '\u{1F4E4}',
    drafts: '\u{1F4DD}',
    trash: '\u{1F5D1}\u{FE0F}',
    junk: '\u{1F6AB}',
    archive: '\u{1F5C3}\u{FE0F}',
    outbox: '\u{1F4EE}',
    custom: '\u{1F4C1}',
}

// FOLDER_INDENT_STEP_PX is the left indent added per tree depth (and the base indent of a top-level row),
// so a folder at depth d sits at (d + 1) steps. The same value drives the drag insertion line's indent.
const FOLDER_INDENT_STEP_PX = 14


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

interface FolderTreeProps {
    folders: Folder[]
    selectedFolder: string
    selectedAccount: string
    onSelectFolder: (id: string) => void
    onRenameFolder: (folder: Folder) => void
    onReparentFolder: (folderId: string, newParentId: string) => void
    onDeleteFolder: (folder: Folder) => void
    onDropMessage: (messageId: string, folderId: string) => void
}

// The folder-path and ordering helpers (detectSeparator, leafName, ancestorPaths, nearestParentPath,
// orderFolders, placeAdjacent) live in ../folderPaths and are imported at the top of this file, keeping
// the pure tree logic out of the component.

// FolderTree renders the folders as a nested, collapsible tree derived from their paths. Custom folders
// can be dragged to reparent them (a server move) or reorder amongst their siblings (a local, persisted
// order). Both the collapsed state and the local order are kept per account in localStorage, so they
// survive restarts.
function FolderTree(props: FolderTreeProps) {
    const {folders, selectedFolder, selectedAccount} = props
    const {collapsed, order, toggle, persistOrder} = usePersistedFolderState(selectedAccount)
    // dragOverId marks the folder a message is being dragged onto (an into cue). draggingFolderId is the
    // custom folder currently being dragged to move it. folderDrop marks the row and zone a dragged
    // folder is aimed at, driving the drop cue: a box for an into (child) drop, an insertion line for a
    // before or after (same-level) drop.
    const [dragOverId, setDragOverId] = useState<string>('')
    const [draggingFolderId, setDraggingFolderId] = useState<string>('')
    const [folderDrop, setFolderDrop] = useState<{folderId: string; zone: FolderDropZone} | null>(null)

    const paths = folders.map((f) => f.path)
    const sep = detectSeparator(paths)
    const existing = new Set(paths)
    // byPath maps a folder path to its id, so a drop that reparents under a folder path can name the id.
    const byPath = new Map(folders.map((f) => [f.path, f.id]))
    const hasChildren = (path: string) => paths.some((p) => p.startsWith(path + sep))
    const ordered = orderFolders(folders, sep, order)
    // A folder is visible only when none of its ancestors are collapsed.
    const visible = ordered.filter((f) => ancestorPaths(f.path, sep).every((a) => !collapsed.has(a)))
    // The folder list is a single focus-ring stop: only one folder is tabbable (the selected one; the
    // first when none is selected). Up/Down move between folders from there.
    const tabStopId = selectedFolder || (visible.length > 0 ? visible[0].id : '')
    const draggedFolder = draggingFolderId ? folders.find((f) => f.id === draggingFolderId) : undefined

    // customSiblingPaths returns the paths of the custom folders under parentPath in their current display
    // order, which is the sibling group a reorder or a gap reparent splices the moved folder into.
    const customSiblingPaths = (parentPath: string): string[] =>
        ordered
            .filter((f) => f.kind === 'custom' && nearestParentPath(f.path, existing, sep) === parentPath)
            .map((f) => f.path)

    // applyFolderDrop carries out a resolved drop: a local reorder persists a new order and stops; a
    // reparent records the landing position for a gap drop (so it survives the refresh) then asks the
    // server to move the folder.
    const applyFolderDrop = (dragged: Folder, target: Folder, zone: FolderDropZone) => {
        const action = resolveFolderDrop(dragged, target, zone, sep, existing, byPath)
        if (!action) {
            return
        }
        if (action.kind === 'reorder') {
            persistOrder(
                placeAdjacent(order, customSiblingPaths(action.parentPath), dragged.path, action.anchorPath, action.after),
            )
            return
        }
        if (action.anchorPath !== undefined && action.after !== undefined) {
            const newPath = (action.parentPath ? action.parentPath + sep : '') + leafName(dragged.path, sep)
            persistOrder(
                placeAdjacent(order, customSiblingPaths(action.parentPath), newPath, action.anchorPath, action.after),
            )
        }
        props.onReparentFolder(dragged.id, action.parentId)
    }

    return (
        <ul className="list" data-folder-list="">
            {visible.map((folder) => {
                const leaf = leafName(folder.path, sep)
                const depth = ancestorPaths(folder.path, sep).length
                const parent = hasChildren(folder.path)
                const isCollapsed = collapsed.has(folder.path)
                const rowIndentPx = (depth + 1) * FOLDER_INDENT_STEP_PX
                const rowStyle = {
                    paddingLeft: rowIndentPx,
                    ['--row-indent']: `${rowIndentPx}px`,
                } as CSSProperties
                return (
                    <li
                        key={folder.id}
                        data-folder-id={folder.id}
                        className={
                            'list-item folder' +
                            (folder.id === selectedFolder ? ' selected' : '') +
                            (folder.id === dragOverId ? ' drag-over' : '') +
                            (folder.id === draggingFolderId ? ' dragging' : '') +
                            (folderDrop && folderDrop.folderId === folder.id ? ' drag-' + folderDrop.zone : '')
                        }
                        draggable={folder.kind === 'custom'}
                        style={rowStyle}
                        tabIndex={folder.id === tabStopId ? 0 : -1}
                        onClick={() => props.onSelectFolder(folder.id)}
                        onKeyDown={(e) => {
                            // Only the row itself drives navigation here; a key while focus is on a child
                            // button (the collapse toggle, rename or delete) is left to that button. The
                            // collapse toggle stays out of the Tab order (Left/Right drives it); the selected
                            // row's rename and delete ARE tabbable, so Tab steps the row then those two then
                            // out of the list, while Up/Down move between folders.
                            if (e.target !== e.currentTarget) {
                                return
                            }
                            const li = e.currentTarget
                            const moveFocus = (forward: boolean) => {
                                let sibling = forward ? li.nextElementSibling : li.previousElementSibling
                                if (!sibling && li.parentElement) {
                                    sibling = forward
                                        ? li.parentElement.firstElementChild
                                        : li.parentElement.lastElementChild
                                }
                                if (sibling instanceof HTMLElement) {
                                    sibling.focus()
                                    const id = sibling.getAttribute('data-folder-id')
                                    if (id) {
                                        props.onSelectFolder(id)
                                    }
                                }
                            }
                            if (e.key === 'Enter') {
                                e.preventDefault()
                                props.onSelectFolder(folder.id)
                                return
                            }
                            // Up/Down move between folders, wrapping at the ends.
                            if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                                e.preventDefault()
                                e.stopPropagation()
                                moveFocus(e.key === 'ArrowDown')
                                return
                            }
                            // Right expands a collapsed parent, else moves to the next folder; Left collapses
                            // an expanded parent, else moves to the previous. Consumed here (not bubbled to the
                            // ring) so the arrows navigate the tree and Tab is what leaves the list.
                            if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
                                e.preventDefault()
                                e.stopPropagation()
                                if (e.key === 'ArrowRight' && parent && isCollapsed) {
                                    toggle(folder.path)
                                    return
                                }
                                if (e.key === 'ArrowLeft' && parent && !isCollapsed) {
                                    toggle(folder.path)
                                    return
                                }
                                moveFocus(e.key === 'ArrowRight')
                                return
                            }
                        }}
                        onDragStart={(e) => {
                            // Only custom folders are draggable; move by dropping onto another folder (nest)
                            // or into the gap at a level (reparent up/out or reorder). The dragged id travels
                            // in the dataTransfer; draggingFolderId in state drives the drop-target resolving
                            // and the dimmed-row cue during the drag.
                            e.stopPropagation()
                            setDraggingFolderId(folder.id)
                            e.dataTransfer.setData(folderDragType, folder.id)
                            e.dataTransfer.effectAllowed = 'move'
                        }}
                        onDragEnd={() => {
                            setDraggingFolderId('')
                            setDragOverId('')
                            setFolderDrop(null)
                        }}
                        onDragOver={(e) => {
                            // A message drops onto any folder (into it). A dragged folder aims at a zone: the
                            // row's middle nests it inside this folder, the top or bottom edge places it at
                            // this folder's own level. The drop is accepted only when resolveFolderDrop allows
                            // it (not itself, its own subtree or a change that does nothing).
                            if (e.dataTransfer.types.includes(messageDragType)) {
                                e.preventDefault()
                                e.dataTransfer.dropEffect = 'move'
                                setDragOverId(folder.id)
                                return
                            }
                            if (!e.dataTransfer.types.includes(folderDragType) || !draggedFolder) {
                                return
                            }
                            const zone = dropZoneFor(e.clientY, e.currentTarget.getBoundingClientRect())
                            if (resolveFolderDrop(draggedFolder, folder, zone, sep, existing, byPath)) {
                                e.preventDefault()
                                e.dataTransfer.dropEffect = 'move'
                                setFolderDrop({folderId: folder.id, zone})
                            } else {
                                setFolderDrop((cur) => (cur?.folderId === folder.id ? null : cur))
                            }
                        }}
                        onDragLeave={() => {
                            setDragOverId((id) => (id === folder.id ? '' : id))
                            setFolderDrop((cur) => (cur?.folderId === folder.id ? null : cur))
                        }}
                        onDrop={(e) => {
                            e.preventDefault()
                            setDragOverId('')
                            setFolderDrop(null)
                            const messageId = e.dataTransfer.getData(messageDragType)
                            if (messageId) {
                                props.onDropMessage(messageId, folder.id)
                                return
                            }
                            const movedFolderId = e.dataTransfer.getData(folderDragType)
                            const dragged = movedFolderId ? folders.find((f) => f.id === movedFolderId) : draggedFolder
                            setDraggingFolderId('')
                            if (!dragged) {
                                return
                            }
                            const zone = dropZoneFor(e.clientY, e.currentTarget.getBoundingClientRect())
                            applyFolderDrop(dragged, folder, zone)
                        }}
                    >
                        <span className="folder-name">
                            {parent ? (
                                <button
                                    type="button"
                                    className="folder-toggle"
                                    tabIndex={-1}
                                    aria-label={isCollapsed ? `Expand ${leaf}` : `Collapse ${leaf}`}
                                    onClick={(e) => {
                                        e.stopPropagation()
                                        toggle(folder.path)
                                    }}
                                >
                                    {isCollapsed ? '▸' : '▾'}
                                </button>
                            ) : (
                                <span className="folder-toggle-spacer"/>
                            )}
                            <span className="folder-icon">{folderIcon[folder.kind] ?? folderIcon.custom}</span>
                            {leaf}
                        </span>
                        {folder.unread > 0 && <span className="badge">{folder.unread}</span>}
                        {folder.kind === 'custom' && (
                            <span className="account-actions">
                                <button
                                    className="account-action"
                                    tabIndex={folder.id === tabStopId ? 0 : -1}
                                    aria-label={`Rename ${folder.name}`}
                                    title="Rename folder"
                                    onClick={(e) => {
                                        e.stopPropagation()
                                        props.onRenameFolder(folder)
                                    }}
                                >
                                    &#9998;
                                </button>
                                <button
                                    className="account-action delete"
                                    tabIndex={folder.id === tabStopId ? 0 : -1}
                                    aria-label={`Delete ${folder.name}`}
                                    title="Delete folder"
                                    onClick={(e) => {
                                        e.stopPropagation()
                                        props.onDeleteFolder(folder)
                                    }}
                                >
                                    &times;
                                </button>
                            </span>
                        )}
                    </li>
                )
            })}
        </ul>
    )
}
