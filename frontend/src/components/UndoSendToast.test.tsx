// The undo-send toast counts the hold window down, offers Undo while it lasts and reports expiry once,
// after which it renders nothing: the message itself is sent by the backend dispatcher.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, fireEvent, render, screen} from '@testing-library/react'
import {UndoSendToast} from './UndoSendToast'

beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(0)
})

afterEach(() => {
    cleanup()
    vi.useRealTimers()
})

describe('UndoSendToast', () => {
    it('shows the remaining seconds and counts down', () => {
        render(<UndoSendToast expiresAt={10_000} onUndo={() => {}} onExpired={() => {}}/>)
        expect(screen.getByText('Sending in 10s')).toBeInTheDocument()
        act(() => {
            vi.advanceTimersByTime(3_000)
        })
        expect(screen.getByText('Sending in 7s')).toBeInTheDocument()
    })

    it('fires onUndo when Undo is clicked', () => {
        const onUndo = vi.fn()
        render(<UndoSendToast expiresAt={10_000} onUndo={onUndo} onExpired={() => {}}/>)
        fireEvent.click(screen.getByRole('button', {name: 'Undo'}))
        expect(onUndo).toHaveBeenCalledTimes(1)
    })

    it('reports expiry once and disappears', () => {
        const onExpired = vi.fn()
        render(<UndoSendToast expiresAt={2_000} onUndo={() => {}} onExpired={onExpired}/>)
        act(() => {
            vi.advanceTimersByTime(2_500)
        })
        expect(onExpired).toHaveBeenCalledTimes(1)
        expect(screen.queryByText(/Sending in/)).toBeNull()
        act(() => {
            vi.advanceTimersByTime(2_000)
        })
        expect(onExpired).toHaveBeenCalledTimes(1)
    })
})
