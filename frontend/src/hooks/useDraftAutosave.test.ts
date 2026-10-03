// The draft autosave at its outer interface: after a real edit it writes a recovery snapshot of a compose
// that holds something and clears the slot for one emptied back out. A pasted picture is something: a
// body holding only an image has no text, so a text-only check once cleared the slot for it instead of
// saving it. A crash then lost the picture.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, renderHook} from '@testing-library/react'
import type {Editor} from '@tiptap/react'
import {useDraftAutosave} from './useDraftAutosave'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({
    saveDraftRecovery: vi.fn(),
    clearDraftRecovery: vi.fn(),
}))

const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

// AUTOSAVE_WAIT_MS runs the clock past the hook's debounce, so the snapshot (or the clear) has landed.
const AUTOSAVE_WAIT_MS = 2000
const IMAGE_ONLY_HTML = '<p><img src="data:image/png;base64,AAAA"></p>'

// editorWith is a stand-in for the TipTap editor holding the given body; the hook reads only its text
// and its HTML.
function editorWith(text: string, html: string): Editor {
    return {getText: () => text, getHTML: () => html} as unknown as Editor
}

// editAndWait renders the hook over the given body, records one editor edit and lets the debounce run.
function editAndWait(editor: Editor) {
    const {result} = renderHook(() =>
        useDraftAutosave({accountId: 'acc1', to: '', cc: '', bcc: '', subject: '', editor}))
    act(() => result.current.noteEdit())
    act(() => {
        vi.advanceTimersByTime(AUTOSAVE_WAIT_MS)
    })
}

beforeEach(() => {
    vi.useFakeTimers()
    apiSpies.saveDraftRecovery.mockReset().mockResolvedValue(undefined)
    apiSpies.clearDraftRecovery.mockReset().mockResolvedValue(undefined)
})

afterEach(() => {
    cleanup()
    vi.useRealTimers()
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

describe('useDraftAutosave', () => {
    it('saves a body holding only a pasted image rather than clearing the slot', () => {
        editAndWait(editorWith('', IMAGE_ONLY_HTML))
        expect(apiSpies.clearDraftRecovery).not.toHaveBeenCalled()
        expect(apiSpies.saveDraftRecovery).toHaveBeenCalledWith(
            expect.objectContaining({accountId: 'acc1', bodyHtml: IMAGE_ONLY_HTML}))
    })

    it('clears the slot for a compose emptied back out', () => {
        editAndWait(editorWith('', '<p></p>'))
        expect(apiSpies.clearDraftRecovery).toHaveBeenCalled()
        expect(apiSpies.saveDraftRecovery).not.toHaveBeenCalled()
    })

    it('declares no spy the real api does not have', async () => {
        const actual = await vi.importActual<typeof import('../api')>('../api')
        expect(spiesNotInApi(actual, apiSpies as unknown as Record<string, unknown>)).toEqual([])
    })
})
