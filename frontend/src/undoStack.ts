// undoStack holds the pure logic behind Edit > Undo and Redo: the entry types describing each
// undoable action, the capped stacks they live on, their menu labels and the id rebinding applied
// after an entry executes. No React, no api runtime, so it is unit-tested in isolation; the
// api calls that carry an entry out live in useUndoRedo.

// MoveFlavour tells a move-shaped entry which action created it: the flavour picks the redo verb
// (a delete is redone through the delete api so Trash is re-resolved, a junking through the junk
// api so the spam-verdict keywords are rewritten) and the label shown on the menu item.
export type MoveFlavour = 'move' | 'delete' | 'junk' | 'notJunk'

// MoveItem is one message inside a move-shaped entry: the id it currently carries (wherever it
// now sits) and the folder it originally came from, which is where an undo returns it.
export interface MoveItem {
    messageId: string
    sourceFolderId: string
}

// ToggleItem is one message inside a read or flag entry: the id (stable across the toggle) and
// the value the message held before the action, which is what an undo restores.
export interface ToggleItem {
    messageId: string
    before: boolean
}

// UndoEntry describes one undoable action. A move-shaped entry covers move, delete-to-Trash, junk
// and rescue (single or bulk: items sit in destFolderId, '' for a delete whose Trash varies by
// account); read and flag entries restore each message's prior value; a tag entry re-toggles one
// colour tag. The same entry shape sits on both stacks: the stack it is on decides the direction.
export type UndoEntry =
    | {kind: 'move'; flavour: MoveFlavour; items: MoveItem[]; destFolderId: string}
    | {kind: 'read'; items: ToggleItem[]; after: boolean}
    | {kind: 'flag'; items: ToggleItem[]; after: boolean}
    | {kind: 'tag'; messageId: string; tagId: string; assigned: boolean}

// UNDO_DEPTH caps each stack. A deeper history buys nothing (nobody unwinds twenty mail actions)
// while every entry pins message ids that grow staler with age.
export const UNDO_DEPTH = 20

// pushEntry appends an entry, dropping the oldest once the stack is at depth. It returns a new
// array so React state updates see a fresh reference.
export function pushEntry(stack: UndoEntry[], entry: UndoEntry): UndoEntry[] {
    const next = [...stack, entry]
    return next.length > UNDO_DEPTH ? next.slice(next.length - UNDO_DEPTH) : next
}

// actionName is the noun shared by undoLabel and redoLabel, matching the menu wording that
// created the entry.
function actionName(entry: UndoEntry): string {
    switch (entry.kind) {
        case 'move':
            switch (entry.flavour) {
                case 'delete': return 'delete'
                case 'junk': return 'mark as junk'
                case 'notJunk': return 'not junk'
                default: return 'move'
            }
        case 'read':
            return entry.after ? 'mark as read' : 'mark as unread'
        case 'flag':
            return entry.after ? 'add star' : 'remove star'
        default:
            return entry.assigned ? 'tag' : 'tag removal'
    }
}

// undoLabel and redoLabel name the top entry on the menu item, Thunderbird-style ("Undo delete"),
// so the user can see what the shortcut is about to unwind before pressing it.
export function undoLabel(entry: UndoEntry): string {
    return `Undo ${actionName(entry)}`
}

export function redoLabel(entry: UndoEntry): string {
    return `Redo ${actionName(entry)}`
}

// groupBySource buckets a move-shaped entry's items by the folder an undo returns them to, one
// bucket per batched moveMessages call, preserving first-seen order.
export function groupBySource(items: MoveItem[]): Array<{folderId: string; ids: string[]}> {
    const groups: Array<{folderId: string; ids: string[]}> = []
    const byFolder = new Map<string, {folderId: string; ids: string[]}>()
    for (const item of items) {
        let group = byFolder.get(item.sourceFolderId)
        if (!group) {
            group = {folderId: item.sourceFolderId, ids: []}
            byFolder.set(item.sourceFolderId, group)
            groups.push(group)
        }
        group.ids.push(item.messageId)
    }
    return groups
}

// rebindMoveItems returns the entry with each item's id replaced by where it landed, ready for
// the opposite stack. An item the server did not report is dropped (it can no longer be
// addressed); when nothing survives the entry is spent and null is returned.
export function rebindMoveItems(
    entry: Extract<UndoEntry, {kind: 'move'}>,
    landedIds: Record<string, string>,
): Extract<UndoEntry, {kind: 'move'}> | null {
    const items = entry.items
        .filter((item) => Boolean(landedIds[item.messageId]))
        .map((item) => ({...item, messageId: landedIds[item.messageId]}))
    return items.length > 0 ? {...entry, items} : null
}
