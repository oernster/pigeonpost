import {useCallback, useEffect, useRef, useState} from 'react'
import {
    TICK_MS,
    autoScrollTick,
    initialAutoScrollState,
    suspended,
    type AutoScrollState,
} from '../autoScroll'

// MANUAL_EVENTS are the inputs that count as reading by hand and so suspend the cycle. mousedown covers a
// press on the native scrollbar as well as on the content, and focusin covers keyboard navigation arriving
// in the surface: focus landing here is someone about to read, exactly like a scroll.
const MANUAL_EVENTS = ['wheel', 'mousedown', 'touchstart', 'keydown', 'focusin'] as const

// useAutoScroll gives a scrollable surface the self-reading cycle in autoScroll.ts. It returns a ref
// callback to put on the element that actually scrolls, so a surface that is mounted and unmounted with its
// dialog starts a fresh cycle each time it opens.
//
// Three things are handled here rather than in the state machine, because they are properties of the page:
// a reader who has asked for reduced motion gets no cycle at all, a surface underneath another modal is
// FROZEN rather than suspended (the tick is skipped entirely, so phase, position and the remaining hold are
// all still there when the modal above closes), and the movement the cycle applies is not mistaken for the
// reader moving it.
export function useAutoScroll(): (node: HTMLElement | null) => void {
    const [node, setNode] = useState<HTMLElement | null>(null)
    const state = useRef<AutoScrollState>(initialAutoScrollState())

    useEffect(() => {
        if (!node || prefersReducedMotion()) {
            return
        }
        state.current = initialAutoScrollState()
        const onManualInput = () => {
            state.current = suspended(state.current)
        }
        for (const type of MANUAL_EVENTS) {
            node.addEventListener(type, onManualInput, {passive: true})
        }
        const timer = window.setInterval(() => {
            if (!isTopmostModalSurface(node)) {
                return
            }
            const view = {
                scrollTop: node.scrollTop,
                maxScrollTop: node.scrollHeight - node.clientHeight,
            }
            const {state: next, delta} = autoScrollTick(state.current, view)
            state.current = next
            if (delta !== 0) {
                node.scrollTop = view.scrollTop + delta
            }
        }, TICK_MS)
        return () => {
            window.clearInterval(timer)
            for (const type of MANUAL_EVENTS) {
                node.removeEventListener(type, onManualInput)
            }
        }
    }, [node])

    return useCallback((next: HTMLElement | null) => setNode(next), [])
}

// prefersReducedMotion reports whether the reader has asked the system not to animate, in which case the
// content simply sits still and is theirs to scroll.
function prefersReducedMotion(): boolean {
    return window.matchMedia?.('(prefers-reduced-motion: reduce)').matches === true
}

// isTopmostModalSurface reports whether the surface is the one the reader is actually looking at. Two
// surfaces reading at once compete for the eye, so a surface under another modal stops moving until that
// modal closes. A surface outside any modal reads only while no modal is open at all.
function isTopmostModalSurface(node: HTMLElement): boolean {
    const backdrops = document.querySelectorAll('.modal-backdrop')
    const own = node.closest('.modal-backdrop')
    if (!own) {
        return backdrops.length === 0
    }
    return backdrops[backdrops.length - 1] === own
}
