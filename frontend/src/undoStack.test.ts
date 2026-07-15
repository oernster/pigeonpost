import {describe, expect, it} from 'vitest'
import {
    UNDO_DEPTH, UndoEntry, groupBySource, pushEntry, rebindMoveItems, redoLabel, undoLabel,
} from './undoStack'

function moveEntry(flavour: 'move' | 'delete' | 'junk' | 'notJunk'): Extract<UndoEntry, {kind: 'move'}> {
    return {kind: 'move', flavour, items: [{messageId: 'm1', sourceFolderId: 'f1'}], destFolderId: 'fd'}
}

describe('pushEntry', () => {
    it('appends to a fresh array', () => {
        const stack: UndoEntry[] = []
        const next = pushEntry(stack, moveEntry('move'))
        expect(next).toHaveLength(1)
        expect(stack).toHaveLength(0)
    })

    it('drops the oldest entry once the stack is at depth', () => {
        let stack: UndoEntry[] = []
        for (let i = 0; i < UNDO_DEPTH; i++) {
            stack = pushEntry(stack, {kind: 'tag', messageId: `m${i}`, tagId: 't', assigned: true})
        }
        const next = pushEntry(stack, moveEntry('delete'))
        expect(next).toHaveLength(UNDO_DEPTH)
        expect(next[0]).toEqual({kind: 'tag', messageId: 'm1', tagId: 't', assigned: true})
        expect(next[next.length - 1]).toEqual(moveEntry('delete'))
    })
})

describe('undoLabel and redoLabel', () => {
    it('names each move flavour', () => {
        expect(undoLabel(moveEntry('move'))).toBe('Undo move')
        expect(undoLabel(moveEntry('delete'))).toBe('Undo delete')
        expect(undoLabel(moveEntry('junk'))).toBe('Undo mark as junk')
        expect(undoLabel(moveEntry('notJunk'))).toBe('Undo not junk')
        expect(redoLabel(moveEntry('delete'))).toBe('Redo delete')
    })

    it('names read and flag toggles by the direction the action set', () => {
        expect(undoLabel({kind: 'read', items: [{messageId: 'm1', before: false}], after: true})).toBe('Undo mark as read')
        expect(undoLabel({kind: 'read', items: [{messageId: 'm1', before: true}], after: false})).toBe('Undo mark as unread')
        expect(undoLabel({kind: 'flag', items: [{messageId: 'm1', before: false}], after: true})).toBe('Undo add star')
        expect(redoLabel({kind: 'flag', items: [{messageId: 'm1', before: true}], after: false})).toBe('Redo remove star')
    })

    it('names tagging and tag removal', () => {
        expect(undoLabel({kind: 'tag', messageId: 'm1', tagId: 't', assigned: true})).toBe('Undo tag')
        expect(undoLabel({kind: 'tag', messageId: 'm1', tagId: 't', assigned: false})).toBe('Undo tag removal')
    })
})

describe('groupBySource', () => {
    it('buckets items by source folder preserving first-seen order', () => {
        const groups = groupBySource([
            {messageId: 'm1', sourceFolderId: 'f1'},
            {messageId: 'm2', sourceFolderId: 'f2'},
            {messageId: 'm3', sourceFolderId: 'f1'},
        ])
        expect(groups).toEqual([
            {folderId: 'f1', ids: ['m1', 'm3']},
            {folderId: 'f2', ids: ['m2']},
        ])
    })

    it('returns no groups for no items', () => {
        expect(groupBySource([])).toEqual([])
    })
})

describe('rebindMoveItems', () => {
    it('replaces each id with where it landed', () => {
        const entry = {
            kind: 'move' as const,
            flavour: 'move' as const,
            items: [
                {messageId: 'm1', sourceFolderId: 'f1'},
                {messageId: 'm2', sourceFolderId: 'f2'},
            ],
            destFolderId: 'fd',
        }
        const rebound = rebindMoveItems(entry, {m1: 'n1', m2: 'n2'})
        expect(rebound?.items).toEqual([
            {messageId: 'n1', sourceFolderId: 'f1'},
            {messageId: 'n2', sourceFolderId: 'f2'},
        ])
        expect(rebound?.destFolderId).toBe('fd')
    })

    it('drops items the server did not report', () => {
        const rebound = rebindMoveItems(moveEntry('move'), {})
        expect(rebound).toBeNull()
    })

    it('keeps the survivors when only some ids are reported', () => {
        const entry = {
            kind: 'move' as const,
            flavour: 'delete' as const,
            items: [
                {messageId: 'm1', sourceFolderId: 'f1'},
                {messageId: 'm2', sourceFolderId: 'f1'},
            ],
            destFolderId: '',
        }
        const rebound = rebindMoveItems(entry, {m2: 'n2'})
        expect(rebound?.items).toEqual([{messageId: 'n2', sourceFolderId: 'f1'}])
    })
})
