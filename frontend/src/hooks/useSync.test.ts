// The manual account sync. A sync that fails part way through has still cached the folder list and every
// folder it reached before it stopped, so the views are refreshed either way; the failure itself is what
// the user reads. ../api is mocked (the Wails seam).
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook} from '@testing-library/react'
import type {Folder} from '../api'
import {autoSyncIntervalMs, useSync} from './useSync'
import {unstubbedNames} from '../test/apiMock'
import {badgeRefresh} from '../test/badgeRefresh'

const badges = badgeRefresh()

const apiSpies = vi.hoisted(() => ({
    syncAccount: vi.fn(),
    replayOutbox: vi.fn(),
    listFolders: vi.fn(),
    syncFolder: vi.fn(),
}))

// The mock is built from the real api rather than hand-listed here, so a method reached with no spy fails
// the test by name instead of throwing a TypeError into the nearest catch and passing.
const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

const INBOX = {
    id: 'f1', accountId: 'a1', path: 'INBOX', name: 'Inbox', kind: 'inbox', unread: 0, total: 0,
} as Folder

const errors: string[] = []
const applied: {accountId: string, folders: Folder[]}[] = []

// harness opens `selectedFolder` (none by default, which keeps the background poll off).
function harness(selectedFolder = '') {
    return renderHook(() => useSync({
        selectedAccount: 'a1',
        selectedFolder,
        selectedFolderRef: {current: selectedFolder},
        applyFolders: (accountId: string, folders: Folder[]) => applied.push({accountId, folders}),
        reloadFolder: async () => undefined,
        refreshOutbox: async () => undefined,
        loadUnread: async () => undefined,
        ...badges.deps,
        setError: (message: string) => errors.push(message),
    }))
}

beforeEach(() => {
    errors.length = 0
    applied.length = 0
    badges.reset()
    apiSpies.syncAccount.mockReset().mockResolvedValue(undefined)
    apiSpies.replayOutbox.mockReset().mockResolvedValue(undefined)
    apiSpies.listFolders.mockReset().mockResolvedValue([INBOX])
    apiSpies.syncFolder.mockReset().mockResolvedValue(undefined)
})

afterEach(() => {
    cleanup()
    vi.useRealTimers()
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

describe('useSync: a sync that fails part way through', () => {
    it('still loads the cached folder list, so the sidebar does not claim there is none', async () => {
        apiSpies.syncAccount.mockRejectedValue(new Error('sync: fetch messages for "All Mail": unreadable'))
        const {result} = harness()

        await act(async () => {
            await result.current.sync()
        })

        expect(apiSpies.listFolders).toHaveBeenCalledWith('a1')
        expect(applied).toEqual([{accountId: 'a1', folders: [INBOX]}])
        expect(errors.join()).toContain('fetch messages')
    })

    it('reports the sync failure rather than a refresh failure behind it', async () => {
        apiSpies.syncAccount.mockRejectedValue(new Error('the real failure'))
        apiSpies.listFolders.mockRejectedValue(new Error('a later refresh failure'))
        const {result} = harness()

        await act(async () => {
            await result.current.sync()
        })

        expect(errors.join()).toContain('the real failure')
        expect(errors.join()).not.toContain('a later refresh failure')
    })

    it('clears the syncing state for the account whether it worked or not', async () => {
        apiSpies.syncAccount.mockRejectedValue(new Error('boom'))
        const {result} = harness()

        await act(async () => {
            await result.current.sync()
        })

        expect(result.current.accountSyncing).toBe(false)
    })
})

describe('useSync: a sync that works', () => {
    it('refreshes the folder list and reports nothing', async () => {
        const {result} = harness()

        await act(async () => {
            await result.current.sync()
        })

        expect(applied).toEqual([{accountId: 'a1', folders: [INBOX]}])
        expect(errors.filter((message) => message !== '')).toEqual([])
    })
})

// Mail can arrive in the open folder between syncs, so each background poll refreshes the unread badges.
describe('useSync: the background poll', () => {
    it('refreshes the unread badges after re-syncing the open folder', async () => {
        vi.useFakeTimers()
        harness('f1')

        await act(async () => {
            await vi.advanceTimersByTimeAsync(autoSyncIntervalMs)
        })

        expect(apiSpies.syncFolder).toHaveBeenCalledWith('f1')
        expect(badges.refreshed()).toBe(true)
    })
})
