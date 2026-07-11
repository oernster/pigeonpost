// sidebarDnd holds the pure drag-and-drop resolution for the sidebar: the drag MIME types, the drop-zone
// geometry, and the reorder and reparent decisions for a folder drag. No React, no api runtime, so the
// logic is unit-tested in isolation.
import {Folder} from './api'
import {folderRank, nearestParentPath} from './folderPaths'

// accountDragType identifies an account row being dragged to reorder. It is distinct from the message
// drag type so a message dropped on an account row is ignored and vice versa.
export const accountDragType = 'application/x-pigeonpost-account'

// folderDragType identifies a custom folder row being dragged to reparent or reorder it. It is distinct
// from the account and message drag types so each drop target accepts only what it understands.
export const folderDragType = 'application/x-pigeonpost-folder'

// FOLDER_DROP_EDGE_FRACTION is the fraction of a folder row's height at its top and at its bottom that
// targets the folder's own level (a sibling drop, drawn as an insertion line); the middle band targets
// inside the folder (a child drop, drawn as a box). At 0.3 the top and bottom thirds are the sibling
// zones and the middle ~40% nests, so the same-level target is easy to hit.
const FOLDER_DROP_EDGE_FRACTION = 0.3

// FolderDropZone is where a folder drag is aimed on a target row: before or after places the dragged
// folder at the target's own level (a sibling of it); into nests the dragged folder inside the target.
export type FolderDropZone = 'before' | 'into' | 'after'

// dropZoneFor returns which zone the pointer at clientY falls in for a row with bounds rect.
export function dropZoneFor(clientY: number, rect: DOMRect): FolderDropZone {
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
export type FolderDropAction =
    | {kind: 'reorder'; parentPath: string; anchorPath: string; after: boolean}
    | {kind: 'reparent'; parentId: string; parentPath: string; anchorPath?: string; after?: boolean}

// resolveFolderDrop decides what dropping dragged onto target in the given zone should do. It returns
// null when the drop is not allowed (onto itself, into its own subtree or a move that changes nothing).
// An into drop nests dragged inside target. A before or after drop aims at target's own level: when that
// is dragged's current level it is a local reorder against target, otherwise it reparents dragged under
// target's parent (the top level when target is top-level). Only same-rank (custom) siblings reorder,
// since rank fixes the well-known mailboxes ahead of every custom folder.
export function resolveFolderDrop(
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
export function moveId(ids: string[], fromId: string, toId: string): string[] {
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
export function swapId(ids: string[], i: number, j: number): string[] {
    const next = [...ids]
    ;[next[i], next[j]] = [next[j], next[i]]
    return next
}
