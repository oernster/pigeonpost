// The message clipboard behind Edit > Cut / Copy / Paste at the message level: cut or copy takes
// messages onto an internal clipboard and paste files them into a folder. A pasted cut is
// optimistic: its rows join the open folder's list immediately, the server move settles behind
// them (rows re-pointed at their new ids, refused rows rolled back) and one undo entry records the
// paste. A pasted copy duplicates through the api and stays on the clipboard; each duplicate's row
// joins the open folder as soon as the server reports the id it carries. ../api is mocked (the
// Wails seam) and the real message store is wired in the way App does.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook} from '@testing-library/react'
import type {Message} from '../api'
import {useMessageStore} from './useMessageStore'
import {useMessageClipboard} from './useMessageClipboard'

const apiSpies = vi.hoisted(() => ({
    moveMessages: vi.fn(),
    copyMessage: vi.fn(),
    syncFolder: vi.fn(),
}))

vi.mock('../api', () => ({api: apiSpies}))

function makeMessage(id: string, folderId: string): Message {
    return {
        id, folderId, accountId: '', snoozedUntilMs: 0, subject: 'S', fromName: '', fromAddress: 'a@b.c',
        to: [], cc: [], date: '2026-07-16T10:00:00.000Z', size: 1, read: true, flagged: false,
        hasAttachments: false, answered: false, forwarded: false, snippet: '', tagColours: [],
    } as Message
}

const undoSpies = {
    push: vi.fn(),
    registerTagExecutor: vi.fn(),
}
const errors: string[] = []

function harness() {
    return renderHook(() => {
        const store = useMessageStore()
        const clipboard = useMessageClipboard({
            store,
            selectedFolderId: 'fd',
            undo: undoSpies,
            loadUnread: async () => {},
            refreshFolders: async () => {},
            setError: (message: string) => errors.push(message),
        })
        return {store, clipboard}
    })
}

beforeEach(() => {
    errors.length = 0
    undoSpies.push.mockReset()
    apiSpies.moveMessages.mockReset().mockResolvedValue({ids: [], failed: 0, error: '', newIds: {}})
    apiSpies.copyMessage.mockReset().mockResolvedValue({newId: ''})
    apiSpies.syncFolder.mockReset().mockResolvedValue(undefined)
})
afterEach(() => cleanup())

