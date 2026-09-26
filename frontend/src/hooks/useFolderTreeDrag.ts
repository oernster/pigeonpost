import {useEffect, useRef, useState, type DragEvent as ReactDragEvent} from 'react'
import {Folder} from '../api'
import {messageDragType} from '../components/MessageList'
import {leafName, placeAdjacent} from '../folderPaths'
import {dropZoneFor, folderDragType, resolveFolderDrop, type FolderDropZone} from '../sidebarDnd'

// SPRING_DELAY_MS is how long a message must hover a collapsed parent folder before it auto-expands
// (spring-loaded folders): long enough not to fire on a quick pass-over, short enough to feel responsive.
const SPRING_DELAY_MS = 700

// A completed drop flashes the folder that took the message, so it is obvious which folder it landed in
// rather than which one the pointer happened to be near. These must match the folder-drop-flash animation
// in accounts-and-folders.css: the class is removed once the pulses have run.
const DROP_FLASH_PULSE_MS = 900
const DROP_FLASH_PULSES = 2

type RowDragEvent = ReactDragEvent<HTMLLIElement>

// FolderTreeDragDeps is what dragging needs from the tree: the folders and the path facts derived from them,
// the collapsed set and local order with their writers, plus the two things a drop can ask of the app.
export interface FolderTreeDragDeps {
    folders: Folder[]
    sep: string
    existing: Set<string>
    byPath: Map<string, string>
    collapsed: Set<string>
    order: string[]
    expand: (path: string) => void
    persistOrder: (order: string[]) => void
    hasChildren: (path: string) => boolean
    // customSiblingPaths is the custom folders under a parent in display order, the group a reorder or a
    // gap reparent splices the moved folder into.
    customSiblingPaths: (parentPath: string) => string[]
    onReparentFolder: (folderId: string, newParentId: string) => void
    // onDropMessage moves a dragged message into the folder and reports whether the drop was taken.
    onDropMessage: (messageId: string, folderId: string) => boolean
}

export interface RowDragHandlers {
    onDragStart: (e: RowDragEvent) => void
    onDragEnd: () => void
    onDragOver: (e: RowDragEvent) => void
    onDragLeave: () => void
    onDrop: (e: RowDragEvent) => void
}

export interface FolderTreeDrag {
    // dragOverId is the folder a message is over; draggingFolderId the custom folder being dragged;
    // droppedId the folder a message just landed in (the flash); folderDrop the row and zone a dragged
    // folder is aimed at (the drop cue).
    dragOverId: string
    draggingFolderId: string
    droppedId: string
    folderDrop: {folderId: string; zone: FolderDropZone} | null
    rowDrag: (folder: Folder) => RowDragHandlers
}

// useFolderTreeDrag owns everything the folder tree does under a drag. A row accepts two independent kinds:
// a message dropped onto it (moved into the folder) and a custom folder dragged onto it (reparented or
// reordered), handled by separate paths so a change to one cannot affect the other. A collapsed parent
// springs open under either after a short hover; a message that lands flashes the folder that took it.
export function useFolderTreeDrag(deps: FolderTreeDragDeps): FolderTreeDrag {
    const {folders, sep, existing, byPath, collapsed, order, expand, persistOrder, hasChildren, customSiblingPaths} = deps
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
    // droppedId marks the folder a message has just landed in, for the confirmation flash. flashTimer holds
    // the pending clear, so a second drop while the first is still pulsing restarts the flash rather than
    // having the older timer cut the newer one short.
    const [droppedId, setDroppedId] = useState<string>('')
    const flashTimer = useRef<number | null>(null)
    useEffect(() => {
        return () => {
            if (springTimer.current) {
                clearTimeout(springTimer.current.timer)
            }
            if (flashTimer.current !== null) {
                clearTimeout(flashTimer.current)
            }
        }
    }, [])

    const draggedFolder = draggingFolderId ? folders.find((f) => f.id === draggingFolderId) : undefined

    // flashDrop lights the folder that took the drop for the length of the animation, then clears it so the
    // row goes back to its ordinary styling and the class is free to re-apply on the next drop.
    const flashDrop = (folderId: string) => {
        if (flashTimer.current !== null) {
            clearTimeout(flashTimer.current)
        }
        setDroppedId('')
        // A frame with the class off, so re-dropping into the same folder restarts the animation rather
        // than leaving the class in place and playing nothing.
        requestAnimationFrame(() => setDroppedId(folderId))
        flashTimer.current = window.setTimeout(() => {
            flashTimer.current = null
            setDroppedId('')
        }, DROP_FLASH_PULSE_MS * DROP_FLASH_PULSES)
    }

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
        deps.onReparentFolder(dragged.id, action.parentId)
    }

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
    const handleMessageDragOver = (e: RowDragEvent, folder: Folder) => {
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
    const handleFolderDragOver = (e: RowDragEvent, folder: Folder) => {
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

    const onRowDragOver = (e: RowDragEvent, folder: Folder) => {
        if (e.dataTransfer.types.includes(messageDragType)) {
            handleMessageDragOver(e, folder)
        } else if (e.dataTransfer.types.includes(folderDragType)) {
            handleFolderDragOver(e, folder)
        }
    }

    // A dropped folder reparents or reorders relative to the target row; the dragged id travels in the
    // dataTransfer, falling back to the in-flight draggedFolder.
    const handleFolderDrop = (e: RowDragEvent, folder: Folder) => {
        const movedFolderId = e.dataTransfer.getData(folderDragType)
        const dragged = movedFolderId ? folders.find((f) => f.id === movedFolderId) : draggedFolder
        setDraggingFolderId('')
        if (!dragged) {
            return
        }
        const zone = dropZoneFor(e.clientY, e.currentTarget.getBoundingClientRect())
        applyFolderDrop(dragged, folder, zone)
    }

    const onRowDrop = (e: RowDragEvent, folder: Folder) => {
        e.preventDefault()
        clearSpring()
        setDragOverId('')
        setFolderDrop(null)
        const messageId = e.dataTransfer.getData(messageDragType)
        if (messageId) {
            // Only a drop that is actually moving something is confirmed on screen. A drop the app skips
            // (already in this folder, across accounts, a repeat of one still in flight) must not flash; otherwise
            // the cue would be reassuring about a move that is not happening.
            if (deps.onDropMessage(messageId, folder.id)) {
                flashDrop(folder.id)
            }
            return
        }
        handleFolderDrop(e, folder)
    }

    const rowDrag = (folder: Folder): RowDragHandlers => ({
        onDragStart: (e) => {
            // Only custom folders are draggable; move by dropping onto another folder (nest) or into the gap
            // at a level (reparent up/out or reorder). The dragged id travels in the dataTransfer;
            // draggingFolderId in state drives the drop-target resolving and the dimmed-row cue during the drag.
            e.stopPropagation()
            setDraggingFolderId(folder.id)
            e.dataTransfer.setData(folderDragType, folder.id)
            e.dataTransfer.effectAllowed = 'move'
        },
        onDragEnd: () => {
            clearSpring()
            setDraggingFolderId('')
            setDragOverId('')
            setFolderDrop(null)
        },
        onDragOver: (e) => onRowDragOver(e, folder),
        onDragLeave: () => {
            setDragOverId((id) => (id === folder.id ? '' : id))
            setFolderDrop((cur) => (cur?.folderId === folder.id ? null : cur))
            if (springTimer.current?.folderId === folder.id) {
                clearSpring()
            }
        },
        onDrop: (e) => onRowDrop(e, folder),
    })

    return {dragOverId, draggingFolderId, droppedId, folderDrop, rowDrag}
}
