// The reader's conversation loader. ../api is mocked (the Wails seam); the hook's job is to load the
// open message's thread; above all it discards a load the reader has moved on from.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook, waitFor} from '@testing-library/react'
import type {ConversationEntry, Message} from '../api'
import {useConversation} from './useConversation'

const apiSpies = vi.hoisted(() => ({conversation: vi.fn()}))
vi.mock('../api', () => ({api: apiSpies}))

function makeMessage(id: string): Message {
    return {id, folderId: 'f1', subject: 'Lunch'} as Message
}

function makeEntry(id: string): ConversationEntry {
    return {message: makeMessage(id), folderName: 'Inbox', folderKind: 'inbox'} as ConversationEntry
}

beforeEach(() => {
    apiSpies.conversation.mockReset().mockResolvedValue([])
})

afterEach(() => cleanup())

describe('useConversation', () => {
    it('loads the thread of the open message', async () => {
        apiSpies.conversation.mockResolvedValue([makeEntry('m1'), makeEntry('m2')])
        const {result} = renderHook(() => useConversation(makeMessage('m1')))
        await waitFor(() => expect(result.current).toHaveLength(2))
        expect(apiSpies.conversation).toHaveBeenCalledWith('m1')
    })

    it('asks for nothing while the reader is closed', () => {
        const {result} = renderHook(() => useConversation(null))
        expect(apiSpies.conversation).not.toHaveBeenCalled()
        expect(result.current).toEqual([])
    })

    it('empties the strip when the reader closes', async () => {
        apiSpies.conversation.mockResolvedValue([makeEntry('m1'), makeEntry('m2')])
        const {result, rerender} = renderHook((message: Message | null) => useConversation(message), {
            initialProps: makeMessage('m1') as Message | null,
        })
        await waitFor(() => expect(result.current).toHaveLength(2))
        rerender(null)
        expect(result.current).toEqual([])
    })

    it('discards a load the reader has moved on from', async () => {
        // The first lookup resolves after the second message has opened. Its entries belong to a message
        // no longer on screen, so showing them would put one message's thread under another.
        let releaseFirst: (entries: ConversationEntry[]) => void = () => undefined
        apiSpies.conversation.mockImplementationOnce(
            () => new Promise<ConversationEntry[]>((resolve) => {
                releaseFirst = resolve
            }),
        )
        apiSpies.conversation.mockResolvedValueOnce([makeEntry('second')])
        const {result, rerender} = renderHook((message: Message | null) => useConversation(message), {
            initialProps: makeMessage('m1') as Message | null,
        })
        rerender(makeMessage('m2'))
        await waitFor(() => expect(result.current).toHaveLength(1))
        await act(async () => {
            releaseFirst([makeEntry('stale'), makeEntry('stale2')])
        })
        expect(result.current).toHaveLength(1)
        expect(result.current[0].message.id).toBe('second')
    })

    it('shows an empty strip when the lookup fails', async () => {
        apiSpies.conversation.mockResolvedValue([makeEntry('m1'), makeEntry('m2')])
        const {result, rerender} = renderHook((message: Message | null) => useConversation(message), {
            initialProps: makeMessage('m1') as Message | null,
        })
        await waitFor(() => expect(result.current).toHaveLength(2))
        apiSpies.conversation.mockRejectedValueOnce(new Error('no'))
        rerender(makeMessage('m2'))
        await waitFor(() => expect(result.current).toEqual([]))
    })
})
