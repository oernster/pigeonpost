import {describe, expect, it} from 'vitest'
import {Folder} from './api'
import {dropZoneFor, moveId, resolveFolderDrop, swapId} from './sidebarDnd'

const f = (id: string, path: string, kind = 'custom'): Folder => ({id, path, kind} as Folder)

const rect = (top: number, height: number): DOMRect => ({top, height} as DOMRect)

describe('dropZoneFor', () => {
    it('targets before in the top edge band', () => {
        expect(dropZoneFor(10, rect(0, 100))).toBe('before')
    })

    it('targets into in the middle band', () => {
        expect(dropZoneFor(50, rect(0, 100))).toBe('into')
    })

    it('targets after in the bottom edge band', () => {
        expect(dropZoneFor(90, rect(0, 100))).toBe('after')
    })
})

describe('moveId', () => {
    it('leaves the list unchanged when an id is missing', () => {
        expect(moveId(['a', 'b', 'c'], 'x', 'b')).toEqual(['a', 'b', 'c'])
        expect(moveId(['a', 'b', 'c'], 'a', 'x')).toEqual(['a', 'b', 'c'])
    })

    it('leaves the list unchanged when moving onto itself', () => {
        expect(moveId(['a', 'b', 'c'], 'b', 'b')).toEqual(['a', 'b', 'c'])
    })

    it('moves an id to the target index', () => {
        expect(moveId(['a', 'b', 'c'], 'a', 'c')).toEqual(['b', 'c', 'a'])
    })
})

describe('swapId', () => {
    it('exchanges two entries', () => {
        expect(swapId(['a', 'b', 'c'], 0, 2)).toEqual(['c', 'b', 'a'])
    })
})

describe('resolveFolderDrop', () => {
    const sep = '/'
    const existing = new Set(['Inbox', 'Work', 'Work/A', 'Work/B', 'Personal'])
    const byPath = new Map([['Inbox', 'i'], ['Work', 'w'], ['Work/A', 'wa'], ['Work/B', 'wb'], ['Personal', 'p']])
    const inbox = f('i', 'Inbox', 'inbox')
    const work = f('w', 'Work')
    const workA = f('wa', 'Work/A')
    const personal = f('p', 'Personal')

    it('rejects a drop onto itself', () => {
        expect(resolveFolderDrop(work, work, 'into', sep, existing, byPath)).toBeNull()
    })

    it('rejects nesting a folder into its own subtree', () => {
        expect(resolveFolderDrop(work, workA, 'into', sep, existing, byPath)).toBeNull()
    })

    it('rejects nesting a folder into its current parent', () => {
        expect(resolveFolderDrop(workA, work, 'into', sep, existing, byPath)).toBeNull()
    })

    it('nests a folder into an unrelated target', () => {
        expect(resolveFolderDrop(personal, work, 'into', sep, existing, byPath))
            .toEqual({kind: 'reparent', parentId: 'w', parentPath: 'Work'})
    })

    it('rejects a sibling drop whose target level is inside the dragged subtree', () => {
        expect(resolveFolderDrop(work, workA, 'before', sep, existing, byPath)).toBeNull()
    })

    it('rejects a same-level reorder across differing ranks', () => {
        expect(resolveFolderDrop(work, inbox, 'before', sep, existing, byPath)).toBeNull()
    })

    it('reorders same-rank siblings at the same level', () => {
        expect(resolveFolderDrop(work, personal, 'before', sep, existing, byPath))
            .toEqual({kind: 'reorder', parentPath: '', anchorPath: 'Personal', after: false})
    })

    it('reparents under the target parent when levels differ', () => {
        expect(resolveFolderDrop(personal, workA, 'after', sep, existing, byPath))
            .toEqual({kind: 'reparent', parentId: 'w', parentPath: 'Work', anchorPath: 'Work/A', after: true})
    })

    it('reparents to the top level when the target is top-level', () => {
        expect(resolveFolderDrop(workA, personal, 'after', sep, existing, byPath))
            .toEqual({kind: 'reparent', parentId: '', parentPath: '', anchorPath: 'Personal', after: true})
    })

    it('falls back to the top level when the target parent has no known id', () => {
        expect(resolveFolderDrop(personal, workA, 'after', sep, existing, new Map()))
            .toEqual({kind: 'reparent', parentId: '', parentPath: 'Work', anchorPath: 'Work/A', after: true})
    })
})
