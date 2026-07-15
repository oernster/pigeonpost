// useUndoRedo drives Edit > Undo / Redo: entries recorded by the action hooks execute back through
// the api (moves return, deletes re-delete, toggles restore), rebinding to the opposite stack with
// the ids the server reports so the pair can ping-pong. ../api is mocked (the Wails seam) and the
// real message store is wired in the way App does.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook} from '@testing-library/react'
import type {Message} from '../api'
import {useMessageStore} from './useMessageStore'
import {useUndoRedo} from './useUndoRedo'

const apiSpies = vi.hoisted(() => ({
    moveMessages: vi.fn(),
    deleteMessages: vi.fn(),
    markJunk: vi.fn(),
    markNotJunk: vi.fn(),
    markRead: vi.fn(),
    markFlagged: vi.fn(),
    syncFolder: vi.fn(),
}))

vi.mock('../api', () => ({api: apiSpies}))

function makeMessage(overrides: Partial<Message> = {}): Message {
    return {
        id: 'm1', folderId: 'trash', subject: 'S', fromName: '', fromAddress: 'a@b.c',
        to: [], cc: [], date: '2026-07-15T10:00:00.000Z', size: 1, read: true, flagged: false,
        hasAttachments: false, answered: false, forwarded: false, snippet: '', tagColours: [],
        ...overrides,
    } as Message
}

const errors: string[] = []

function harness() {
    return renderHook(() => {
        const store = useMessageStore()
        const undoRedo = useUndoRedo({
            store,
            loadUnread: async () => {},
            refreshFolders: async () => {},
            setError: (message: string) => errors.push(message),
        })
        return {store, undoRedo}
    })
}

beforeEach(() => {
    errors.length = 0
    apiSpies.moveMessages.mockReset().mockResolvedValue({ids: [], failed: 0, error: '', newIds: {}})
    apiSpies.deleteMessages.mockReset().mockResolvedValue({ids: [], failed: 0, error: '', newIds: {}})
    apiSpies.markJunk.mockReset().mockResolvedValue({newId: ''})
    apiSpies.markNotJunk.mockReset().mockResolvedValue({newId: ''})
    apiSpies.markRead.mockReset().mockResolvedValue(undefined)
    apiSpies.markFlagged.mockReset().mockResolvedValue(undefined)
    apiSpies.syncFolder.mockReset().mockResolvedValue(undefined)
})
afterEach(() => cleanup())

