// The close choice at its outer interface: minimise, quit and cancel, plus the unsaved-work
// callout that appears when another dialog is open behind it (the dialog samples the DOM as it
// opens, exactly what a quit would take down).
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {CloseChoiceDialog} from './CloseChoiceDialog'

function renderDialog() {
    const onMinimise = vi.fn()
    const onQuit = vi.fn()
    const onCancel = vi.fn()
    render(<CloseChoiceDialog onMinimise={onMinimise} onQuit={onQuit} onCancel={onCancel}/>)
    return {onMinimise, onQuit, onCancel}
}

afterEach(() => {
    cleanup()
    document.querySelectorAll('.modal').forEach((el) => el.remove())
})

describe('CloseChoiceDialog', () => {
    it('offers minimise (focused), quit and dismissal with no work open', () => {
        const {onMinimise, onQuit} = renderDialog()
        expect(screen.queryByText(/may be lost if you quit/)).toBeNull()
        expect(screen.queryByRole('button', {name: 'Go back'})).toBeNull()
        expect(screen.getByRole('button', {name: 'Minimise to tray'})).toHaveFocus()
        fireEvent.click(screen.getByRole('button', {name: 'Quit'}))
        expect(onQuit).toHaveBeenCalled()
        fireEvent.click(screen.getByRole('button', {name: 'Minimise to tray'}))
        expect(onMinimise).toHaveBeenCalled()
    })

    it('calls out possible data loss when another dialog is open and defaults to Go back', () => {
        const openWork = document.createElement('div')
        openWork.className = 'modal'
        document.body.appendChild(openWork)

        const {onCancel} = renderDialog()
        expect(screen.getByText(/anything unsaved in it may be lost if you quit/)).toBeInTheDocument()
        const goBack = screen.getByRole('button', {name: 'Go back'})
        expect(goBack).toHaveFocus()
        fireEvent.click(goBack)
        expect(onCancel).toHaveBeenCalled()
    })

    it('still allows quitting past the warning', () => {
        const openWork = document.createElement('div')
        openWork.className = 'modal'
        document.body.appendChild(openWork)

        const {onQuit} = renderDialog()
        fireEvent.click(screen.getByRole('button', {name: 'Quit'}))
        expect(onQuit).toHaveBeenCalled()
    })
})
