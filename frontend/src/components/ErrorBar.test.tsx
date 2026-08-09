// Behaviour test for the main window's error banner. It pins the three behaviours that matter: the bar
// renders nothing when there is no error, it announces the message as an alert when there is one and its
// dismiss button hands control back to the owner (App clears the error state in response).
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {ErrorBar} from './ErrorBar'

afterEach(cleanup)

describe('ErrorBar', () => {
    it('renders nothing when there is no message', () => {
        const {container} = render(<ErrorBar message="" onDismiss={vi.fn()}/>)
        expect(container.firstChild).toBeNull()
    })

    it('announces the message as an alert', () => {
        render(<ErrorBar message="1 of 1 messages could not be moved" onDismiss={vi.fn()}/>)
        expect(screen.getByRole('alert').textContent).toContain('1 of 1 messages could not be moved')
    })

    it('fires onDismiss from the dismiss button', () => {
        const onDismiss = vi.fn()
        render(<ErrorBar message="boom" onDismiss={onDismiss}/>)
        fireEvent.click(screen.getByRole('button', {name: 'Dismiss error'}))
        expect(onDismiss).toHaveBeenCalledTimes(1)
    })
})
