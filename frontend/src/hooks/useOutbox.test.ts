// Cancelling a queued send. The backend answers whether the item was still queued; a false answer means a
// send had already claimed it, so the user must be told the message was not stopped instead of being left
// with the confirmation's "will not be sent". ../api is mocked (the Wails seam).
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook, waitFor} from '@testing-library/react'
import type {Message} from '../api'
import {useOutbox} from './useOutbox'
import {cancelSendTooLateMessage} from '../confirmations'
import {unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({
    listOutbox: vi.fn(),
    cancelOutboxItem: vi.fn(),
}))

// The mock is built from the real api, so a method reached with no spy fails the test by name.
const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

const QUEUED = {id: 'q1', subject: 'Quarterly figures'} as Message

const errors: string[] = []

function harness() {
    return renderHook(() => useOutbox({
        selectedAccount: 'a1',
        folders: [],
        setError: (message: string) => errors.push(message),
    }))
}

beforeEach(() => {
    errors.length = 0
    apiSpies.listOutbox.mockReset().mockResolvedValue([])
    apiSpies.cancelOutboxItem.mockReset()
})

afterEach(() => {
    cleanup()
    expect(unstubbedNames(unstubbedCalls)).toEqual([])
})

// cancelQueued opens the confirmation for QUEUED and confirms it.
async function cancelQueued(result: ReturnType<typeof harness>['result']) {
    await waitFor(() => expect(apiSpies.listOutbox).toHaveBeenCalled())
    act(() => result.current.setMessageToCancelSend(QUEUED))
    await act(async () => {
        await result.current.cancelSend()
    })
}

describe('useOutbox cancelSend', () => {
    it('says nothing more when the item was still queued and is now stopped', async () => {
        apiSpies.cancelOutboxItem.mockResolvedValue(true)
        const {result} = harness()
        await cancelQueued(result)
        expect(apiSpies.cancelOutboxItem).toHaveBeenCalledWith('q1')
        expect(errors.filter((e) => e !== '')).toEqual([])
        expect(result.current.messageToCancelSend).toBeNull()
    })

    it('tells the user when a send had already claimed the item', async () => {
        apiSpies.cancelOutboxItem.mockResolvedValue(false)
        const {result} = harness()
        await cancelQueued(result)
        expect(errors.filter((e) => e !== '')).toEqual([cancelSendTooLateMessage('Quarterly figures')])
        expect(result.current.messageToCancelSend).toBeNull()
    })
})