describe('useMessageClipboard: pasting a cut', () => {
    it('shows the rows in the open folder immediately, before the server settles', async () => {
        const {result} = harness()
        // The server never settles within this test: the paste must not depend on it.
        let settle: (v: {ids: string[]; failed: number; error: string; newIds: Record<string, string>}) => void = () => {}
        apiSpies.moveMessages.mockReturnValueOnce(new Promise((resolve) => { settle = resolve }))
        act(() => result.current.clipboard.cutMessages([makeMessage('a', 'f1')]))

        let pending: Promise<void> = Promise.resolve()
        act(() => {
            pending = result.current.clipboard.pasteInto('fd')
        })
        // Optimistic: the row is already in the list, re-homed to the destination, and the cut is
        // consumed so Paste greys out rather than offering a second move of the same message.
        expect(result.current.store.messages.map((m) => ({id: m.id, folderId: m.folderId})))
            .toEqual([{id: 'a', folderId: 'fd'}])
        expect(result.current.clipboard.hasClip).toBe(false)

        settle({ids: ['a'], failed: 0, error: '', newIds: {a: 'n1'}})
        await act(async () => {
            await pending
        })
        expect(result.current.store.messages[0].id).toBe('n1')
    })

    it('records one undo entry addressing the rows where they landed', async () => {
        const {result} = harness()
        apiSpies.moveMessages.mockResolvedValueOnce({
            ids: ['a', 'b'], failed: 0, error: '', newIds: {a: 'n1', b: 'n2'},
        })
        act(() => result.current.clipboard.cutMessages([makeMessage('a', 'f1'), makeMessage('b', 'f2')]))
        await act(async () => {
            await result.current.clipboard.pasteInto('fd')
        })
        expect(apiSpies.moveMessages).toHaveBeenCalledWith(['a', 'b'], 'fd')
        expect(apiSpies.syncFolder).toHaveBeenCalledWith('fd')
        expect(undoSpies.push).toHaveBeenCalledWith({
            kind: 'move', flavour: 'move',
            items: [
                {messageId: 'n1', sourceFolderId: 'f1'},
                {messageId: 'n2', sourceFolderId: 'f2'},
            ],
            destFolderId: 'fd',
        })
    })

    it('rolls back the rows the server refused and reports the error', async () => {
        const {result} = harness()
        apiSpies.moveMessages.mockResolvedValueOnce({
            ids: ['a'], failed: 1, error: 'boom', newIds: {a: 'n1'},
        })
        act(() => result.current.clipboard.cutMessages([makeMessage('a', 'f1'), makeMessage('b', 'f1')]))
        await act(async () => {
            await result.current.clipboard.pasteInto('fd')
        })
        expect(result.current.store.messages.map((m) => m.id)).toEqual(['n1'])
        expect(errors.some((e) => e.includes('boom'))).toBe(true)
    })

    it('rolls everything back and restores the clipboard when the whole move fails', async () => {
        const {result} = harness()
        apiSpies.moveMessages.mockRejectedValueOnce('offline')
        act(() => result.current.clipboard.cutMessages([makeMessage('a', 'f1')]))
        await act(async () => {
            await result.current.clipboard.pasteInto('fd')
        })
        expect(result.current.store.messages).toHaveLength(0)
        expect(result.current.clipboard.hasClip).toBe(true)
        expect(errors.some((e) => e.includes('offline'))).toBe(true)
    })

    it('does not show optimistic rows when pasting onto a folder that is not being viewed', async () => {
        const {result} = harness()
        apiSpies.moveMessages.mockResolvedValueOnce({ids: ['a'], failed: 0, error: '', newIds: {a: 'n1'}})
        act(() => result.current.clipboard.cutMessages([makeMessage('a', 'f1')]))
        await act(async () => {
            // The harness views 'fd'; this paste targets another folder via its context menu.
            await result.current.clipboard.pasteInto('felsewhere')
        })
        expect(result.current.store.messages).toHaveLength(0)
        expect(apiSpies.moveMessages).toHaveBeenCalledWith(['a'], 'felsewhere')
        expect(undoSpies.push).toHaveBeenCalled()
    })

    it('does not duplicate a row already in the open folder', async () => {
        const {result} = harness()
        apiSpies.moveMessages.mockResolvedValueOnce({ids: [], failed: 0, error: '', newIds: {}})
        act(() => {
            result.current.store.setMessages([makeMessage('a', 'fd')])
            result.current.clipboard.cutMessages([makeMessage('a', 'fd')])
        })
        await act(async () => {
            await result.current.clipboard.pasteInto('fd')
        })
        expect(result.current.store.messages.map((m) => m.id)).toEqual(['a'])
    })
})

