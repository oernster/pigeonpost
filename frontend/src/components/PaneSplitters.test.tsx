// Behaviour test for the pane splitters through the real usePaneWidths hook: dragging a divider
// resizes its column within the clamped bounds, the result persists to localStorage and double-click
// restores the default. The container's rect is stubbed because jsdom lays nothing out.
import {afterEach, beforeEach, describe, expect, it} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {PaneSplitters} from './PaneSplitters'
import {usePaneWidths} from '../hooks/usePaneWidths'
import {LIST_DEFAULT_PX, SIDEBAR_DEFAULT_PX, SIDEBAR_MAX_PX} from '../paneLayout'

const STORAGE_KEY = 'pigeonpost.panewidths'
const CONTAINER_WIDTH_PX = 1200

function Harness({showList = true}: {showList?: boolean}) {
    const control = usePaneWidths()
    return (
        <div data-testid="panes">
            <PaneSplitters control={control} showListSplitter={showList}/>
            <output data-testid="widths">{control.widths.sidebar},{control.widths.list}</output>
        </div>
    )
}

function renderSplitters(showList = true) {
    const view = render(<Harness showList={showList}/>)
    const panes = view.container.querySelector<HTMLElement>('[data-testid="panes"]')!
    panes.getBoundingClientRect = () => ({
        left: 0, width: CONTAINER_WIDTH_PX, top: 0, bottom: 0, right: CONTAINER_WIDTH_PX, height: 800, x: 0, y: 0,
        toJSON: () => ({}),
    }) as DOMRect
    const splitter = (which: string) => view.container.querySelector<HTMLElement>(`[data-splitter="${which}"]`)
    const widths = () => screen.getByTestId('widths').textContent
    return {...view, splitter, widths}
}

beforeEach(() => localStorage.clear())
afterEach(() => cleanup())

describe('PaneSplitters', () => {
    it('starts at the defaults and positions the handles on the column edges', () => {
        const {splitter, widths} = renderSplitters()
        expect(widths()).toBe(`${SIDEBAR_DEFAULT_PX},${LIST_DEFAULT_PX}`)
        expect(splitter('sidebar')!.style.left).toBe(`${SIDEBAR_DEFAULT_PX}px`)
        expect(splitter('list')!.style.left).toBe(`${SIDEBAR_DEFAULT_PX + LIST_DEFAULT_PX}px`)
    })

    it('restores persisted widths', () => {
        localStorage.setItem(STORAGE_KEY, '{"sidebar": 320, "list": 500}')
        const {widths} = renderSplitters()
        expect(widths()).toBe('320,500')
    })

    it('drags the sidebar divider, clamps it and persists on release', () => {
        const {splitter, widths} = renderSplitters()
        const handle = splitter('sidebar')!
        fireEvent.pointerDown(handle, {pointerId: 1, clientX: SIDEBAR_DEFAULT_PX})
        fireEvent.pointerMove(handle, {clientX: 340})
        expect(widths()).toBe(`340,${LIST_DEFAULT_PX}`)
        // Way past the bound: the clamp holds the maximum.
        fireEvent.pointerMove(handle, {clientX: 5000})
        expect(widths()).toBe(`${SIDEBAR_MAX_PX},${LIST_DEFAULT_PX}`)
        fireEvent.pointerUp(handle)
        expect(localStorage.getItem(STORAGE_KEY)).toBe(`{"sidebar":${SIDEBAR_MAX_PX},"list":${LIST_DEFAULT_PX}}`)
        // The drag has ended: a later move on the handle no longer resizes.
        fireEvent.pointerMove(handle, {clientX: 300})
        expect(widths()).toBe(`${SIDEBAR_MAX_PX},${LIST_DEFAULT_PX}`)
    })

    it('drags the list divider against the container width', () => {
        const {splitter, widths} = renderSplitters()
        const handle = splitter('list')!
        fireEvent.pointerDown(handle, {pointerId: 1, clientX: SIDEBAR_DEFAULT_PX + LIST_DEFAULT_PX})
        fireEvent.pointerMove(handle, {clientX: SIDEBAR_DEFAULT_PX + 500})
        expect(widths()).toBe(`${SIDEBAR_DEFAULT_PX},500`)
        fireEvent.pointerUp(handle)
        expect(localStorage.getItem(STORAGE_KEY)).toBe(`{"sidebar":${SIDEBAR_DEFAULT_PX},"list":500}`)
    })

    it('cancelling a drag keeps the width reached so far without a stuck drag', () => {
        const {splitter, widths} = renderSplitters()
        const handle = splitter('sidebar')!
        fireEvent.pointerDown(handle, {pointerId: 1, clientX: SIDEBAR_DEFAULT_PX})
        fireEvent.pointerMove(handle, {clientX: 300})
        fireEvent.pointerCancel(handle)
        fireEvent.pointerMove(handle, {clientX: 450})
        expect(widths()).toBe(`300,${LIST_DEFAULT_PX}`)
    })

    it('double-click restores a divider to its default and persists it', () => {
        localStorage.setItem(STORAGE_KEY, '{"sidebar": 320, "list": 500}')
        const {splitter, widths} = renderSplitters()
        fireEvent.doubleClick(splitter('sidebar')!)
        expect(widths()).toBe(`${SIDEBAR_DEFAULT_PX},500`)
        expect(localStorage.getItem(STORAGE_KEY)).toBe(`{"sidebar":${SIDEBAR_DEFAULT_PX},"list":500}`)
    })

    it('hides the list divider when the reading pane is off', () => {
        const {splitter} = renderSplitters(false)
        expect(splitter('sidebar')).not.toBeNull()
        expect(splitter('list')).toBeNull()
    })
})
