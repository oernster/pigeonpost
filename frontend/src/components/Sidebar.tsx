import {useEffect, useState, type CSSProperties} from 'react'
import icon from '../assets/pigeonpost.png'
import {Account, Folder} from '../api'
import {messageDragType} from './MessageList'
import {
    detectSeparator,
    leafName,
    ancestorPaths,
    nearestParentPath,
    folderRank,
    orderFolders,
    placeAdjacent,
} from '../folderPaths'

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

// accountDragType identifies an account row being dragged to reorder. It is distinct from the message
// drag type so a message dropped on an account row is ignored and vice versa.
const accountDragType = 'application/x-pigeonpost-account'

// folderDragType identifies a custom folder row being dragged to reparent or reorder it. It is distinct
// from the account and message drag types so each drop target accepts only what it understands.
const folderDragType = 'application/x-pigeonpost-folder'

// FOLDER_INDENT_STEP_PX is the left indent added per tree depth (and the base indent of a top-level row),
// so a folder at depth d sits at (d + 1) steps. The same value drives the drag insertion line's indent.
const FOLDER_INDENT_STEP_PX = 14

// FOLDER_DROP_EDGE_FRACTION is the fraction of a folder row's height at its top and at its bottom that
// targets the folder's own level (a sibling drop, drawn as an insertion line); the middle band targets
// inside the folder (a child drop, drawn as a box). At 0.3 the top and bottom thirds are the sibling
// zones and the middle ~40% nests, so the same-level target is easy to hit.
const FOLDER_DROP_EDGE_FRACTION = 0.3

// FolderDropZone is where a folder drag is aimed on a target row: before or after places the dragged
// folder at the target's own level (a sibling of it); into nests the dragged folder inside the target.
type FolderDropZone = 'before' | 'into' | 'after'

// dropZoneFor returns which zone the pointer at clientY falls in for a row with bounds rect.
function dropZoneFor(clientY: number, rect: DOMRect): FolderDropZone {
    const offset = clientY - rect.top
    if (offset < rect.height * FOLDER_DROP_EDGE_FRACTION) {
        return 'before'
    }
    if (offset > rect.height * (1 - FOLDER_DROP_EDGE_FRACTION)) {
        return 'after'
    }
    return 'into'
}

// FolderDropAction is the resolved outcome of a folder drag over a target row: a local reorder amongst
// same-level siblings (no server call) or a reparent under a new parent (an empty parentId is the top
// level). A gap reparent also carries the anchor path it was dropped next to, so the moved folder can
// keep that position once the server refresh brings it in under the new parent.
type FolderDropAction =
    | {kind: 'reorder'; parentPath: string; anchorPath: string; after: boolean}
    | {kind: 'reparent'; parentId: string; parentPath: string; anchorPath?: string; after?: boolean}

// resolveFolderDrop decides what dropping dragged onto target in the given zone should do. It returns
// null when the drop is not allowed (onto itself, into its own subtree or a move that changes nothing).
// An into drop nests dragged inside target. A before or after drop aims at target's own level: when that
// is dragged's current level it is a local reorder against target, otherwise it reparents dragged under
// target's parent (the top level when target is top-level). Only same-rank (custom) siblings reorder,
// since rank fixes the well-known mailboxes ahead of every custom folder.
function resolveFolderDrop(
    dragged: Folder,
    target: Folder,
    zone: FolderDropZone,
    sep: string,
    existing: Set<string>,
    byPath: Map<string, string>,
): FolderDropAction | null {
    if (target.id === dragged.id) {
        return null
    }
    const inDraggedSubtree = (p: string) => p === dragged.path || p.startsWith(dragged.path + sep)
    const draggedParent = nearestParentPath(dragged.path, existing, sep)
    if (zone === 'into') {
        if (inDraggedSubtree(target.path) || draggedParent === target.path) {
            return null
        }
        return {kind: 'reparent', parentId: target.id, parentPath: target.path}
    }
    const targetParent = nearestParentPath(target.path, existing, sep)
    if (inDraggedSubtree(targetParent)) {
        return null
    }
    const after = zone === 'after'
    if (draggedParent === targetParent) {
        if (folderRank(target.kind) !== folderRank(dragged.kind)) {
            return null
        }
        return {kind: 'reorder', parentPath: targetParent, anchorPath: target.path, after}
    }
    const parentId = targetParent ? byPath.get(targetParent) ?? '' : ''
    return {kind: 'reparent', parentId, parentPath: targetParent, anchorPath: target.path, after}
}

// moveId returns a copy of ids with fromId moved to the index toId currently sits at (a splice move),
// which is the drag-and-drop reordering. The input is not mutated.
function moveId(ids: string[], fromId: string, toId: string): string[] {
    const from = ids.indexOf(fromId)
    const to = ids.indexOf(toId)
    if (from < 0 || to < 0 || from === to) {
        return ids
    }
    const next = [...ids]
    next.splice(from, 1)
    next.splice(to, 0, fromId)
    return next
}

