// dragScroll holds the pure geometry of edge auto-scrolling during a drag: how far in from a scrollable
// pane's top and bottom edge the hot zone reaches, and how fast the pane should scroll for a pointer
// resting in it. No DOM and no React, so it sits under the 100% coverage gate; the event and animation
// frame plumbing lives in the useDragAutoScroll hook.

// DRAG_SCROLL_EDGE_PX is how deep the hot zone reaches in from the top and the bottom edge. The browser's
// own drag auto-scroll only fires within a couple of pixels of the edge, which is close to unhittable while
// holding a drag, so the pane provides its own zone at a size that can be aimed at.
export const DRAG_SCROLL_EDGE_PX = 56

// DRAG_SCROLL_EDGE_FRACTION caps the zone on a short pane so the top and bottom zones can never meet: each
// takes at most this fraction of the pane's height, leaving a neutral band in the middle where a drop is a
// drop rather than a scroll.
export const DRAG_SCROLL_EDGE_FRACTION = 0.25

// The speed ramps with how deep into the zone the pointer sits: a slow nudge at the zone's inner boundary,
// where one more row is usually all that is wanted, up to a fast run at the very edge.
export const DRAG_SCROLL_MIN_PX = 3
export const DRAG_SCROLL_MAX_PX = 24

// PaneBounds is the part of a DOMRect the step calculation reads, so a test can state one directly.
export interface PaneBounds {
    top: number
    bottom: number
    height: number
}

// dragScrollEdge returns the hot zone's depth for a pane of the given height: the fixed size, cut back on a
// pane too short to carry two zones of it.
export function dragScrollEdge(height: number): number {
    return Math.min(DRAG_SCROLL_EDGE_PX, height * DRAG_SCROLL_EDGE_FRACTION)
}

// dragScrollStep returns how many pixels the pane should scroll on this frame for a pointer at clientY:
// negative to scroll up, positive to scroll down, zero in the neutral middle band. A pointer past the edge
// entirely (an overshoot while dragging) stays at full speed rather than falling back to nothing.
export function dragScrollStep(clientY: number, rect: PaneBounds): number {
    const edge = dragScrollEdge(rect.height)
    if (edge <= 0) {
        return 0
    }
    const speed = (depth: number) =>
        DRAG_SCROLL_MIN_PX + (DRAG_SCROLL_MAX_PX - DRAG_SCROLL_MIN_PX) * Math.min(1, depth / edge)
    const fromTop = rect.top + edge - clientY
    if (fromTop > 0) {
        return -speed(fromTop)
    }
    const fromBottom = clientY - (rect.bottom - edge)
    if (fromBottom > 0) {
        return speed(fromBottom)
    }
    return 0
}
