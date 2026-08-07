import {describe, expect, it} from 'vitest'
import {
    DRAG_SCROLL_EDGE_FRACTION,
    DRAG_SCROLL_EDGE_PX,
    DRAG_SCROLL_MAX_PX,
    DRAG_SCROLL_MIN_PX,
    dragScrollEdge,
    dragScrollStep,
} from './dragScroll'

// A tall pane, where the fixed edge depth applies, and a short one, where the fraction caps it.
const tall = {top: 100, bottom: 600, height: 500}
const short = {top: 0, bottom: 100, height: 100}

describe('dragScrollEdge', () => {
    it('uses the fixed depth on a pane tall enough to carry two of them', () => {
        expect(dragScrollEdge(tall.height)).toBe(DRAG_SCROLL_EDGE_PX)
    })

    it('caps the depth at a fraction of the height on a short pane', () => {
        expect(dragScrollEdge(short.height)).toBe(short.height * DRAG_SCROLL_EDGE_FRACTION)
    })
})

describe('dragScrollStep', () => {
    it('does not scroll in the neutral middle band', () => {
        expect(dragScrollStep(350, tall)).toBe(0)
    })

    it('does not scroll a pane with no height', () => {
        expect(dragScrollStep(0, {top: 0, bottom: 0, height: 0})).toBe(0)
    })

    it('scrolls up just inside the top zone, at the slow end', () => {
        const step = dragScrollStep(tall.top + DRAG_SCROLL_EDGE_PX - 1, tall)
        expect(step).toBeLessThan(0)
        expect(step).toBeGreaterThan(-2 * DRAG_SCROLL_MIN_PX)
    })

    it('scrolls up at full speed at the top edge and beyond it', () => {
        expect(dragScrollStep(tall.top, tall)).toBe(-DRAG_SCROLL_MAX_PX)
        expect(dragScrollStep(tall.top - 40, tall)).toBe(-DRAG_SCROLL_MAX_PX)
    })

    it('scrolls down just inside the bottom zone, at the slow end', () => {
        const step = dragScrollStep(tall.bottom - DRAG_SCROLL_EDGE_PX + 1, tall)
        expect(step).toBeGreaterThan(0)
        expect(step).toBeLessThan(2 * DRAG_SCROLL_MIN_PX)
    })

    it('scrolls down at full speed at the bottom edge and beyond it', () => {
        expect(dragScrollStep(tall.bottom, tall)).toBe(DRAG_SCROLL_MAX_PX)
        expect(dragScrollStep(tall.bottom + 40, tall)).toBe(DRAG_SCROLL_MAX_PX)
    })

    it('speeds up the deeper into the zone the pointer sits', () => {
        const shallow = dragScrollStep(tall.bottom - DRAG_SCROLL_EDGE_PX + 10, tall)
        const deep = dragScrollStep(tall.bottom - 5, tall)
        expect(deep).toBeGreaterThan(shallow)
    })

    it('reaches well beyond the browsers own few-pixel hot zone', () => {
        // The point of the widened zone: a pointer 40px clear of the base still scrolls.
        expect(dragScrollStep(tall.bottom - 40, tall)).toBeGreaterThan(0)
    })

    it('keeps a neutral band on a short pane, with both zones still live', () => {
        expect(dragScrollStep(short.top + 1, short)).toBeLessThan(0)
        expect(dragScrollStep(short.height / 2, short)).toBe(0)
        expect(dragScrollStep(short.bottom - 1, short)).toBeGreaterThan(0)
    })
})
