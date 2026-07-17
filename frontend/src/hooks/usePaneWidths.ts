import {useCallback, useRef, useState} from 'react'
import type {MouseEvent as ReactMouseEvent} from 'react'
import {
    PaneWidths,
    clampList,
    clampSidebar,
    defaultPaneWidths,
    LIST_DEFAULT_PX,
    parsePaneWidths,
    serialisePaneWidths,
    SIDEBAR_DEFAULT_PX,
} from '../paneLayout'

// The persisted-widths key, alongside the sidebar's collapsed and order keys in shape: UI layout
// state that survives a restart without touching the store.
const STORAGE_KEY = 'pigeonpost.panewidths'

export type SplitterID = 'sidebar' | 'list'

export interface PaneWidthsControl {
    widths: PaneWidths
    // startDrag begins a splitter drag from its mousedown; the hook tracks the mouse until release,
    // clamping through paneLayout and persisting the final widths.
    startDrag: (which: SplitterID, e: ReactMouseEvent<HTMLElement>) => void
    // resetWidth restores one splitter's column to its default (the double-click affordance).
    resetWidth: (which: SplitterID) => void
}

// usePaneWidths owns the widths of the sidebar and message-list columns, dragged via the pane
// splitters and remembered across restarts. The clamping rules are pure (paneLayout); this hook is
// only the pointer plumbing and persistence around them.
export function usePaneWidths(): PaneWidthsControl {
    const [widths, setWidths] = useState<PaneWidths>(() => {
        try {
            return parsePaneWidths(localStorage.getItem(STORAGE_KEY))
        } catch {
            return defaultPaneWidths()
        }
    })
    // widthsRef mirrors the state so a drag reads the widths at its start without a stale closure.
    const widthsRef = useRef(widths)
    widthsRef.current = widths

    const persist = (next: PaneWidths) => {
        try {
            localStorage.setItem(STORAGE_KEY, serialisePaneWidths(next))
        } catch {
            // A storage failure just means the layout is not remembered; resizing still works.
        }
    }

    const startDrag = useCallback((which: SplitterID, e: ReactMouseEvent<HTMLElement>) => {
        // Primary button only: a right- or middle-button press is not a resize.
        if (e.button !== 0) {
            return
        }
        // The splitters are absolutely positioned children of the .panes grid, so the parent is the
        // container whose left edge and width the clamps are measured against.
        const container = e.currentTarget.parentElement
        if (!container) {
            return
        }
        // Stop the drag from starting a text selection across the panes.
        e.preventDefault()
        const rect = container.getBoundingClientRect()
        // Plain mouse events on the window, not pointer capture on the handle: WKWebView (macOS) does
        // not reliably keep delivering captured pointer moves once the cursor crosses the reader's
        // sandboxed email iframe, which froze the drag there while WebView2 and webkit2gtk carried on.
        // Window listeners work identically on all three engines, and the pane-resizing body class
        // turns the iframes' pointer events off for the drag's duration so they cannot swallow the
        // stream (it also pins the col-resize cursor and suppresses text selection).
        document.body.classList.add('pane-resizing')
        let latest = widthsRef.current
        const move = (ev: MouseEvent) => {
            latest = which === 'sidebar'
                ? {...latest, sidebar: clampSidebar(ev.clientX - rect.left)}
                : {...latest, list: clampList(ev.clientX - rect.left - latest.sidebar, latest.sidebar, rect.width)}
            setWidths(latest)
        }
        const end = () => {
            window.removeEventListener('mousemove', move)
            window.removeEventListener('mouseup', end)
            window.removeEventListener('blur', end)
            document.body.classList.remove('pane-resizing')
            persist(latest)
        }
        window.addEventListener('mousemove', move)
        window.addEventListener('mouseup', end)
        // Losing the window mid-drag (Alt-Tab, a notification stealing focus) must not leave a stuck
        // drag with no mouseup to end it.
        window.addEventListener('blur', end)
    }, [])

    const resetWidth = useCallback((which: SplitterID) => {
        const next = which === 'sidebar'
            ? {...widthsRef.current, sidebar: SIDEBAR_DEFAULT_PX}
            : {...widthsRef.current, list: LIST_DEFAULT_PX}
        setWidths(next)
        persist(next)
    }, [])

    return {widths, startDrag, resetWidth}
}
