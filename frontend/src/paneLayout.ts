// paneLayout holds the pure sizing rules for the three-column layout's draggable splitters: the
// default and bounding widths of the sidebar and message-list columns, the clamps a drag runs
// through and the (de)serialisation of the persisted widths. Kept free of React and the api seam so
// it sits under the 100% coverage gate; the pointer plumbing lives in the usePaneWidths hook.

export interface PaneWidths {
    sidebar: number
    list: number
}

export const SIDEBAR_DEFAULT_PX = 260
export const LIST_DEFAULT_PX = 380
export const SIDEBAR_MIN_PX = 180
export const SIDEBAR_MAX_PX = 480
export const LIST_MIN_PX = 240
// READER_MIN_PX is the reading pane's floor: a list drag stops where the reader would fall below it.
export const READER_MIN_PX = 320
// LIST_STORED_MAX_PX caps a persisted list width at parse time, when the live container width is not
// yet known (a live drag re-clamps against the real container). It only guards against a corrupted
// store value, so it is deliberately generous.
export const LIST_STORED_MAX_PX = 1200

export function defaultPaneWidths(): PaneWidths {
    return {sidebar: SIDEBAR_DEFAULT_PX, list: LIST_DEFAULT_PX}
}

// clamp bounds value to [min, max]; degenerate bounds (max below min, a container too narrow for
// every column's floor) resolve to min so a drag in a tiny window stays deterministic.
function clamp(value: number, min: number, max: number): number {
    if (max < min) {
        return min
    }
    return Math.min(Math.max(value, min), max)
}

// clampSidebar bounds a dragged sidebar width to its fixed range.
export function clampSidebar(px: number): number {
    return clamp(Math.round(px), SIDEBAR_MIN_PX, SIDEBAR_MAX_PX)
}

// clampList bounds a dragged message-list width so the reading pane keeps its floor within the
// container: the maximum is whatever remains after the sidebar and the reader's minimum.
export function clampList(px: number, sidebarPx: number, containerPx: number): number {
    return clamp(Math.round(px), LIST_MIN_PX, containerPx - sidebarPx - READER_MIN_PX)
}

// parsePaneWidths reads the persisted JSON back into widths, clamping each field and falling back to
// its default on anything missing, malformed or non-numeric, so a corrupt store can never wedge the
// layout.
export function parsePaneWidths(raw: string | null): PaneWidths {
    if (!raw) {
        return defaultPaneWidths()
    }
    let parsed: unknown
    try {
        parsed = JSON.parse(raw)
    } catch {
        return defaultPaneWidths()
    }
    if (typeof parsed !== 'object' || parsed === null) {
        return defaultPaneWidths()
    }
    const record = parsed as {sidebar?: unknown; list?: unknown}
    const sidebar = typeof record.sidebar === 'number' && Number.isFinite(record.sidebar)
        ? clampSidebar(record.sidebar)
        : SIDEBAR_DEFAULT_PX
    const list = typeof record.list === 'number' && Number.isFinite(record.list)
        ? clamp(Math.round(record.list), LIST_MIN_PX, LIST_STORED_MAX_PX)
        : LIST_DEFAULT_PX
    return {sidebar, list}
}

export function serialisePaneWidths(widths: PaneWidths): string {
    return JSON.stringify(widths)
}
