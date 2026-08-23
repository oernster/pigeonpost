// modalDrag holds the pure geometry behind dragging a centred modal by its header. No React, no DOM
// lookups, so the clamp is unit-tested in isolation; the hook in hooks/useModalDrag.ts supplies the
// measurements and owns the pointer events.

// MODAL_DRAG_EDGE_PX is how much of the window must remain on screen at the left, right and bottom
// edges, so a dragged window can never be pushed out of reach.
export const MODAL_DRAG_EDGE_PX = 48

// DragOffset is the translation applied to a modal on top of the backdrop's flex centring. Zero is the
// centred resting position every window opens at.
export interface DragOffset {
    x: number
    y: number
}

// DragBounds is what clamping needs: the modal's centred (untranslated) left edge, top edge and width,
// plus the viewport it must stay reachable within. The height is not needed: the vertical limits are set
// by the top edge alone (see clampOffset), so it is deliberately absent rather than measured and ignored.
export interface DragBounds {
    baseLeft: number
    baseTop: number
    width: number
    viewportWidth: number
    viewportHeight: number
}

export const NO_DRAG: DragOffset = {x: 0, y: 0}

// clampOffset limits a candidate translation so the window stays grabbable. The horizontal limits keep
// MODAL_DRAG_EDGE_PX of the window inside either side; the vertical ones keep the top edge on screen (the
// header is the only drag handle, so a window dragged above the top could never be brought back) and keep
// the same strip visible above the bottom. Offsets are rounded to whole pixels: a fractional translation
// blurs the text inside a composited layer.
export function clampOffset(candidate: DragOffset, bounds: DragBounds): DragOffset {
    const {baseLeft, baseTop, width, viewportWidth, viewportHeight} = bounds
    const minX = MODAL_DRAG_EDGE_PX - baseLeft - width
    const maxX = viewportWidth - MODAL_DRAG_EDGE_PX - baseLeft
    const minY = -baseTop
    const maxY = viewportHeight - MODAL_DRAG_EDGE_PX - baseTop
    return {
        x: Math.round(clamp(candidate.x, minX, maxX)),
        y: Math.round(clamp(candidate.y, minY, maxY)),
    }
}

// clamp holds value within the range. A range whose minimum exceeds its maximum (a viewport smaller than
// the edge margin itself, where every position breaks one rule) resolves to the minimum, which keeps the
// top-left corner of the window on screen.
function clamp(value: number, min: number, max: number): number {
    if (max < min) {
        return min
    }
    return Math.min(Math.max(value, min), max)
}

// isDragHandle reports whether a pointer-down inside the header should start a drag. A press on a control
// (the close cross or anything else placed on the bar later) must reach that control instead.
export function isDragHandle(target: Element | null): boolean {
    return target !== null && target.closest('button, input, select, textarea, a') === null
}