describe('useUndoRedo: move-shaped entries', () => {
    it('undoes a delete by moving the trash copy back, then offers the redo', async () => {
        const {result} = harness()
        act(() => {
            result.current.store.setMessages([makeMessage({id: 't1', folderId: 'trash'})])
            result.current.undoRedo.recorder.push({
                kind: 'move', flavour: 'delete',
                items: [{messageId: 't1', sourceFolderId: 'inbox'}], destFolderId: '',
            })
        })
        expect(result.current.undoRedo.undoText).toBe('Undo delete')
        apiSpies.moveMessages.mockResolvedValueOnce({ids: ['t1'], failed: 0, error: '', newIds: {t1: 'i9'}})

        await act(async () => {
            await result.current.undoRedo.undo()
        })
        expect(apiSpies.moveMessages).toHaveBeenCalledWith(['t1'], 'inbox')
        expect(apiSpies.syncFolder).toHaveBeenCalledWith('inbox')
        // The trash row leaves the on-screen lists at once.
        expect(result.current.store.messages).toHaveLength(0)
        expect(result.current.undoRedo.undoText).toBeNull()
        expect(result.current.undoRedo.redoText).toBe('Redo delete')

        // Redo re-deletes through the delete api, so Trash is re-resolved server-side.
        apiSpies.deleteMessages.mockResolvedValueOnce({ids: ['i9'], failed: 0, error: '', newIds: {i9: 't2'}})
        await act(async () => {
            await result.current.undoRedo.redo()
        })
        expect(apiSpies.deleteMessages).toHaveBeenCalledWith(['i9'])
        expect(result.current.undoRedo.redoText).toBeNull()
        expect(result.current.undoRedo.undoText).toBe('Undo delete')
    })

    it('groups a bulk undo by source folder', async () => {
        const {result} = harness()
        act(() => {
            result.current.undoRedo.recorder.push({
                kind: 'move', flavour: 'move',
                items: [
                    {messageId: 'a', sourceFolderId: 'f1'},
                    {messageId: 'b', sourceFolderId: 'f2'},
                    {messageId: 'c', sourceFolderId: 'f1'},
                ],
                destFolderId: 'fd',
            })
        })
        await act(async () => {
            await result.current.undoRedo.undo()
        })
        expect(apiSpies.moveMessages.mock.calls).toEqual([
            [['a', 'c'], 'f1'],
            [['b'], 'f2'],
        ])
    })

    it('spends an entry the server cannot locate again', async () => {
        const {result} = harness()
        act(() => {
            result.current.undoRedo.recorder.push({
                kind: 'move', flavour: 'move',
                items: [{messageId: 'a', sourceFolderId: 'f1'}], destFolderId: 'fd',
            })
        })
        apiSpies.moveMessages.mockResolvedValueOnce({ids: ['a'], failed: 0, error: '', newIds: {}})
        await act(async () => {
            await result.current.undoRedo.undo()
        })
        // Undone, but with no reported landing id there is nothing a redo could address.
        expect(result.current.undoRedo.undoText).toBeNull()
        expect(result.current.undoRedo.redoText).toBeNull()
    })

    it('drops a failed entry and reports through the error sink', async () => {
        const {result} = harness()
        act(() => {
            result.current.undoRedo.recorder.push({
                kind: 'move', flavour: 'move',
                items: [{messageId: 'a', sourceFolderId: 'f1'}], destFolderId: 'fd',
            })
        })
        apiSpies.moveMessages.mockRejectedValueOnce('gone')
        await act(async () => {
            await result.current.undoRedo.undo()
        })
        expect(errors).toContain('gone')
        expect(result.current.undoRedo.undoText).toBeNull()
        expect(result.current.undoRedo.redoText).toBeNull()
    })

    it('redoes a junking through the junk api so the verdict keywords are rewritten', async () => {
        const {result} = harness()
        act(() => {
            result.current.undoRedo.recorder.push({
                kind: 'move', flavour: 'junk',
                items: [{messageId: 'j1', sourceFolderId: 'inbox'}], destFolderId: 'fj',
            })
        })
        apiSpies.moveMessages.mockResolvedValueOnce({ids: ['j1'], failed: 0, error: '', newIds: {j1: 'i2'}})
        await act(async () => {
            await result.current.undoRedo.undo()
        })
        apiSpies.markJunk.mockResolvedValueOnce({newId: 'j3'})
        await act(async () => {
            await result.current.undoRedo.redo()
        })
        expect(apiSpies.markJunk).toHaveBeenCalledWith('i2')
        expect(result.current.undoRedo.undoText).toBe('Undo mark as junk')
    })
})

describe('useUndoRedo: toggles and tags', () => {
    it('restores each message’s prior read state on undo and re-applies it on redo', async () => {
        const {result} = harness()
        act(() => {
            result.current.store.setMessages([
                makeMessage({id: 'a', folderId: 'f1', read: true}),
                makeMessage({id: 'b', folderId: 'f1', read: true}),
            ])
            result.current.undoRedo.recorder.push({
                kind: 'read',
                items: [{messageId: 'a', before: false}, {messageId: 'b', before: true}],
                after: true,
            })
        })
        await act(async () => {
            await result.current.undoRedo.undo()
        })
        expect(apiSpies.markRead.mock.calls).toEqual([['a', false], ['b', true]])
        expect(result.current.store.messages.find((m) => m.id === 'a')?.read).toBe(false)
        expect(result.current.undoRedo.redoText).toBe('Redo mark as read')

        await act(async () => {
            await result.current.undoRedo.redo()
        })
        expect(apiSpies.markRead.mock.calls.slice(2)).toEqual([['a', true], ['b', true]])
        expect(result.current.store.messages.find((m) => m.id === 'a')?.read).toBe(true)
    })

    it('runs a tag entry through the registered executor with the direction applied', async () => {
        const executor = vi.fn().mockResolvedValue(undefined)
        const {result} = harness()
        act(() => {
            result.current.undoRedo.recorder.registerTagExecutor(executor)
            result.current.undoRedo.recorder.push({kind: 'tag', messageId: 'a', tagId: 't1', assigned: true})
        })
        await act(async () => {
            await result.current.undoRedo.undo()
        })
        expect(executor).toHaveBeenCalledWith('a', 't1', false)
        await act(async () => {
            await result.current.undoRedo.redo()
        })
        expect(executor).toHaveBeenCalledWith('a', 't1', true)
    })

    it('a fresh action clears the redo history', async () => {
        const {result} = harness()
        act(() => {
            result.current.undoRedo.recorder.push({kind: 'flag', items: [{messageId: 'a', before: false}], after: true})
        })
        await act(async () => {
            await result.current.undoRedo.undo()
        })
        expect(result.current.undoRedo.redoText).toBe('Redo add star')
        act(() => {
            result.current.undoRedo.recorder.push({kind: 'flag', items: [{messageId: 'b', before: true}], after: false})
        })
        expect(result.current.undoRedo.redoText).toBeNull()
    })
})
