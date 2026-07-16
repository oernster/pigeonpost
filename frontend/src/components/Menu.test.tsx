// The title-tray menus open on hover as well as click: entering the trigger (or the open dropdown)
// keeps the menu up, headed with its name, and leaving both closes it after a short grace so the
// pointer can travel between them without the menu slamming shut.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, fireEvent, render, screen} from '@testing-library/react'
import {HOVER_CLOSE_DELAY_MS, Menu} from './Menu'

function renderMenu() {
    const onClick = vi.fn()
    const view = render(<Menu title="Edit" icon="E" items={[{label: 'Search', onClick}]}/>)
    const root = view.container.querySelector('.menu') as HTMLElement
    return {...view, root, onClick}
}

beforeEach(() => {
    vi.useFakeTimers()
})
afterEach(() => {
    vi.useRealTimers()
    cleanup()
})

describe('Menu hover', () => {
    it('opens on hover, headed with the menu name', () => {
        const {root} = renderMenu()
        expect(screen.queryByRole('menu')).toBeNull()
        fireEvent.mouseEnter(root)
        expect(screen.getByRole('menu')).toBeInTheDocument()
        expect(screen.getByText('Edit')).toBeInTheDocument()
        expect(screen.getByText('Search')).toBeInTheDocument()
    })

    it('closes only after the pointer has been gone for the grace period', () => {
        const {root} = renderMenu()
        fireEvent.mouseEnter(root)
        fireEvent.mouseLeave(root)
        // Still open within the grace period, so a diagonal move back in can keep it.
        expect(screen.getByRole('menu')).toBeInTheDocument()
        act(() => {
            vi.advanceTimersByTime(HOVER_CLOSE_DELAY_MS)
        })
        expect(screen.queryByRole('menu')).toBeNull()
    })

    it('coming back within the grace period keeps the menu open', () => {
        const {root} = renderMenu()
        fireEvent.mouseEnter(root)
        fireEvent.mouseLeave(root)
        fireEvent.mouseEnter(root)
        act(() => {
            vi.advanceTimersByTime(HOVER_CLOSE_DELAY_MS * 2)
        })
        expect(screen.getByRole('menu')).toBeInTheDocument()
    })

    it('choosing an item still closes the menu', () => {
        const {root, onClick} = renderMenu()
        fireEvent.mouseEnter(root)
        fireEvent.click(screen.getByText('Search'))
        expect(onClick).toHaveBeenCalled()
        expect(screen.queryByRole('menu')).toBeNull()
    })
})
