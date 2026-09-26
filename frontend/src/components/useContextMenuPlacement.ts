import {RefObject, useEffect, useLayoutEffect, useState} from 'react'

// Keep the menu at least this far inside the viewport edges when clamping its position.
const MENU_MARGIN = 8

// useContextMenuPlacement is what every right-click menu (the message's and the folder's) does beyond
// its own entries: it closes on a click outside the menu or on Escape, then keeps the menu inside the
// window when it was opened near an edge. `ref` is the menu's own element; (x, y) is where it was asked
// to open. It answers the position to draw the menu at.
export function useContextMenuPlacement(
    ref: RefObject<HTMLElement>, x: number, y: number, onClose: () => void,
): {x: number; y: number} {
    const [pos, setPos] = useState({x, y})

    // Dismiss on an outside click or the Escape key.
    useEffect(() => {
        const onDown = (e: MouseEvent) => {
            if (ref.current && !ref.current.contains(e.target as Node)) {
                onClose()
            }
        }
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                onClose()
            }
        }
        document.addEventListener('mousedown', onDown)
        document.addEventListener('keydown', onKey)
        return () => {
            document.removeEventListener('mousedown', onDown)
            document.removeEventListener('keydown', onKey)
        }
    }, [ref, onClose])

    // After the first render, nudge the menu back inside the viewport if the cursor was near an edge.
    // Flyouts are absolutely positioned, so they do not change the menu's own size and this runs once.
    useLayoutEffect(() => {
        const el = ref.current
        if (!el) {
            return
        }
        const rect = el.getBoundingClientRect()
        let nx = x
        let ny = y
        if (nx + rect.width > window.innerWidth - MENU_MARGIN) {
            nx = Math.max(MENU_MARGIN, window.innerWidth - rect.width - MENU_MARGIN)
        }
        if (ny + rect.height > window.innerHeight - MENU_MARGIN) {
            ny = Math.max(MENU_MARGIN, window.innerHeight - rect.height - MENU_MARGIN)
        }
        if (nx !== pos.x || ny !== pos.y) {
            setPos({x: nx, y: ny})
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [x, y])

    return pos
}
