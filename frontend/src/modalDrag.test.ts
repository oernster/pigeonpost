import {describe, expect, it} from 'vitest'
import {clampOffset, DragBounds, isDragHandle, MODAL_DRAG_EDGE_PX, NO_DRAG} from './modalDrag'

// A 600x400 window centred in a 1000x800 viewport: the resting box every case is measured against.
const centred: DragBounds = {
    baseLeft: 200,
    baseTop: 200,
    width: 600,
    viewportWidth: 1000,
    viewportHeight: 800,
}

describe('clampOffset', () => {
    it('leaves a translation well inside the viewport alone', () => {
        expect(clampOffset({x: 120, y: -60}, centred)).toEqual({x: 120, y: -60})
    })

    it('rounds a fractional translation to whole pixels', () => {
        expect(clampOffset({x: 10.4, y: -3.5}, centred)).toEqual({x: 10, y: -3})
    })

    it('keeps a strip of the window on screen when dragged left', () => {
        // The right edge rests at 800; it may travel left until it sits MODAL_DRAG_EDGE_PX from zero.
        const limit = MODAL_DRAG_EDGE_PX - centred.baseLeft - centred.width
        expect(clampOffset({x: -5000, y: 0}, centred)).toEqual({x: limit, y: 0})
    })

    it('keeps a strip of the window on screen when dragged right', () => {
        const limit = centred.viewportWidth - MODAL_DRAG_EDGE_PX - centred.baseLeft
        expect(clampOffset({x: 5000, y: 0}, centred)).toEqual({x: limit, y: 0})
    })

    it('never lets the top edge leave the viewport, so the header stays grabbable', () => {
        expect(clampOffset({x: 0, y: -5000}, centred)).toEqual({x: 0, y: -centred.baseTop})
    })

    it('keeps a strip of the window above the bottom edge', () => {
        const limit = centred.viewportHeight - MODAL_DRAG_EDGE_PX - centred.baseTop
        expect(clampOffset({x: 0, y: 5000}, centred)).toEqual({x: 0, y: limit})
    })

    it('holds the top-left corner on screen in a viewport smaller than the margin itself', () => {
        // Every position breaks one rule here, so the minimum wins and the window stays reachable.
        const tiny: DragBounds = {baseLeft: 4, baseTop: 5, width: 600, viewportWidth: 10, viewportHeight: 10}
        expect(clampOffset({x: 300, y: 300}, tiny)).toEqual({x: -42, y: -5})
    })

    it('resolves the resting position to no translation at all', () => {
        expect(clampOffset(NO_DRAG, centred)).toEqual(NO_DRAG)
    })
})

describe('isDragHandle', () => {
    it('rejects a press with no target', () => {
        expect(isDragHandle(null)).toBe(false)
    })

    it('rejects a press on a control sitting on the bar', () => {
        const bar = document.createElement('div')
        bar.innerHTML = '<button><span>x</span></button>'
        expect(isDragHandle(bar.querySelector('button'))).toBe(false)
        expect(isDragHandle(bar.querySelector('span'))).toBe(false)
    })

    it('accepts a press on the bar itself', () => {
        const bar = document.createElement('div')
        bar.innerHTML = '<h2>New message</h2>'
        expect(isDragHandle(bar.querySelector('h2'))).toBe(true)
    })
})
