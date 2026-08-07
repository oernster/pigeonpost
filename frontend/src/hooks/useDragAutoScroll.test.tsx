// The pane's own edge auto-scroll during a drag. jsdom has no layout, no real scrolling and no real
// animation frames, so the element's bounds and scrollTop are stubbed and the frame loop is driven by hand:
// the test is about when the loop runs and in which direction, the pixel arithmetic being covered in
// dragScroll.test.ts. The drag events are dispatched by hand too, because jsdom's fireEvent drops clientY
// and relatedTarget from a drag event's init (the same gotcha the Sidebar drop tests work around).
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {useRef} from 'react'
import {cleanup, render} from '@testing-library/react'
import {useDragAutoScroll} from './useDragAutoScroll'

const PANE_TOP = 100
const PANE_HEIGHT = 400
const PANE_BOTTOM = PANE_TOP + PANE_HEIGHT

// frames holds the pending animation-frame callbacks, so a test steps the loop one frame at a time.
let frames: Map<number, FrameRequestCallback>
let nextFrameId: number

function runFrame() {
    const [id, callback] = [...frames][0]
    frames.delete(id)
    callback(0)
}

function Pane() {
    const ref = useRef<HTMLDivElement | null>(null)
    useDragAutoScroll(ref)
    return <div data-testid="pane" ref={ref}><span data-testid="row">row</span></div>
}

function renderPane() {
    const view = render(<Pane/>)
    const pane = view.getByTestId('pane')
    pane.getBoundingClientRect = () => ({
        top: PANE_TOP, bottom: PANE_BOTTOM, height: PANE_HEIGHT, left: 0, right: 200, width: 200,
        x: 0, y: PANE_TOP, toJSON: () => ({}),
    })
    // jsdom pins scrollTop at 0 on an element it never laid out, so it is backed by a plain value here.
    let scrollTop = 0
    Object.defineProperty(pane, 'scrollTop', {
        get: () => scrollTop,
        set: (value: number) => {
            scrollTop = value
        },
    })
    return {pane, row: view.getByTestId('row')}
}

// dispatch fires a drag event carrying the fields jsdom's own drag events drop, set as own properties.
function dispatch(target: EventTarget, type: string, fields: {clientY?: number; relatedTarget?: EventTarget} = {}) {
    const event = new Event(type, {bubbles: true})
    Object.assign(event, fields)
    target.dispatchEvent(event)
}

beforeEach(() => {
    frames = new Map()
    nextFrameId = 1
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
        const id = nextFrameId++
        frames.set(id, callback)
        return id
    })
    vi.stubGlobal('cancelAnimationFrame', (id: number) => {
        frames.delete(id)
    })
})
afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
})

describe('useDragAutoScroll', () => {
    it('scrolls down while the pointer rests near the bottom edge, frame after frame', () => {
        const {pane} = renderPane()
        dispatch(pane, 'dragover', {clientY: PANE_BOTTOM - 10})
        runFrame()
        const afterOne = pane.scrollTop
        expect(afterOne).toBeGreaterThan(0)
        // The pointer has not moved and no further dragover has fired: the loop keeps going regardless.
        runFrame()
        expect(pane.scrollTop).toBeGreaterThan(afterOne)
    })

    it('scrolls up near the top edge', () => {
        const {pane} = renderPane()
        pane.scrollTop = 500
        dispatch(pane, 'dragover', {clientY: PANE_TOP + 10})
        runFrame()
        expect(pane.scrollTop).toBeLessThan(500)
    })

    it('leaves the pane alone in the neutral middle band', () => {
        const {pane} = renderPane()
        dispatch(pane, 'dragover', {clientY: PANE_TOP + PANE_HEIGHT / 2})
        runFrame()
        expect(pane.scrollTop).toBe(0)
        // Still looping, ready for the pointer to move back into a zone.
        expect(frames.size).toBe(1)
    })

    it('starts a hot zone well clear of the base, where the browsers own scroll does nothing', () => {
        const {pane} = renderPane()
        dispatch(pane, 'dragover', {clientY: PANE_BOTTOM - 40})
        runFrame()
        expect(pane.scrollTop).toBeGreaterThan(0)
    })

    it('stops on drop', () => {
        const {pane} = renderPane()
        dispatch(pane, 'dragover', {clientY: PANE_BOTTOM - 10})
        dispatch(pane, 'drop')
        expect(frames.size).toBe(0)
    })

    it('stops when the drag ends anywhere', () => {
        const {pane} = renderPane()
        dispatch(pane, 'dragover', {clientY: PANE_BOTTOM - 10})
        dispatch(window, 'dragend')
        expect(frames.size).toBe(0)
    })

    it('stops when the pointer leaves the pane', () => {
        const {pane} = renderPane()
        dispatch(pane, 'dragover', {clientY: PANE_BOTTOM - 10})
        dispatch(pane, 'dragleave', {relatedTarget: document.body})
        expect(frames.size).toBe(0)
    })

    it('keeps going when the pointer only crosses onto a row inside the pane', () => {
        const {pane, row} = renderPane()
        dispatch(pane, 'dragover', {clientY: PANE_BOTTOM - 10})
        dispatch(pane, 'dragleave', {relatedTarget: row})
        expect(frames.size).toBe(1)
    })

    it('runs one loop however many dragover events arrive', () => {
        const {pane} = renderPane()
        dispatch(pane, 'dragover', {clientY: PANE_BOTTOM - 10})
        dispatch(pane, 'dragover', {clientY: PANE_BOTTOM - 12})
        expect(frames.size).toBe(1)
    })
})
