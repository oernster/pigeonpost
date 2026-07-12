// The reader and the .eml viewer both drive image loading through useRemoteImages, so its contract is
// pinned here directly: it resolves a body's parked remote images through the proxy exactly once, falls back
// to the parked HTML on failure without looping and re-resolves when the body changes so one message's images
// never appear against another. The ../api module is mocked so no real fetch runs.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, renderHook, waitFor} from '@testing-library/react'
import {useRemoteImages} from './useRemoteImages'

const loadRemoteImages = vi.hoisted(() => vi.fn())
vi.mock('../api', () => ({api: {loadRemoteImages}}))

const PARKED = '<img data-pp-src="https://x.test/i.png">body'

beforeEach(() => {
    loadRemoteImages.mockReset().mockResolvedValue('')
})
afterEach(() => cleanup())

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
