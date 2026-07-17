import type {PointerEvent as ReactPointerEvent} from 'react'
import type {PaneWidthsControl, SplitterID} from '../hooks/usePaneWidths'

interface PaneSplittersProps {
    control: PaneWidthsControl
    // showListSplitter is false with the reading pane off: the two-column layout has no list|reader
    // boundary to drag.
    showListSplitter: boolean
}

// PaneSplitters renders the draggable vertical dividers between the three panes: sidebar|list and
// list|reader. Each is an absolutely positioned overlay sitting on its column boundary, so the grid
// children never reflow around a handle. Dragging resizes the column to its clamped bounds and
// double-click restores the default. The handles are deliberately not tab stops: the keyboard focus
// ring covers controls, and a divider is a mouse affordance with a keyboard-neutral default.
export function PaneSplitters({control, showListSplitter}: PaneSplittersProps) {
    const {widths, startDrag, resetWidth} = control
    const handle = (which: SplitterID, left: number, label: string) => (
        <div
            className="pane-splitter"
            data-splitter={which}
            role="separator"
            aria-orientation="vertical"
            aria-label={label}
            style={{left}}
            onPointerDown={(e: ReactPointerEvent<HTMLDivElement>) => startDrag(which, e)}
            onDoubleClick={() => resetWidth(which)}
        />
    )
    return (
        <>
            {handle('sidebar', widths.sidebar, 'Resize sidebar')}
            {showListSplitter && handle('list', widths.sidebar + widths.list, 'Resize message list')}
        </>
    )
}
