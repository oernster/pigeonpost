// Printing must keep the reader's privacy promise: a message's remote images stay parked in the printed copy
// unless the reader has already shown them. Even then they reach the print frame only as the image
// proxy's inlined data: URIs, never as a live link to the sender. The print frame carries the reader's own
// Content-Security-Policy as the second line of defence. The ../api module is mocked so no real fetch runs.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook, waitFor} from '@testing-library/react'
import {useMessageExport} from './useMessageExport'
import {useImagesShown} from './useImagesShown'
import {printFrameId} from '../print'
import {EMAIL_CONTENT_SECURITY_POLICY} from '../emailContentPolicy'
import {Message} from '../api'

import {unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({messageBody: vi.fn(), loadRemoteImages: vi.fn(), saveMessageAs: vi.fn()}))

const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

const PARKED_BODY = '<p><img data-pp-src="https://tracker.test/a.png">Body</p>'
const RESOLVED_BODY = '<p><img src="data:image/png;base64,AAAA">Body</p>'

function message(id: string): Message {
    return {id, subject: 'Weekly report', fromName: 'Ann', fromAddress: 'ann@x.test', date: '', snippet: ''} as Message
}

// printed returns the document written into the print frame once it has been written.
async function printed(): Promise<string> {
    await waitFor(() => expect(document.getElementById(printFrameId)).not.toBeNull())
    const frame = document.getElementById(printFrameId) as HTMLIFrameElement
    await waitFor(() => expect(frame.contentDocument?.documentElement.innerHTML ?? '').toContain('Weekly report'))
    return frame.contentDocument?.documentElement.outerHTML ?? ''
}

beforeEach(() => {
    apiSpies.messageBody.mockReset().mockResolvedValue({plain: '', html: PARKED_BODY, hasInvite: false, attachments: []})
    apiSpies.loadRemoteImages.mockReset().mockResolvedValue(RESOLVED_BODY)
})
afterEach(() => {
    cleanup()
    document.getElementById(printFrameId)?.remove()
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

describe('printMessage', () => {
    it('keeps remote images parked when the reader has not loaded them', async () => {
        const {result} = renderHook(() => useMessageExport({setError: vi.fn()}))
        await act(() => result.current.printMessage(message('p1')))
        const doc = await printed()
        expect(doc).toContain('data-pp-src="https://tracker.test/a.png"')
        expect(doc).not.toContain(' src="https://tracker.test')
        expect(apiSpies.loadRemoteImages).not.toHaveBeenCalled()
    })

    it('prints the proxy-inlined images once the reader has loaded them', async () => {
        const shown = renderHook(() => useImagesShown('p2', false))
        act(() => shown.result.current[1]())
        const {result} = renderHook(() => useMessageExport({setError: vi.fn()}))
        await act(() => result.current.printMessage(message('p2')))
        const doc = await printed()
        expect(apiSpies.loadRemoteImages).toHaveBeenCalledWith(PARKED_BODY)
        expect(doc).toContain('src="data:image/png;base64,AAAA"')
        expect(doc).not.toContain('tracker.test')
    })

    it('falls back to the parked copy and reports it when the image proxy fails', async () => {
        apiSpies.loadRemoteImages.mockRejectedValue(new Error('offline'))
        const shown = renderHook(() => useImagesShown('p3', true))
        expect(shown.result.current[0]).toBe(true)
        const setError = vi.fn()
        const {result} = renderHook(() => useMessageExport({setError}))
        await act(() => result.current.printMessage(message('p3')))
        const doc = await printed()
        expect(doc).toContain('data-pp-src="https://tracker.test/a.png"')
        expect(doc).not.toContain(' src="https://tracker.test')
        expect(setError).toHaveBeenCalledWith(expect.stringContaining('offline'))
    })

    it('locks the print frame with the reader frame Content-Security-Policy', async () => {
        const {result} = renderHook(() => useMessageExport({setError: vi.fn()}))
        await act(() => result.current.printMessage(message('p4')))
        const doc = await printed()
        expect(doc).toContain(`http-equiv="Content-Security-Policy" content="${EMAIL_CONTENT_SECURITY_POLICY}"`)
    })
})
