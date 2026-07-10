import {useEffect, useMemo, useState} from 'react'
import icon from '../assets/pigeonpost.png'
import {Account, Folder} from '../api'
import {messageDragType} from './MessageList'
import {detectSeparator, leafName, ancestorPaths, moveTargets} from '../folderPaths'

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
    onMoveFolder: (folder: Folder) => void
    // onReparentFolder moves the folder with folderId under newParentId (empty for the top level); it
    // backs the drag-and-drop reparenting, calling the same use case as the move dialog.
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

// specialFolderOrder is the canonical top-to-bottom order for the well-known mailboxes, so Inbox,
// Sent and the rest sit at the top rather than wherever the server happens to list them. Any kind not
// named here (custom folders) ranks after all of these and keeps its original relative order.
const specialFolderOrder = ['inbox', 'drafts', 'sent', 'archive', 'junk', 'trash']

function folderRank(kind: string): number {
    const idx = specialFolderOrder.indexOf(kind)
    return idx === -1 ? specialFolderOrder.length : idx
}

// accountDragType identifies an account row being dragged to reorder. It is distinct from the message
// drag type so a message dropped on an account row is ignored and vice versa.
const accountDragType = 'application/x-pigeonpost-account'

// folderDragType identifies a custom folder row being dragged to reparent it. It is distinct from the
// account and message drag types so each drop target accepts only what it understands.
const folderDragType = 'application/x-pigeonpost-folder'

// FOLDER_DROP_EDGE_FRACTION is the fraction of a folder row's height at its top and at its bottom that
// targets the folder's own level (a sibling drop); the middle band targets inside the folder (a child
// drop). It splits each row into before / into / after zones for reparent drag-and-drop.
const FOLDER_DROP_EDGE_FRACTION = 0.25

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
                            onMoveFolder={props.onMoveFolder}
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
    onMoveFolder: (folder: Folder) => void
    onReparentFolder: (folderId: string, newParentId: string) => void
    onDeleteFolder: (folder: Folder) => void
    onDropMessage: (messageId: string, folderId: string) => void
}

function collapseKey(accountId: string): string {
    return `pigeonpost.collapsed.${accountId}`
}

// The folder-path helpers (detectSeparator, leafName, ancestorPaths) live in ../folderPaths so the move
// dialog can share them; they are imported at the top of this file.

// orderFolders reorders the folders for display so the well-known mailboxes lead (see
// specialFolderOrder) while every subtree stays contiguous under its parent. It walks the tree from
// the roots, sorting siblings at each level by folder rank; the sort is stable, so same-rank siblings
// (in particular custom folders) keep their original server order.
function orderFolders(folders: Folder[], sep: string): Folder[] {
    const byPath = new Map(folders.map((f) => [f.path, f]))
    const nearestParent = (f: Folder): string | null => {
        const ancestors = ancestorPaths(f.path, sep)
        for (let i = ancestors.length - 1; i >= 0; i--) {
            if (byPath.has(ancestors[i])) {
                return ancestors[i]
            }
        }
        return null
    }
    const childrenOf = new Map<string, Folder[]>()
    const roots: Folder[] = []
    folders.forEach((f) => {
        const parent = nearestParent(f)
        if (parent === null) {
            roots.push(f)
            return
        }
        const siblings = childrenOf.get(parent) ?? []
        siblings.push(f)
        childrenOf.set(parent, siblings)
    })
    const sortSiblings = (arr: Folder[]) =>
        [...arr].sort((a, b) => folderRank(a.kind) - folderRank(b.kind))
    const ordered: Folder[] = []
    const walk = (f: Folder) => {
        ordered.push(f)
        sortSiblings(childrenOf.get(f.path) ?? []).forEach(walk)
    }
    sortSiblings(roots).forEach(walk)
    return ordered
}

