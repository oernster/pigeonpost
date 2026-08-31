// SnoozeNotifications is the in-window announcement that a snoozed message has come back. It exists
// because the desktop notification is not a reliable one: the Windows shell can suppress a balloon for
// reasons the app cannot see, which left an expiring snooze showing the user nothing at all. These tests
// pin that the toast appears on the backend event, names the message, opens it on click and dismisses.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, fireEvent, render, screen} from '@testing-library/react'
import {SnoozeNotifications} from './SnoozeNotifications'
import type {Message} from '../api'

// listeners holds the handler EventsOn was given, keyed by event name, so a test can fire the backend
// event the component subscribes to.
const listeners = vi.hoisted(() => new Map<string, (payload: unknown) => void>())
const offSpy = vi.hoisted(() => vi.fn())

vi.mock('../../wailsjs/runtime', () => ({
    EventsOn: (event: string, handler: (payload: unknown) => void) => {
        listeners.set(event, handler)
        return offSpy
    },
}))

function makeMessage(overrides: Partial<Message> = {}): Message {
    return {
        id: 'm1', folderId: 'f1', accountId: 'a1',
        subject: 'Roof quote', fromName: 'Alice Smith', fromAddress: 'alice@example.com',
        to: [], cc: [], date: '', size: 0,
        read: false, flagged: false, hasAttachments: false,
        answered: false, forwarded: false, snippet: '',
        tagColours: [], snoozedUntilMs: 0,
        ...overrides,
    } as Message
}

// emit fires the backend's resurfaced event with the given payload.
function emit(payload: unknown) {
    act(() => {
        listeners.get('snooze:resurfaced')?.(payload)
    })
}

describe('SnoozeNotifications', () => {
    beforeEach(() => {
        listeners.clear()
        offSpy.mockClear()
    })
    afterEach(cleanup)

    it('shows nothing until a message comes back', () => {
        const {container} = render(<SnoozeNotifications onOpen={vi.fn()}/>)
        expect(container.firstChild).toBeNull()
        expect(listeners.has('snooze:resurfaced')).toBe(true)
    })

    it('raises one toast per returned message, naming each', () => {
        render(<SnoozeNotifications onOpen={vi.fn()}/>)
        emit([makeMessage(), makeMessage({id: 'm2', subject: 'Second', fromName: 'Bob'})])

        expect(screen.getAllByRole('alert')).toHaveLength(2)
        expect(screen.getByText('Roof quote')).toBeTruthy()
        expect(screen.getByText('Alice Smith')).toBeTruthy()
        expect(screen.getByText('Second')).toBeTruthy()
        expect(screen.getAllByText('Snoozed message is back')).toHaveLength(2)
    })

    it('keeps earlier toasts when a later batch arrives', () => {
        render(<SnoozeNotifications onOpen={vi.fn()}/>)
        emit([makeMessage()])
        emit([makeMessage({id: 'm2', subject: 'Second'})])
        expect(screen.getAllByRole('alert')).toHaveLength(2)
    })

    it('opens the message it names when the toast is clicked, then drops the toast', () => {
        const onOpen = vi.fn()
        render(<SnoozeNotifications onOpen={onOpen}/>)
        emit([makeMessage()])

        fireEvent.click(screen.getByTitle('Open the message'))
        expect(onOpen).toHaveBeenCalledTimes(1)
        expect(onOpen.mock.calls[0][0].id).toBe('m1')
        expect(screen.queryByRole('alert')).toBeNull()
    })

    it('dismisses without opening', () => {
        const onOpen = vi.fn()
        render(<SnoozeNotifications onOpen={onOpen}/>)
        emit([makeMessage()])

        fireEvent.click(screen.getByLabelText('Dismiss returned message'))
        expect(onOpen).not.toHaveBeenCalled()
        expect(screen.queryByRole('alert')).toBeNull()
    })

    it('labels a message with no subject rather than showing a blank line', () => {
        render(<SnoozeNotifications onOpen={vi.fn()}/>)
        emit([makeMessage({subject: ''})])
        expect(screen.getByText('(no subject)')).toBeTruthy()
    })

    it('falls back to the sender address when the sender has no display name', () => {
        render(<SnoozeNotifications onOpen={vi.fn()}/>)
        emit([makeMessage({fromName: ''})])
        expect(screen.getByText('alice@example.com')).toBeTruthy()
    })

    // A payload the backend could not fill must not take the window down with it: the event carries an
    // array; anything else is treated as nothing to show.
    it('survives an empty or absent payload', () => {
        render(<SnoozeNotifications onOpen={vi.fn()}/>)
        emit([])
        emit(undefined)
        expect(screen.queryByRole('alert')).toBeNull()
    })

    it('unsubscribes from the backend event when it unmounts', () => {
        const {unmount} = render(<SnoozeNotifications onOpen={vi.fn()}/>)
        unmount()
        expect(offSpy).toHaveBeenCalled()
    })
})
