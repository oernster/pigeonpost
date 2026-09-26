// useUnreadBadges owns the unread counts and the one refresh that pairs them with the folder list. ../api
// is mocked (the Wails seam).
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook, waitFor} from '@testing-library/react'
import {useUnreadBadges} from './useUnreadBadges'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({
    unreadCounts: vi.fn(),
}))

// The mock is built from the real api rather than hand-listed here, so a method reached with no spy fails
// the test by name instead of throwing a TypeError into the nearest catch and passing.
const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

const COUNTS = {total: 3, byAccount: {a1: 3}, newestByAccount: {}}

const errors: string[] = []
const refreshFolders = vi.fn(async () => {})
// setError is one stable function, as App's useState setter is: a fresh sink per render would give loadUnread
// a new identity and re-run the mount load, which App never does.
const setError = (message: string) => {
    errors.push(message)
}

function harness() {
    return renderHook(() => useUnreadBadges({refreshFolders, setError}))
}

beforeEach(() => {
    errors.length = 0
    refreshFolders.mockClear()
    apiSpies.unreadCounts.mockReset().mockResolvedValue(COUNTS)
})
afterEach(() => {
    cleanup()
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

describe('useUnreadBadges', () => {
    it('loads the counts on mount', async () => {
        const {result} = harness()

        await waitFor(() => expect(result.current.unreadCounts).toEqual(COUNTS))
    })

    it('refreshes the counts and the folder list together', async () => {
        const {result} = harness()
        await waitFor(() => expect(apiSpies.unreadCounts).toHaveBeenCalledTimes(1))

        await act(async () => {
            await result.current.refreshBadges()
        })

        expect(apiSpies.unreadCounts).toHaveBeenCalledTimes(2)
        expect(refreshFolders).toHaveBeenCalledTimes(1)
    })

    it('reports a failed count read through the error sink rather than rejecting', async () => {
        apiSpies.unreadCounts.mockRejectedValue('offline')
        const {result} = harness()

        await act(async () => {
            await result.current.loadUnread()
        })

        expect(errors).toContain('offline')
    })
})

describe('the api mock', () => {
    it('declares no spy the real api does not have', async () => {
        const actual = await vi.importActual<typeof import('../api')>('../api')
        expect(spiesNotInApi(actual, apiSpies as unknown as Record<string, unknown>)).toEqual([])
    })
})
