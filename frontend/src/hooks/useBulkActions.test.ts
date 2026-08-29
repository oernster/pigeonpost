// The drag-and-drop move onto a folder. The point under test is the timing: an IMAP move can take seconds
// on a slow provider, so the dropped rows leave the lists at once rather than when the server answers, a
// refused or partial move puts the untouched rows back where they sat; a second drop of a message whose
// move is still open is ignored rather than issued twice. ../api is mocked (the Wails seam) and the real
// message store and selection are wired in the way App does.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook} from '@testing-library/react'
import type {Folder, Message} from '../api'
import {useMessageStore} from './useMessageStore'
import {useSelection} from './useSelection'
import {useBulkActions} from './useBulkActions'

const apiSpies = vi.hoisted(() => ({
    moveMessages: vi.fn(),
    syncFolder: vi.fn(),
}))

vi.mock('../api', () => ({api: apiSpies}))

function makeMessage(id: string, folderId: string): Message {
    return {
        id, folderId, accountId: '', snoozedUntilMs: 0, subject: id, fromName: '', fromAddress: 'a@b.c',
        to: [], cc: [], date: '2026-08-07T10:00:00.000Z', size: 1, read: true, flagged: false,
        hasAttachments: false, answered: false, forwarded: false, snippet: '', tagColours: [],
    } as Message
}

const folders = [
    {id: 'inbox', accountId: 'a1'},
    {id: 'work', accountId: 'a1'},
] as Folder[]

const undoSpies = {push: vi.fn(), registerTagExecutor: vi.fn()}
// Every action clears the error banner first, so only the non-empty entries are the reported failures.
const errors: string[] = []
const reported = () => errors.filter((e) => e !== '')

// harness wires the hook to a real store seeded with three inbox messages, the way App does.
function harness() {
    const rendered = renderHook(() => {
        const store = useMessageStore()
        const selection = useSelection()
        const bulk = useBulkActions({
            store,
            selection,
            folders,
            // The account under test is IMAP, so its delete confirmation says Trash rather than gone.
            isPop3: false,
            loadUnread: async () => {},
            refreshFolders: async () => {},
            setError: (message: string) => errors.push(message),
            undo: undoSpies,
        })
        return {store, selection, bulk}
    })
    act(() => {
        rendered.result.current.store.setMessages(
            ['m1', 'm2', 'm3'].map((id) => makeMessage(id, 'inbox')),
        )
    })
    return rendered
}

const listedIds = (result: {current: {store: {messages: Message[]}}}) =>
    result.current.store.messages.map((m) => m.id)

// deferred hands back a promise the test settles by hand, so the window between the drop and the
// server's answer can be inspected.
function deferred() {
    let settle: (value: unknown) => void = () => {}
    const promise = new Promise((resolve) => {
        settle = resolve
    })
    return {promise, settle}
}

beforeEach(() => {
    errors.length = 0
    undoSpies.push.mockReset()
    apiSpies.moveMessages.mockReset().mockResolvedValue({ids: [], failed: 0, error: '', newIds: {}})
    apiSpies.syncFolder.mockReset().mockResolvedValue(undefined)
})
afterEach(() => cleanup())

describe('useBulkActions: dropping a message on a folder', () => {
    it('takes the row out of the list at once, before the server answers', () => {
        const {result} = harness()
        const {promise} = deferred()
        apiSpies.moveMessages.mockReturnValueOnce(promise)

        act(() => result.current.bulk.dropMessageOnFolder('m2', 'work'))

        expect(listedIds(result)).toEqual(['m1', 'm3'])
        expect(apiSpies.moveMessages).toHaveBeenCalledWith(['m2'], 'work')
    })

    it('ignores a second drop of the same message while the first move is still open', () => {
        const {result} = harness()
        const {promise} = deferred()
        apiSpies.moveMessages.mockReturnValueOnce(promise)

        act(() => result.current.bulk.dropMessageOnFolder('m2', 'work'))
        act(() => result.current.bulk.dropMessageOnFolder('m2', 'work'))

        expect(apiSpies.moveMessages).toHaveBeenCalledTimes(1)
    })

    it('puts the row back where it sat when the move fails', async () => {
        const {result} = harness()
        apiSpies.moveMessages.mockRejectedValueOnce('offline')

        await act(async () => {
            result.current.bulk.dropMessageOnFolder('m2', 'work')
            await Promise.resolve()
        })

        expect(listedIds(result)).toEqual(['m1', 'm2', 'm3'])
        expect(reported()[0]).toContain('Move failed')
    })

    it('puts back only the rows the server did not move', async () => {
        const {result} = harness()
        act(() => result.current.selection.setMarkedIds(new Set(['m1', 'm2'])))
        apiSpies.moveMessages.mockResolvedValueOnce({
            ids: ['m1'], failed: 1, error: 'mailbox full', newIds: {m1: 'n1'},
        })

        await act(async () => {
            await result.current.bulk.bulkMove(
                [makeMessage('m1', 'inbox'), makeMessage('m2', 'inbox')], 'work',
            )
        })

        expect(listedIds(result)).toEqual(['m2', 'm3'])
        expect(reported()[0]).toContain('could not be moved')
    })

    it('keeps the move once the server confirms it', async () => {
        const {result} = harness()
        apiSpies.moveMessages.mockResolvedValueOnce({
            ids: ['m2'], failed: 0, error: '', newIds: {m2: 'n2'},
        })

        await act(async () => {
            await result.current.bulk.bulkMove([makeMessage('m2', 'inbox')], 'work')
        })

        expect(listedIds(result)).toEqual(['m1', 'm3'])
        expect(apiSpies.syncFolder).toHaveBeenCalledWith('work')
        expect(reported()).toEqual([])
    })

    it('allows a fresh drop once a failed move has released the message', async () => {
        const {result} = harness()
        apiSpies.moveMessages.mockRejectedValueOnce('offline')
        await act(async () => {
            result.current.bulk.dropMessageOnFolder('m2', 'work')
            await Promise.resolve()
        })

        apiSpies.moveMessages.mockReturnValueOnce(deferred().promise)
        act(() => result.current.bulk.dropMessageOnFolder('m2', 'work'))

        expect(apiSpies.moveMessages).toHaveBeenCalledTimes(2)
        expect(listedIds(result)).toEqual(['m1', 'm3'])
    })
})