describe('useMessageClipboard: copies and gating', () => {
    it('starts empty and ignores a cut or copy of nothing', () => {
        const {result} = harness()
        expect(result.current.clipboard.hasClip).toBe(false)
        act(() => result.current.clipboard.cutMessages([]))
        expect(result.current.clipboard.hasClip).toBe(false)
    })

    it('pastes a copy as duplicates and keeps the clipboard for further pastes', async () => {
        const {result} = harness()
        act(() => result.current.clipboard.copyMessages([makeMessage('a', 'f1'), makeMessage('b', 'f1')]))
        await act(async () => {
            await result.current.clipboard.pasteInto('fd')
        })
        expect(apiSpies.copyMessage.mock.calls).toEqual([['a', 'fd'], ['b', 'fd']])
        expect(apiSpies.syncFolder).toHaveBeenCalledWith('fd')
        expect(apiSpies.moveMessages).not.toHaveBeenCalled()
        expect(result.current.clipboard.hasClip).toBe(true)
    })

    it('shows a pasted copy in the open folder as soon as the server reports where it landed', async () => {
        const {result} = harness()
        apiSpies.copyMessage
            .mockResolvedValueOnce({newId: 'c1'})
            .mockResolvedValueOnce({newId: ''}) // no COPYUID: this copy waits for the sync
        act(() => {
            result.current.store.setMessages([makeMessage('a', 'fd')])
            result.current.clipboard.copyMessages([makeMessage('a', 'fd'), makeMessage('b', 'f1')])
        })
        await act(async () => {
            await result.current.clipboard.pasteInto('fd')
        })
        // The reported duplicate is listed beside its original under its real id; the unreported
        // one is not shown early (it has no identity to show under).
        expect(result.current.store.messages.map((m) => ({id: m.id, folderId: m.folderId})))
            .toEqual([{id: 'a', folderId: 'fd'}, {id: 'c1', folderId: 'fd'}])
        expect(result.current.clipboard.hasClip).toBe(true)
    })

    it('does not show a pasted copy early when pasting onto a folder that is not being viewed', async () => {
        const {result} = harness()
        apiSpies.copyMessage.mockResolvedValue({newId: 'c1'})
        act(() => result.current.clipboard.copyMessages([makeMessage('a', 'f1')]))
        await act(async () => {
            await result.current.clipboard.pasteInto('felsewhere')
        })
        expect(result.current.store.messages).toHaveLength(0)
        expect(apiSpies.copyMessage).toHaveBeenCalledWith('a', 'felsewhere')
    })

    it('reports how many copies could not be pasted', async () => {
        apiSpies.copyMessage.mockRejectedValueOnce('gone')
        const {result} = harness()
        act(() => result.current.clipboard.copyMessages([makeMessage('a', 'f1'), makeMessage('b', 'f1')]))
        await act(async () => {
            await result.current.clipboard.pasteInto('fd')
        })
        expect(errors).toContain('1 of 2 messages could not be pasted.')
    })

    it('pasting with an empty clipboard or no target folder is a no-op', async () => {
        const {result} = harness()
        await act(async () => {
            await result.current.clipboard.pasteInto('fd')
        })
        act(() => result.current.clipboard.cutMessages([makeMessage('a', 'f1')]))
        await act(async () => {
            await result.current.clipboard.pasteInto('')
        })
        expect(apiSpies.moveMessages).not.toHaveBeenCalled()
        expect(result.current.clipboard.hasClip).toBe(true)
    })

    it('reports the cut rows so the list can dim them, empty for a copy or once pasted', async () => {
        const {result} = harness()
        act(() => result.current.clipboard.cutMessages([makeMessage('a', 'f1'), makeMessage('b', 'f1')]))
        expect([...result.current.clipboard.cutIds].sort()).toEqual(['a', 'b'])
        // A copy replaces the cut: nothing is pending departure any more.
        act(() => result.current.clipboard.copyMessages([makeMessage('c', 'f1')]))
        expect(result.current.clipboard.cutIds.size).toBe(0)
        // A pasted cut is consumed, so its dim clears with it.
        act(() => result.current.clipboard.cutMessages([makeMessage('a', 'f1')]))
        await act(async () => {
            await result.current.clipboard.pasteInto('fd')
        })
        expect(result.current.clipboard.cutIds.size).toBe(0)
    })

    it('a fresh cut replaces whatever was on the clipboard', async () => {
        const {result} = harness()
        apiSpies.moveMessages.mockResolvedValueOnce({ids: ['b'], failed: 0, error: '', newIds: {}})
        act(() => result.current.clipboard.copyMessages([makeMessage('a', 'f1')]))
        act(() => result.current.clipboard.cutMessages([makeMessage('b', 'f1')]))
        await act(async () => {
            await result.current.clipboard.pasteInto('fd')
        })
        expect(apiSpies.moveMessages).toHaveBeenCalledWith(['b'], 'fd')
    })
})
