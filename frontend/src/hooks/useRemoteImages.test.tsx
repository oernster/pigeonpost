// The reader and the .eml viewer both drive image loading through useRemoteImages, so its contract is
// pinned here directly: it resolves a body's parked remote images through the proxy exactly once, falls back
// to the parked HTML on failure without looping and re-resolves when the body changes so one message's images
// never appear against another. The ../api module is mocked so no real fetch runs.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, renderHook, waitFor} from '@testing-library/react'
import {useRemoteImages} from './useRemoteImages'

import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({loadRemoteImages: vi.fn()}))
const loadRemoteImages = apiSpies.loadRemoteImages

// The mock is built from the real api rather than hand-listed here, so a method reached with no spy
// fails the test by name instead of throwing a TypeError into the nearest catch and passing. The
// afterEach below reports any that were reached. See src/test/apiMock.ts.
const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

const PARKED = '<img data-pp-src="https://x.test/i.png">body'

beforeEach(() => {
    loadRemoteImages.mockReset().mockResolvedValue('')
})
afterEach(() => {
    cleanup()
    // Unmounting can reach the api too, so this is read after cleanup rather than before it.
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

describe('useRemoteImages', () => {
    it('returns the parked HTML and does not call the proxy while images are hidden', () => {
        const {result} = renderHook(() => useRemoteImages(PARKED, false))
        expect(result.current.renderedHtml).toBe(PARKED)
        expect(result.current.hasBlockedImages).toBe(true)
        expect(loadRemoteImages).not.toHaveBeenCalled()
    })

    it('does not call the proxy when the body has no blocked images', () => {
        const {result} = renderHook(() => useRemoteImages('<p>plain</p>', true))
        expect(result.current.hasBlockedImages).toBe(false)
        expect(result.current.renderedHtml).toBe('<p>plain</p>')
        expect(loadRemoteImages).not.toHaveBeenCalled()
    })

    it('resolves the images once and renders the inlined HTML when shown', async () => {
        loadRemoteImages.mockResolvedValue('<img src="data:image/png;base64,AAAA">body')
        const {result} = renderHook(() => useRemoteImages(PARKED, true))
        expect(loadRemoteImages).toHaveBeenCalledWith(PARKED)
        await waitFor(() => expect(result.current.renderedHtml).toContain('data:image/png;base64,AAAA'))
        expect(result.current.loadingImages).toBe(false)
        expect(loadRemoteImages).toHaveBeenCalledTimes(1)
    })

    it('falls back to the parked HTML and does not loop when the proxy fails', async () => {
        loadRemoteImages.mockRejectedValue(new Error('boom'))
        const {result} = renderHook(() => useRemoteImages(PARKED, true))
        await waitFor(() => expect(result.current.loadingImages).toBe(false))
        expect(result.current.renderedHtml).toBe(PARKED)
        expect(loadRemoteImages).toHaveBeenCalledTimes(1)
    })

    it('re-resolves for a new body so a previous message\'s images are not shown against it', async () => {
        loadRemoteImages.mockResolvedValueOnce('<img src="data:image/png;base64,AAAA">a')
        const {result, rerender} = renderHook(({html}) => useRemoteImages(html, true), {
            initialProps: {html: '<img data-pp-src="https://x.test/a.png">a'},
        })
        await waitFor(() => expect(result.current.renderedHtml).toContain('AAAA'))
        loadRemoteImages.mockResolvedValueOnce('<img src="data:image/png;base64,BBBB">b')
        rerender({html: '<img data-pp-src="https://x.test/b.png">b'})
        await waitFor(() => expect(result.current.renderedHtml).toContain('BBBB'))
        expect(result.current.renderedHtml).not.toContain('AAAA')
        expect(loadRemoteImages).toHaveBeenCalledTimes(2)
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
