// The shared conversation loader behind the reader strip and the thread view. ../api is mocked (the
// Wails seam); the hook's job is to load a message's thread; above all it discards a load the surface
// has moved on from.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook, waitFor} from '@testing-library/react'
import type {ConversationEntry, Message} from '../api'
import {useConversation} from './useConversation'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({conversation: vi.fn()}))

// The mock is built from the real api rather than hand-listed here, so a method this hook reaches that
// no spy declares fails the test by name instead of throwing a TypeError into the nearest catch and
// passing. See src/test/apiMock.ts.
const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

function makeMessage(id: string): Message {
    return {id, folderId: 'f1', subject: 'Lunch'} as Message
}

function makeEntry(id: string): ConversationEntry {
    return {message: makeMessage(id), folderName: 'Inbox', folderKind: 'inbox'} as ConversationEntry
}

beforeEach(() => {
    apiSpies.conversation.mockReset().mockResolvedValue([])
})

afterEach(() => {
    cleanup()
    // Unmounting can reach the api too, so this is read after cleanup rather than before it.
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

describe('useConversation', () => {
    it('loads the thread of the open message', async () => {
        apiSpies.conversation.mockResolvedValue([makeEntry('m1'), makeEntry('m2')])
        const {result} = renderHook(() => useConversation('m1'))
        await waitFor(() => expect(result.current).toHaveLength(2))
        expect(apiSpies.conversation).toHaveBeenCalledWith('m1')
    })

    it('asks for nothing without a message', () => {
        const {result} = renderHook(() => useConversation(null))
        expect(apiSpies.conversation).not.toHaveBeenCalled()
        expect(result.current).toEqual([])
    })

    it('empties the entries when the surface closes', async () => {
        apiSpies.conversation.mockResolvedValue([makeEntry('m1'), makeEntry('m2')])
        const {result, rerender} = renderHook((id: string | null) => useConversation(id), {
            initialProps: 'm1' as string | null,
        })
        await waitFor(() => expect(result.current).toHaveLength(2))
        rerender(null)
        expect(result.current).toEqual([])
    })

    it('discards a load the surface has moved on from', async () => {
        // The first lookup resolves after the second message has opened. Its entries belong to a message
        // no longer on screen, so showing them would put one message's thread under another.
        let releaseFirst: (entries: ConversationEntry[]) => void = () => undefined
        apiSpies.conversation.mockImplementationOnce(
            () => new Promise<ConversationEntry[]>((resolve) => {
                releaseFirst = resolve
            }),
        )
        apiSpies.conversation.mockResolvedValueOnce([makeEntry('second')])
        const {result, rerender} = renderHook((id: string | null) => useConversation(id), {
            initialProps: 'm1' as string | null,
        })
        rerender('m2')
        await waitFor(() => expect(result.current).toHaveLength(1))
        await act(async () => {
            releaseFirst([makeEntry('stale'), makeEntry('stale2')])
        })
        expect(result.current).toHaveLength(1)
        expect(result.current[0].message.id).toBe('second')
    })

    it('shows nothing when the lookup fails', async () => {
        apiSpies.conversation.mockResolvedValue([makeEntry('m1'), makeEntry('m2')])
        const {result, rerender} = renderHook((id: string | null) => useConversation(id), {
            initialProps: 'm1' as string | null,
        })
        await waitFor(() => expect(result.current).toHaveLength(2))
        apiSpies.conversation.mockRejectedValueOnce(new Error('no'))
        rerender('m2')
        await waitFor(() => expect(result.current).toEqual([]))
    })
})

// The mock covers the api in both directions: the afterEach above catches a method reached with no
// spy; this catches the opposite, a spy declared under a name the api does not have, which binds to
// nothing, so every test configuring it would be configuring a stub the code can never call.
describe('the api mock', () => {
    it('declares no spy the real api does not have', async () => {
        const actual = await vi.importActual<typeof import('../api')>('../api')
        expect(spiesNotInApi(actual, apiSpies as unknown as Record<string, unknown>)).toEqual([])
    })
})
