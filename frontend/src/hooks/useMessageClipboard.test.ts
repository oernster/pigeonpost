// The message clipboard behind Edit > Cut / Copy / Paste at the message level: cut or copy takes
// messages onto an internal clipboard and paste files them into a folder. A pasted cut moves
// through bulkMoveIds (which records the undo entry) and is consumed; a pasted copy duplicates
// through the api and stays on the clipboard for further pastes. ../api is mocked (the Wails seam).
import {beforeEach, describe, expect, it, vi} from 'vitest'
import {act, renderHook} from '@testing-library/react'
import type {Message} from '../api'
import {useMessageClipboard} from './useMessageClipboard'

const apiSpies = vi.hoisted(() => ({
    copyMessage: vi.fn(),
    syncFolder: vi.fn(),
}))

vi.mock('../api', () => ({api: apiSpies}))

function makeMessage(id: string): Message {
    return {id, folderId: 'f1'} as Message
}

const bulkMoveIds = vi.fn<(ids: string[], destFolderId: string) => Promise<void>>()
const errors: string[] = []

function harness() {
    return renderHook(() => useMessageClipboard({
        bulkMoveIds,
        setError: (message: string) => errors.push(message),
    }))
}

beforeEach(() => {
    errors.length = 0
    bulkMoveIds.mockReset().mockResolvedValue(undefined)
    apiSpies.copyMessage.mockReset().mockResolvedValue(undefined)
    apiSpies.syncFolder.mockReset().mockResolvedValue(undefined)
})

describe('useMessageClipboard', () => {
    it('starts empty and ignores a cut or copy of nothing', () => {
        const {result} = harness()
        expect(result.current.hasClip).toBe(false)
        act(() => result.current.cutMessages([]))
        expect(result.current.hasClip).toBe(false)
    })

    it('pastes a cut as a batched move and consumes the clipboard', async () => {
        const {result} = harness()
        act(() => result.current.cutMessages([makeMessage('a'), makeMessage('b')]))
        expect(result.current.hasClip).toBe(true)
        await act(async () => {
            await result.current.pasteInto('fd')
        })
        expect(bulkMoveIds).toHaveBeenCalledWith(['a', 'b'], 'fd')
        expect(result.current.hasClip).toBe(false)
    })

    it('pastes a copy as duplicates and keeps the clipboard for further pastes', async () => {
        const {result} = harness()
        act(() => result.current.copyMessages([makeMessage('a'), makeMessage('b')]))
        await act(async () => {
            await result.current.pasteInto('fd')
        })
        expect(apiSpies.copyMessage.mock.calls).toEqual([['a', 'fd'], ['b', 'fd']])
        expect(apiSpies.syncFolder).toHaveBeenCalledWith('fd')
        expect(bulkMoveIds).not.toHaveBeenCalled()
        expect(result.current.hasClip).toBe(true)
    })

    it('reports how many copies could not be pasted', async () => {
        apiSpies.copyMessage.mockRejectedValueOnce('gone')
        const {result} = harness()
        act(() => result.current.copyMessages([makeMessage('a'), makeMessage('b')]))
        await act(async () => {
            await result.current.pasteInto('fd')
        })
        expect(errors).toContain('1 of 2 messages could not be pasted.')
    })

    it('pasting with an empty clipboard or no target folder is a no-op', async () => {
        const {result} = harness()
        await act(async () => {
            await result.current.pasteInto('fd')
        })
        act(() => result.current.cutMessages([makeMessage('a')]))
        await act(async () => {
            await result.current.pasteInto('')
        })
        expect(bulkMoveIds).not.toHaveBeenCalled()
        expect(result.current.hasClip).toBe(true)
    })

    it('a fresh cut replaces whatever was on the clipboard', async () => {
        const {result} = harness()
        act(() => result.current.copyMessages([makeMessage('a')]))
        act(() => result.current.cutMessages([makeMessage('b')]))
        await act(async () => {
            await result.current.pasteInto('fd')
        })
        expect(bulkMoveIds).toHaveBeenCalledWith(['b'], 'fd')
    })
})