// swapId returns a copy of ids with the entries at i and j exchanged, which is one step of an up or down
// move. The input is not mutated.
function swapId(ids: string[], i: number, j: number): string[] {
    const next = [...ids]
    ;[next[i], next[j]] = [next[j], next[i]]
    return next
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
    const {accounts, selectedAccount, unreadByAccount, folders, selectedFolder} = props
    // dragId is the account being dragged; dragOverId is the row it is currently over. Both drive the
    // visual cue while a reorder drag is in flight.
    const [dragId, setDragId] = useState('')
    const [dragOverId, setDragOverId] = useState('')
    const accountIds = accounts.map((a) => a.id)
    // Reordering is only meaningful with more than one account, so the drag and the up/down buttons are
    // enabled only then.
    const canReorder = accounts.length > 1
    // The account list is a single focus-ring stop: only one row is tabbable (the selected account,
    // otherwise the first when none is selected). Up/Down move between accounts, wrapping at the ends.
    const accountTabStopId = selectedAccount || (accounts.length > 0 ? accounts[0].id : '')
    return (
        <>
            <div className="section-label">Accounts</div>
            <ul className="list" data-account-list="">
                {accounts.map((account, index) => (
                    <li
                        key={account.id}
                        data-account-id={account.id}
                        tabIndex={account.id === accountTabStopId ? 0 : -1}
                        className={
                            'list-item account' +
                            (account.id === selectedAccount ? ' selected' : '') +
                            (account.id === dragOverId ? ' drag-over' : '') +
                            (account.id === dragId ? ' dragging' : '')
                        }
                        draggable={canReorder}
                        onClick={() => props.onSelectAccount(account.id)}
                        onKeyDown={(e) => {
                            // Only the row itself drives selection and Up/Down; a key on a child action
                            // button (edit, remove, reorder) is left to that button.
                            if (e.target !== e.currentTarget) {
                                return
                            }
                            if (e.key === 'Enter' || e.key === ' ' || e.key === 'Spacebar') {
                                e.preventDefault()
                                props.onSelectAccount(account.id)
                                return
                            }
                            // Up/Down move between accounts within this one focus-ring stop, wrapping at the
                            // ends. Left/Right bubble to the window handler, which steps the focus ring.
                            if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                                e.preventDefault()
                                e.stopPropagation()
                                const li = e.currentTarget
                                const parent = li.parentElement
                                let sibling = e.key === 'ArrowDown' ? li.nextElementSibling : li.previousElementSibling
                                if (!sibling && parent) {
                                    sibling = e.key === 'ArrowDown' ? parent.firstElementChild : parent.lastElementChild
                                }
                                if (sibling instanceof HTMLElement) {
                                    sibling.focus()
                                    const id = sibling.getAttribute('data-account-id')
                                    if (id) {
                                        props.onSelectAccount(id)
                                    }
                                }
                            }
                        }}
                        onDragStart={(e) => {
                            setDragId(account.id)
                            e.dataTransfer.setData(accountDragType, account.id)
                            e.dataTransfer.effectAllowed = 'move'
                        }}
                        onDragEnd={() => {
                            setDragId('')
                            setDragOverId('')
                        }}
                        onDragOver={(e) => {
                            if (e.dataTransfer.types.includes(accountDragType)) {
                                e.preventDefault()
                                e.dataTransfer.dropEffect = 'move'
                                setDragOverId(account.id)
                            }
                        }}
                        onDragLeave={() => setDragOverId((id) => (id === account.id ? '' : id))}
                        onDrop={(e) => {
                            e.preventDefault()
                            const from = e.dataTransfer.getData(accountDragType)
                            setDragId('')
                            setDragOverId('')
                            if (from && from !== account.id) {
                                props.onReorderAccounts(moveId(accountIds, from, account.id))
                            }
                        }}
                    >
                        <span className="account-badge-slot">
                            {(unreadByAccount[account.id] ?? 0) > 0 && (
                                <span
                                    className="badge account-badge"
                                    title={`${unreadByAccount[account.id]} unread`}
                                >
                                    {unreadByAccount[account.id]}
                                </span>
                            )}
                        </span>
                        <span className="item-text">
                            <span className="item-title" title={account.displayName}>{account.displayName}</span>
                            <span className="item-sub" title={account.email}>{account.email}</span>
                            {props.syncingAccountIds.has(account.id) && (
                                <span className="account-syncing">Synchronising…</span>
                            )}
                        </span>
                        <span className="account-actions">
                            {canReorder && (
                                <>
                                    <button
                                        className="account-action"
                                        tabIndex={account.id === accountTabStopId ? 0 : -1}
                                        aria-label={`Move ${account.email} up`}
                                        title="Move up"
                                        disabled={index === 0}
                                        onClick={(e) => {
                                            e.stopPropagation()
                                            props.onReorderAccounts(swapId(accountIds, index, index - 1))
                                        }}
                                    >
                                        &#8593;
                                    </button>
                                    <button
                                        className="account-action"
                                        tabIndex={account.id === accountTabStopId ? 0 : -1}
                                        aria-label={`Move ${account.email} down`}
                                        title="Move down"
                                        disabled={index === accounts.length - 1}
                                        onClick={(e) => {
                                            e.stopPropagation()
                                            props.onReorderAccounts(swapId(accountIds, index, index + 1))
                                        }}
                                    >
                                        &#8595;
                                    </button>
                                </>
                            )}
                            <button
                                className="account-action"
                                tabIndex={account.id === accountTabStopId ? 0 : -1}
                                aria-label={`Edit ${account.email}`}
                                title="Edit account"
                                onClick={(e) => {
                                    e.stopPropagation()
                                    props.onEditAccount(account)
                                }}
                            >
                                &#9998;
                            </button>
                            <button
                                className="account-action delete"
                                tabIndex={account.id === accountTabStopId ? 0 : -1}
                                aria-label={`Remove ${account.email}`}
                                title="Remove account"
                                onClick={(e) => {
                                    e.stopPropagation()
                                    props.onDeleteAccount(account)
                                }}
                            >
                                &times;
                            </button>
                        </span>
                    </li>
                ))}
            </ul>

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

function collapseKey(accountId: string): string {
    return `pigeonpost.collapsed.${accountId}`
}

// folderOrderKey names the per-account localStorage entry holding the custom folders' local display
// order (a list of folder paths). IMAP has no folder order of its own, so a same-level reorder is a
// purely local, persisted display concern.
function folderOrderKey(accountId: string): string {
    return `pigeonpost.folderorder.${accountId}`
}

// The folder-path and ordering helpers (detectSeparator, leafName, ancestorPaths, nearestParentPath,
// folderRank, orderFolders, placeAdjacent) live in ../folderPaths and are imported at the top of this
// file, keeping the pure tree logic out of the component.

// FolderTree renders the folders as a nested, collapsible tree derived from their paths. Custom folders
// can be dragged to reparent them (a server move) or reorder amongst their siblings (a local, persisted
// order). Both the collapsed state and the local order are kept per account in localStorage, so they
// survive restarts.
function FolderTree(props: FolderTreeProps) {
    const {folders, selectedFolder, selectedAccount} = props
    const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
    // dragOverId marks the folder a message is being dragged onto (an into cue). draggingFolderId is the
    // custom folder currently being dragged to move it. folderDrop marks the row and zone a dragged
    // folder is aimed at, driving the drop cue: a box for an into (child) drop, an insertion line for a
    // before or after (same-level) drop. order is the persisted local order of the custom folders.
    const [dragOverId, setDragOverId] = useState<string>('')
    const [draggingFolderId, setDraggingFolderId] = useState<string>('')
    const [folderDrop, setFolderDrop] = useState<{folderId: string; zone: FolderDropZone} | null>(null)
    const [order, setOrder] = useState<string[]>([])

    useEffect(() => {
        try {
            const raw = localStorage.getItem(collapseKey(selectedAccount))
            setCollapsed(new Set(raw ? (JSON.parse(raw) as string[]) : []))
        } catch {
            setCollapsed(new Set())
        }
    }, [selectedAccount])

    useEffect(() => {
        try {
            const raw = localStorage.getItem(folderOrderKey(selectedAccount))
            setOrder(raw ? (JSON.parse(raw) as string[]) : [])
        } catch {
            setOrder([])
        }
    }, [selectedAccount])

    const toggle = (path: string) => {
        setCollapsed((prev) => {
            const next = new Set(prev)
            if (next.has(path)) {
                next.delete(path)
            } else {
                next.add(path)
            }
            try {
                localStorage.setItem(collapseKey(selectedAccount), JSON.stringify([...next]))
            } catch {
                // A storage failure just means the state is not remembered; the UI still works.
            }
            return next
        })
    }

    const persistOrder = (next: string[]) => {
        setOrder(next)
        try {
            localStorage.setItem(folderOrderKey(selectedAccount), JSON.stringify(next))
        } catch {
            // A storage failure just means the order is not remembered; the UI still works.
        }
    }

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
                            // Delete removes this folder: the keyboard equivalent of the row's delete
                            // button, showing the same confirmation. Only custom folders are deletable, so a
                            // well-known row just swallows the key. Either way the key is consumed here so it
                            // never bubbles to the window handler to delete the message selection while focus
                            // sits on a folder.
                            if (e.key === 'Delete') {
                                e.preventDefault()
                                e.stopPropagation()
                                if (folder.kind === 'custom') {
                                    props.onDeleteFolder(folder)
                                }
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
