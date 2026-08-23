import {CSSProperties, PointerEvent as ReactPointerEvent, RefObject, useCallback, useEffect, useRef, useState} from 'react'
import {clampOffset, DragOffset, isDragHandle, NO_DRAG} from '../modalDrag'

// ModalDrag is what a draggable modal spreads onto its own elements: the ref and style go on the modal
// box, the handle props go on the bar that moves it.
export interface ModalDrag {
    ref: RefObject<HTMLDivElement>
    style: CSSProperties
    handleProps: {
        onPointerDown: (e: ReactPointerEvent<HTMLElement>) => void
        onPointerMove: (e: ReactPointerEvent<HTMLElement>) => void
        onPointerUp: (e: ReactPointerEvent<HTMLElement>) => void
        onPointerCancel: (e: ReactPointerEvent<HTMLElement>) => void
        className: string
    }
    dragging: boolean
}

// useModalDrag makes a centred modal movable by its header, so a compose window can be pushed aside to
// read what is underneath it. The backdrop keeps doing the centring and the drag applies a translation on
// top, so the offset belongs to this modal instance alone: a window always opens centred and a closed one
// forgets where it was put. The pointer is captured on the header, so a fast drag that outruns the cursor
// keeps moving the window rather than dropping it.
export function useModalDrag(): ModalDrag {
    const ref = useRef<HTMLDivElement>(null)
    const [offset, setOffset] = useState<DragOffset>(NO_DRAG)
    const [dragging, setDragging] = useState(false)
    // origin is where the drag started: the pointer position and the offset in force at the time, so each
    // move is measured from the press rather than accumulated frame by frame.
    const origin = useRef({pointerX: 0, pointerY: 0, offset: NO_DRAG})

    // bounded resolves a candidate translation against the modal's measured box. getBoundingClientRect
    // reports the box as it is drawn, so the offset already applied is backed out to recover the centred
    // base position the clamp is expressed against.
    const bounded = useCallback((candidate: DragOffset, applied: DragOffset): DragOffset => {
        const el = ref.current
        if (!el) {
            return candidate
        }
        const rect = el.getBoundingClientRect()
        return clampOffset(candidate, {
            baseLeft: rect.left - applied.x,
            baseTop: rect.top - applied.y,
            width: rect.width,
            viewportWidth: window.innerWidth,
            viewportHeight: window.innerHeight,
        })
    }, [])

    // A window resized smaller can leave a moved modal off the edge, so the offset is re-clamped against
    // the new viewport. A modal still at rest is left alone, which is every modal that was never dragged.
    useEffect(() => {
        const onResize = () => setOffset((current) =>
            current.x === 0 && current.y === 0 ? current : bounded(current, current))
        window.addEventListener('resize', onResize)
        return () => window.removeEventListener('resize', onResize)
    }, [bounded])

    const onPointerDown = (e: ReactPointerEvent<HTMLElement>) => {
        // Primary button only; never a press that belongs to a control sitting on the bar.
        if (e.button !== 0 || !isDragHandle(e.target as Element)) {
            return
        }
        // Stop the press selecting the header text as the pointer travels.
        e.preventDefault()
        e.currentTarget.setPointerCapture(e.pointerId)
        origin.current = {pointerX: e.clientX, pointerY: e.clientY, offset}
        setDragging(true)
    }

    const onPointerMove = (e: ReactPointerEvent<HTMLElement>) => {
        if (!dragging) {
            return
        }
        const start = origin.current
        const candidate = {
            x: start.offset.x + e.clientX - start.pointerX,
            y: start.offset.y + e.clientY - start.pointerY,
        }
        setOffset(bounded(candidate, offset))
    }

    const endDrag = (e: ReactPointerEvent<HTMLElement>) => {
        if (!dragging) {
            return
        }
        if (e.currentTarget.hasPointerCapture(e.pointerId)) {
            e.currentTarget.releasePointerCapture(e.pointerId)
        }
        setDragging(false)
    }

    return {
        ref,
        style: offset.x === 0 && offset.y === 0
            ? {}
            : {transform: `translate(${offset.x}px, ${offset.y}px)`},
        handleProps: {
            onPointerDown,
            onPointerMove,
            onPointerUp: endDrag,
            onPointerCancel: endDrag,
            className: dragging ? 'modal-drag-handle dragging' : 'modal-drag-handle',
        },
        dragging,
    }
}
