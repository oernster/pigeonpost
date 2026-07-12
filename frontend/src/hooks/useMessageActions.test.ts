// The instant replied/forwarded row update lives here: marking records the flag through the backend, then
// flips the answered/forwarded field in place across every list so the glyph shows on the row at once,
// without reopening the folder. It is best-effort, so a backend failure leaves the message untouched. ../api
// is mocked (the Wails seam), and the real message store is wired to the actions the way App does.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook} from '@testing-library/react'
import type {Message} from '../api'
import {useMessageStore} from './useMessageStore'
import {useMessageActions} from './useMessageActions'

const apiSpies = vi.hoisted(() => ({
    markReplied: vi.fn(),
    markForwarded: vi.fn(),
}))

vi.mock('../api', () => ({
    api: {
        markReplied: apiSpies.markReplied,
        markForwarded: apiSpies.markForwarded,
    },
}))

function makeMessage(overrides: Partial<Message> = {}): Message {
    return {
        id: 'm1', folderId: 'inbox', subject: 'Weekly report',
        fromName: 'Alice Example', fromAddress: 'alice@example.com',
        to: [{name: 'Me', address: 'me@example.com'}], cc: [],
        date: '2026-07-11T10:00:00.000Z', size: 1024, read: false, flagged: false,
        hasAttachments: false, answered: false, forwarded: false, snippet: 'A short snippet', tagColours: [],
        ...overrides,
    } as Message
}

// harness wires the real message store to the actions under test, the way App does, and exposes both so a
// test can seed the lists and read them back after an action.
function harness() {
    return renderHook(() => {
        const store = useMessageStore()
        const actions = useMessageActions({
            store,
            displayMessages: store.messages,
            searchActive: false,
            loadUnread: async () => {},
            setError: () => {},
        })
        return {store, actions}
    })
}

beforeEach(() => {
    apiSpies.markReplied.mockReset().mockResolvedValue(undefined)
    apiSpies.markForwarded.mockReset().mockResolvedValue(undefined)
})
afterEach(() => cleanup())

describe('useMessageActions: markReplied / markForwarded', () => {
    it('flips answered on the matching row across every list once the server records it', async () => {
        const {result} = harness()
        act(() => {
            result.current.store.setMessages([makeMessage({id: 'a'}), makeMessage({id: 'b'})])
            result.current.store.setSearchResults([makeMessage({id: 'a'})])
            result.current.store.setSelectedMessage(makeMessage({id: 'a'}))
        })
        await act(async () => {
            await result.current.actions.markReplied('a')
        })
        expect(apiSpies.markReplied).toHaveBeenCalledWith('a')
        expect(result.current.store.messages.find((m) => m.id === 'a')?.answered).toBe(true)
        expect(result.current.store.messages.find((m) => m.id === 'b')?.answered).toBe(false)
        expect(result.current.store.searchResults[0].answered).toBe(true)
        expect(result.current.store.selectedMessage?.answered).toBe(true)
    })

    it('flips forwarded on the matching row once a forward is recorded', async () => {
        const {result} = harness()
        act(() => {
            result.current.store.setMessages([makeMessage({id: 'a'})])
        })
        await act(async () => {
            await result.current.actions.markForwarded('a')
        })
        expect(apiSpies.markForwarded).toHaveBeenCalledWith('a')
        expect(result.current.store.messages[0].forwarded).toBe(true)
    })

    it('leaves the message unflagged when the backend mark fails', async () => {
        apiSpies.markReplied.mockRejectedValueOnce('offline')
        const {result} = harness()
        act(() => {
            result.current.store.setMessages([makeMessage({id: 'a'})])
        })
        await act(async () => {
            await result.current.actions.markReplied('a')
        })
        expect(result.current.store.messages[0].answered).toBe(false)
    })
})