// FolderTree renders the folders as a nested, collapsible tree derived from their paths. The collapsed
// state is persisted per account in localStorage, so it survives restarts.
function FolderTree(props: FolderTreeProps) {
    const {folders, selectedFolder, selectedAccount} = props
    const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
    const [dragOverId, setDragOverId] = useState<string>('')
    // draggingFolderId is the custom folder currently being dragged to reparent it; topDropActive marks
    // the top-level drop strip as hovered. Both drive the drag cues.
    const [draggingFolderId, setDraggingFolderId] = useState<string>('')
    const [topDropActive, setTopDropActive] = useState(false)
    // folderDrop marks the row and zone a dragged folder is currently aimed at, driving the drop cue: a
    // box for an into (child) drop, an edge line for a before or after (same-level sibling) drop.
    const [folderDrop, setFolderDrop] = useState<{folderId: string; zone: FolderDropZone} | null>(null)

    useEffect(() => {
        try {
            const raw = localStorage.getItem(collapseKey(selectedAccount))
            setCollapsed(new Set(raw ? (JSON.parse(raw) as string[]) : []))
        } catch {
            setCollapsed(new Set())
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

    // While a folder is being dragged, validTargetIds is the set of folder ids it may be dropped onto to
    // reparent it (plus the empty id for the top level), so a drop is only accepted and highlighted on a
    // valid target. It is derived from the same rule as the move dialog. canDropTopLevel is true when the
    // dragged folder is nested, so it can be dragged out to the top level.
    const validTargetIds = useMemo(() => {
        const dragged = draggingFolderId ? folders.find((f) => f.id === draggingFolderId) : undefined
        return dragged ? new Set(moveTargets(dragged, folders).map((t) => t.id)) : new Set<string>()
    }, [draggingFolderId, folders])
    const canDropTopLevel = validTargetIds.has('')

    const paths = folders.map((f) => f.path)
    const sep = detectSeparator(paths)
    // byPath maps a folder path to its id, for resolving a folder's parent when a sibling drop needs the
    // parent's id as the destination.
    const byPath = new Map(folders.map((f) => [f.path, f.id]))
    const hasChildren = (path: string) => paths.some((p) => p.startsWith(path + sep))
    const ordered = orderFolders(folders, sep)
    // A folder is visible only when none of its ancestors are collapsed.
    const visible = ordered.filter((f) => ancestorPaths(f.path, sep).every((a) => !collapsed.has(a)))
    // The folder list is a single focus-ring stop: only one folder is tabbable (the selected one, or the
    // first when none is selected), and Up/Down move between folders from there.
    const tabStopId = selectedFolder || (visible.length > 0 ? visible[0].id : '')

    return (
        <>
            {canDropTopLevel && (
                <div
                    className={'folder-drop-top' + (topDropActive ? ' active' : '')}
                    onDragOver={(e) => {
                        if (e.dataTransfer.types.includes(folderDragType)) {
                            e.preventDefault()
                            e.dataTransfer.dropEffect = 'move'
                            setTopDropActive(true)
                        }
                    }}
                    onDragLeave={() => setTopDropActive(false)}
                    onDrop={(e) => {
                        e.preventDefault()
                        const movedFolderId = e.dataTransfer.getData(folderDragType)
                        setTopDropActive(false)
                        setDraggingFolderId('')
                        if (movedFolderId) {
                            props.onReparentFolder(movedFolderId, '')
                        }
                    }}
                >
                    Move to top level
                </div>
            )}
            <ul className="list" data-folder-list="">
            {visible.map((folder) => {
                const leaf = leafName(folder.path, sep)
                const depth = ancestorPaths(folder.path, sep).length
                const parent = hasChildren(folder.path)
                const isCollapsed = collapsed.has(folder.path)
                // siblingParentId is the destination for a same-level drop on this row: this folder's own
                // parent id (or the empty id when it already sits at the top level). dropParentFor maps a
                // drop zone on this row to the id the dragged folder is reparented under.
                const parentPath = ancestorPaths(folder.path, sep).slice(-1)[0] ?? ''
                const siblingParentId = parentPath ? (byPath.get(parentPath) ?? '') : ''
                const dropParentFor = (zone: FolderDropZone) => (zone === 'into' ? folder.id : siblingParentId)
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
                        style={{paddingLeft: 14 + depth * 14}}
                        tabIndex={folder.id === tabStopId ? 0 : -1}
                        onClick={() => props.onSelectFolder(folder.id)}
                        onKeyDown={(e) => {
                            // Only the row itself drives navigation; a key on a child button (the collapse
                            // toggle, move, rename or delete) is left to that button. Only the selected row's
                            // action buttons are in the Tab order (a roving tab stop), so Tab steps from the
                            // row into its buttons then straight out of the list, while Up/Down move between
                            // folders. That keeps the buttons keyboard-reachable without trapping Tab in the
                            // tree: it is always at most a few Tabs out to the next region.
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
                            // Only custom folders are draggable; reparent by dropping onto another folder or
                            // the top-level strip. The dragged id travels in the dataTransfer; draggingFolderId
                            // in state drives the valid-target highlighting during the drag.
                            e.stopPropagation()
                            setDraggingFolderId(folder.id)
                            e.dataTransfer.setData(folderDragType, folder.id)
                            e.dataTransfer.effectAllowed = 'move'
                        }}
                        onDragEnd={() => {
                            setDraggingFolderId('')
                            setDragOverId('')
                            setTopDropActive(false)
                            setFolderDrop(null)
                        }}
                        onDragOver={(e) => {
                            // A message drops onto any folder (into it). A dragged folder aims at a zone: the
                            // row's middle nests it inside this folder, the top or bottom edge places it at
                            // this folder's own level (a sibling). The drop is accepted only when that
                            // destination is valid (not itself, its own subtree or a no-op to where it is).
                            if (e.dataTransfer.types.includes(messageDragType)) {
                                e.preventDefault()
                                e.dataTransfer.dropEffect = 'move'
                                setDragOverId(folder.id)
                                return
                            }
                            if (!e.dataTransfer.types.includes(folderDragType)) {
                                return
                            }
                            const zone = dropZoneFor(e.clientY, e.currentTarget.getBoundingClientRect())
                            if (validTargetIds.has(dropParentFor(zone))) {
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
                            if (movedFolderId) {
                                const dropParentId = dropParentFor(dropZoneFor(e.clientY, e.currentTarget.getBoundingClientRect()))
                                if (validTargetIds.has(dropParentId)) {
                                    props.onReparentFolder(movedFolderId, dropParentId)
                                }
                            }
                            setDraggingFolderId('')
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
                                    aria-label={`Move ${folder.name}`}
                                    title="Move folder"
                                    onClick={(e) => {
                                        e.stopPropagation()
                                        props.onMoveFolder(folder)
                                    }}
                                >
                                    &#8618;
                                </button>
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
        </>
    )
}
