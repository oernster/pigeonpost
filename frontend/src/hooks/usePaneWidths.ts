import {useCallback, useRef, useState} from 'react'
import type {PointerEvent as ReactPointerEvent} from 'react'
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
    // startDrag begins a splitter drag from its pointerdown; the hook tracks the pointer until
    // release, clamping through paneLayout and persisting the final widths.
    startDrag: (which: SplitterID, e: ReactPointerEvent<HTMLElement>) => void
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

    const startDrag = useCallback((which: SplitterID, e: ReactPointerEvent<HTMLElement>) => {
        // The splitters are absolutely positioned children of the .panes grid, so the parent is the
        // container whose left edge and width the clamps are measured against.
        const container = e.currentTarget.parentElement
        if (!container) {
            return
        }
        // Stop the drag from starting a text selection across the panes.
        e.preventDefault()
        const rect = container.getBoundingClientRect()
        const handle = e.currentTarget
        try {
            handle.setPointerCapture(e.pointerId)
        } catch {
            // Pointer capture is an enhancement (it keeps fast drags from escaping the handle); a
            // runtime without it still drags via the listeners below.
        }
        let latest = widthsRef.current
        const move = (ev: PointerEvent) => {
            latest = which === 'sidebar'
                ? {...latest, sidebar: clampSidebar(ev.clientX - rect.left)}
                : {...latest, list: clampList(ev.clientX - rect.left - latest.sidebar, latest.sidebar, rect.width)}
            setWidths(latest)
        }
        const end = () => {
            handle.removeEventListener('pointermove', move)
            handle.removeEventListener('pointerup', end)
            handle.removeEventListener('pointercancel', end)
            persist(latest)
        }
        handle.addEventListener('pointermove', move)
        handle.addEventListener('pointerup', end)
        handle.addEventListener('pointercancel', end)
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
