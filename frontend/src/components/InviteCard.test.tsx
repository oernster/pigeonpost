// InviteCard behaviour at its outer interface: it fetches the invitation for the message, offers the
// REQUEST actions and, after a response, refetches so the attendee list shows the recorded statuses
// (the backend overlays them from the stored meeting) instead of the email's frozen ICS. ../api is
// stubbed (the one Wails seam).
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import {InviteCard} from './InviteCard'
import type {Invitation} from '../api'

const apiSpies = vi.hoisted(() => ({
    getInvitation: vi.fn(),
    respondToInvitation: vi.fn(),
    removeCancelledMeeting: vi.fn(),
    applyMeetingReply: vi.fn(),
}))

vi.mock('../api', () => ({api: apiSpies}))

function makeInvitation(status: string): Invitation {
    return {
        method: 'REQUEST',
        me: 'me@example.com',
        myStatus: status,
        organizer: {address: 'chair@example.com', commonName: 'Chair'},
        attendees: [
            {address: 'me@example.com', commonName: '', role: 'REQ-PARTICIPANT', status, rsvp: true},
            {address: 'other@example.com', commonName: '', role: 'REQ-PARTICIPANT', status: 'NEEDS-ACTION', rsvp: true},
        ],
        event: {
            id: 'evt1', uid: 'uid1', summary: 'Sync', description: '', location: '',
            start: '2026-08-05T14:00:00Z', end: '2026-08-05T14:30:00Z',
        },
    } as unknown as Invitation
}

afterEach(() => {
    cleanup()
    vi.clearAllMocks()
})

describe('InviteCard', () => {
    it('shows the invitation with each attendee status', async () => {
        apiSpies.getInvitation.mockResolvedValue(makeInvitation('NEEDS-ACTION'))
        render(<InviteCard messageId="m1"/>)

        await screen.findByText('Sync')
        expect(screen.getAllByText('(No response yet)').length).toBe(2)
        expect(apiSpies.getInvitation).toHaveBeenCalledWith('m1')
    })

    it('refetches after responding so the recorded statuses replace the frozen ICS', async () => {
        apiSpies.getInvitation.mockResolvedValueOnce(makeInvitation('NEEDS-ACTION'))
        apiSpies.respondToInvitation.mockResolvedValue(undefined)
        apiSpies.getInvitation.mockResolvedValueOnce(makeInvitation('ACCEPTED'))
        const onActed = vi.fn()
        render(<InviteCard messageId="m1" onActed={onActed}/>)

        fireEvent.click(await screen.findByText('Accept'))

        await waitFor(() => expect(apiSpies.respondToInvitation).toHaveBeenCalledWith('m1', 'ACCEPTED'))
        await screen.findByText('(Accepted)')
        expect(apiSpies.getInvitation).toHaveBeenCalledTimes(2)
        expect(onActed).toHaveBeenCalled()
        expect(screen.getByText('Your response: Accepted')).toBeTruthy()
    })

    it('still records the response when the refetch fails', async () => {
        apiSpies.getInvitation.mockResolvedValueOnce(makeInvitation('NEEDS-ACTION'))
        apiSpies.respondToInvitation.mockResolvedValue(undefined)
        apiSpies.getInvitation.mockRejectedValueOnce(new Error('offline'))
        render(<InviteCard messageId="m1"/>)

        fireEvent.click(await screen.findByText('Decline'))

        await screen.findByText('Your response: Declined')
        expect(screen.queryByText(/Something went wrong/)).toBeNull()
    })
})
