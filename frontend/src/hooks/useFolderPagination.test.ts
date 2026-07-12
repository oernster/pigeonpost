// useFolderPagination owns the keyset cursor for the flat folder view. These tests drive it directly (the
// api seam is stubbed, as in App.test) to pin the cursor bookkeeping the app relies on: the first page
// records the cursor, the next page walks it in the same direction, already-loaded ids are filtered,
// paging stops when no more remains, overlapping loads are guarded, and reset clears the cursor.
import {act, renderHook} from '@testing-library/react'
import {beforeEach, describe, expect, it, vi} from 'vitest'
import type {Message, MessagePage} from '../api'
import {useFolderPagination} from './useFolderPagination'

const listMessagesPage = vi.hoisted(() => vi.fn())
// MESSAGE_PAGE_SIZE is a real runtime export of ../api, so the mock supplies it too; the stub asserts it is
// forwarded as the page limit.
vi.mock('../api', () => ({
    api: {listMessagesPage},
    MESSAGE_PAGE_SIZE: 200,
}))

// The hook reads only a message's id, so a bare id cast to Message is enough to exercise it.
function msg(id: string): Message {
    return {id} as unknown as Message
}

function page(messages: Message[], hasMore: boolean, dateMs = 0, id = ''): MessagePage {
    return {messages, hasMore, nextCursorDateMs: dateMs, nextCursorId: id}
}

beforeEach(() => {
    listMessagesPage.mockReset()
})

describe('useFolderPagination', () => {
    it('loads the first page with no cursor and records whether more remain', async () => {
        listMessagesPage.mockResolvedValue(page([msg('a'), msg('b')], true, 5, 'cur'))
        const {result} = renderHook(() => useFolderPagination())

        let got: Message[] = []
        await act(async () => {
            got = await result.current.loadFirst('inbox', false)
        })

        expect(got.map((m) => m.id)).toEqual(['a', 'b'])
        expect(listMessagesPage).toHaveBeenCalledWith('inbox', false, 0, '', 200, false)
        expect(result.current.hasMore()).toBe(true)
    })

    it('loads the next page after the recorded cursor, in the same direction, filtering already-loaded ids', async () => {
        listMessagesPage
            .mockResolvedValueOnce(page([msg('a'), msg('b')], true, 5, 'cur'))
            .mockResolvedValueOnce(page([msg('b'), msg('c')], false, 0, ''))
        const {result} = renderHook(() => useFolderPagination())

        await act(async () => {
            await result.current.loadFirst('inbox', true)
        })
        let more: Message[] = []
        await act(async () => {
            more = await result.current.loadNext('inbox', new Set(['a', 'b']))
        })

        // b is already loaded, so only c is returned; the second call carried the recorded cursor and the
        // ascending direction the first page was loaded in.
        expect(more.map((m) => m.id)).toEqual(['c'])
        expect(listMessagesPage).toHaveBeenNthCalledWith(2, 'inbox', true, 5, 'cur', 200, true)
        expect(result.current.hasMore()).toBe(false)
    })

    it('stops paging (no fetch) once no more pages remain', async () => {
        listMessagesPage.mockResolvedValueOnce(page([msg('a')], false, 0, ''))
        const {result} = renderHook(() => useFolderPagination())

        await act(async () => {
            await result.current.loadFirst('inbox', false)
        })
        listMessagesPage.mockClear()
        let none: Message[] = []
        await act(async () => {
            none = await result.current.loadNext('inbox', new Set(['a']))
        })

        expect(none).toEqual([])
        expect(listMessagesPage).not.toHaveBeenCalled()
    })

    it('guards overlapping loads so a second loadNext does not double-fetch', async () => {
        let resolveSecond!: (value: MessagePage) => void
        listMessagesPage
            .mockResolvedValueOnce(page([msg('a')], true, 1, 'c1'))
            .mockImplementationOnce(() => new Promise<MessagePage>((resolve) => {
                resolveSecond = resolve
            }))
        const {result} = renderHook(() => useFolderPagination())

        await act(async () => {
            await result.current.loadFirst('inbox', false)
        })
        let firstCall!: Promise<Message[]>
        let secondCall!: Promise<Message[]>
        await act(async () => {
            firstCall = result.current.loadNext('inbox', new Set(['a']))
            secondCall = result.current.loadNext('inbox', new Set(['a']))
            resolveSecond(page([msg('b')], false, 0, ''))
            await Promise.all([firstCall, secondCall])
        })

        // The overlapping second call is turned away while the first is in flight; only one page is fetched.
        expect(await secondCall).toEqual([])
        expect((await firstCall).map((m) => m.id)).toEqual(['b'])
        expect(listMessagesPage).toHaveBeenCalledTimes(2)
    })

    it('discards an in-flight page when the cursor is reset mid-load, without recording its cursor', async () => {
        let resolveNext!: (value: MessagePage) => void
        listMessagesPage
            .mockResolvedValueOnce(page([msg('a')], true, 1, 'c1'))
            .mockImplementationOnce(() => new Promise<MessagePage>((resolve) => {
                resolveNext = resolve
            }))
        const {result} = renderHook(() => useFolderPagination())

        await act(async () => {
            await result.current.loadFirst('inbox', false)
        })

        let nextCall!: Promise<Message[]>
        await act(async () => {
            nextCall = result.current.loadNext('inbox', new Set(['a']))
            // The user switches folder while the page is in flight: reset swaps the cursor object, so the
            // stale page that arrives next must be dropped rather than recorded over the fresh cursor.
            result.current.reset()
            resolveNext(page([msg('b')], true, 9, 'stale'))
            await nextCall
        })

        expect(await nextCall).toEqual([])
        // The stale page's hasMore/cursor were not recorded: the reset cursor still reports no more pages.
        expect(result.current.hasMore()).toBe(false)
    })

    it('reset clears the cursor so the next loadNext does nothing', async () => {
        listMessagesPage.mockResolvedValueOnce(page([msg('a')], true, 1, 'c1'))
        const {result} = renderHook(() => useFolderPagination())

        await act(async () => {
            await result.current.loadFirst('inbox', false)
        })
        expect(result.current.hasMore()).toBe(true)
        act(() => result.current.reset())
        expect(result.current.hasMore()).toBe(false)

        listMessagesPage.mockClear()
        let none: Message[] = []
        await act(async () => {
            none = await result.current.loadNext('inbox', new Set())
        })
        expect(none).toEqual([])
        expect(listMessagesPage).not.toHaveBeenCalled()
    })
})
