import {useEffect, useRef, useState, type CSSProperties, type DragEvent as ReactDragEvent} from 'react'
import {Folder} from '../api'
import {messageDragType} from './MessageList'
import {
    detectSeparator,
    leafName,
    ancestorPaths,
    descendantUnread,
    nearestParentPath,
    orderFolders,
    placeAdjacent,
} from '../folderPaths'
import {dropZoneFor, folderDragType, resolveFolderDrop, type FolderDropZone} from '../sidebarDnd'
import {usePersistedFolderState} from '../hooks/usePersistedFolderState'

interface FolderTreeProps {
    folders: Folder[]
    selectedFolder: string
    selectedAccount: string
    onSelectFolder: (id: string) => void
    onRenameFolder: (folder: Folder) => void
    onReparentFolder: (folderId: string, newParentId: string) => void
    onDeleteFolder: (folder: Folder) => void
    onDropMessage: (messageId: string, folderId: string) => void
    // onFolderContextMenu opens the folder right-click menu (Paste and friends) at the cursor.
    onFolderContextMenu: (folder: Folder, x: number, y: number) => void
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

// SPRING_DELAY_MS is how long a message must hover a collapsed parent folder before it auto-expands
// (spring-loaded folders): long enough not to fire on a quick pass-over, short enough to feel responsive.
const SPRING_DELAY_MS = 700

// FolderTree renders the folders as a nested, collapsible tree derived from their paths. A collapsed parent
// rolls the unread hidden in its subtree up onto its own badge (outlined, so a rolled-up count reads
// differently from a folder's own unread) and the badge reverts to the folder's own count on expand.
// Custom folders can be dragged to reparent them (a server move) or reorder amongst their siblings (a
// local, persisted order).
// Both the collapsed state and the local order are kept per account in localStorage, so they survive
// restarts. The folder-path and ordering helpers live in ../folderPaths, keeping the pure tree logic out of
// this component.
export function FolderTree(props: FolderTreeProps) {
    const {folders, selectedFolder, selectedAccount} = props
    const {collapsed, order, toggle, persistOrder, expand} = usePersistedFolderState(selectedAccount)
    // dragOverId marks the folder a message is being dragged onto (an into cue). draggingFolderId is the
    // custom folder currently being dragged to move it. folderDrop marks the row and zone a dragged
    // folder is aimed at, driving the drop cue: a box for an into (child) drop, an insertion line for a
    // before or after (same-level) drop.
    const [dragOverId, setDragOverId] = useState<string>('')
    const [draggingFolderId, setDraggingFolderId] = useState<string>('')
    const [folderDrop, setFolderDrop] = useState<{folderId: string; zone: FolderDropZone} | null>(null)
    // springTimer holds the pending auto-expand for the collapsed parent a message is hovering (see
    // scheduleSpring). It is a ref, not state, because it must not trigger a re-render on every dragover.
    const springTimer = useRef<{folderId: string; timer: number} | null>(null)
    useEffect(() => {
        return () => {
            if (springTimer.current) {
                clearTimeout(springTimer.current.timer)
            }
        }
    }, [])

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

    // A folder row accepts two independent kinds of drag, handled by separate paths so a change to one
    // cannot affect the other: a message dropped onto it (moved into the folder) and a custom folder dragged
    // onto it (reparented or reordered). onRowDragOver and onRowDrop only dispatch by the drag's MIME type.

    // clearSpring cancels any pending auto-expand.
    const clearSpring = () => {
        if (springTimer.current) {
            clearTimeout(springTimer.current.timer)
            springTimer.current = null
        }
    }

    // scheduleSpring spring-loads a collapsed parent during a message drag: after hovering it for
    // SPRING_DELAY_MS the folder auto-expands so its sub-folders appear and the message can be dropped into
    // one of them. Hovering a leaf, an already-expanded folder or a different row resets the pending expand.
    const scheduleSpring = (folder: Folder) => {
        if (!(hasChildren(folder.path) && collapsed.has(folder.path))) {
            clearSpring()
            return
        }
        if (springTimer.current?.folderId === folder.id) {
            return
        }
        clearSpring()
        const timer = window.setTimeout(() => {
            springTimer.current = null
            expand(folder.path)
        }, SPRING_DELAY_MS)
        springTimer.current = {folderId: folder.id, timer}
    }

    // A message drag highlights the folder it is over as the drop target and spring-loads a collapsed parent.
    const handleMessageDragOver = (e: ReactDragEvent<HTMLLIElement>, folder: Folder) => {
        e.preventDefault()
        e.dataTransfer.dropEffect = 'move'
        setDragOverId(folder.id)
        scheduleSpring(folder)
    }

    // scheduleFolderSpring spring-loads a collapsed parent during a folder drag, the same as a message drag,
    // so a dragged folder can be nested into a sub-folder that was hidden. A folder cannot land inside its own
    // subtree, so the dragged folder and its descendants are never sprung open.
    const scheduleFolderSpring = (folder: Folder) => {
        if (!draggedFolder || folder.path === draggedFolder.path || folder.path.startsWith(draggedFolder.path + sep)) {
            clearSpring()
            return
        }
        scheduleSpring(folder)
    }

    // A dragged folder aims at a zone: the row's middle nests it inside this folder, the top or bottom edge
    // places it at this folder's own level. The cue shows only for a move resolveFolderDrop allows (not onto
    // itself, its own subtree or a change that does nothing).
    const handleFolderDragOver = (e: ReactDragEvent<HTMLLIElement>, folder: Folder) => {
        if (!draggedFolder) {
            return
        }
        scheduleFolderSpring(folder)
        const zone = dropZoneFor(e.clientY, e.currentTarget.getBoundingClientRect())
        if (resolveFolderDrop(draggedFolder, folder, zone, sep, existing, byPath)) {
            e.preventDefault()
            e.dataTransfer.dropEffect = 'move'
            setFolderDrop({folderId: folder.id, zone})
        } else {
            setFolderDrop((cur) => (cur?.folderId === folder.id ? null : cur))
        }
    }

    const onRowDragOver = (e: ReactDragEvent<HTMLLIElement>, folder: Folder) => {
        if (e.dataTransfer.types.includes(messageDragType)) {
            handleMessageDragOver(e, folder)
        } else if (e.dataTransfer.types.includes(folderDragType)) {
            handleFolderDragOver(e, folder)
        }
    }

    // A dropped folder reparents or reorders relative to the target row; the dragged id travels in the
    // dataTransfer, falling back to the in-flight draggedFolder.
    const handleFolderDrop = (e: ReactDragEvent<HTMLLIElement>, folder: Folder) => {
        const movedFolderId = e.dataTransfer.getData(folderDragType)
        const dragged = movedFolderId ? folders.find((f) => f.id === movedFolderId) : draggedFolder
        setDraggingFolderId('')
        if (!dragged) {
            return
        }
        const zone = dropZoneFor(e.clientY, e.currentTarget.getBoundingClientRect())
        applyFolderDrop(dragged, folder, zone)
    }

    const onRowDrop = (e: ReactDragEvent<HTMLLIElement>, folder: Folder) => {
        e.preventDefault()
        clearSpring()
        setDragOverId('')
        setFolderDrop(null)
        const messageId = e.dataTransfer.getData(messageDragType)
        if (messageId) {
            props.onDropMessage(messageId, folder.id)
            return
        }
        handleFolderDrop(e, folder)
    }

    return (
        <ul className="list" data-folder-list="">
            {visible.map((folder) => {
                const leaf = leafName(folder.path, sep)
                const depth = ancestorPaths(folder.path, sep).length
                const parent = hasChildren(folder.path)
                const isCollapsed = collapsed.has(folder.path)
                // A collapsed parent's children are not rendered, so their unread would otherwise vanish
                // from the sidebar (while still counting toward the account badge). Roll it up onto the
                // collapsed row; an expanded parent shows only its own unread, its children showing theirs.
                const hiddenUnread = parent && isCollapsed ? descendantUnread(folder, folders, sep) : 0
                const badgeCount = folder.unread + hiddenUnread
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
                        onContextMenu={(e) => {
                            e.preventDefault()
                            props.onFolderContextMenu(folder, e.clientX, e.clientY)
                        }}
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
                            clearSpring()
                            setDraggingFolderId('')
                            setDragOverId('')
                            setFolderDrop(null)
                        }}
                        onDragOver={(e) => onRowDragOver(e, folder)}
                        onDragLeave={() => {
                            setDragOverId((id) => (id === folder.id ? '' : id))
                            setFolderDrop((cur) => (cur?.folderId === folder.id ? null : cur))
                            if (springTimer.current?.folderId === folder.id) {
                                clearSpring()
                            }
                        }}
                        onDrop={(e) => onRowDrop(e, folder)}
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
                        {badgeCount > 0 && (
                            <span
                                className={'badge' + (hiddenUnread > 0 ? ' badge-rollup' : '')}
                                title={hiddenUnread > 0 ? `${badgeCount} unread including subfolders` : undefined}
                            >
                                {badgeCount}
                            </span>
                        )}
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
