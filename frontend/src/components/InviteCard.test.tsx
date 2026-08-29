// InviteCard behaviour at its outer interface: it fetches the invitation for the message, offers the
// REQUEST actions then refetches after a response, so the attendee list shows the recorded statuses
// (the backend overlays them from the stored meeting) instead of the email's frozen ICS. ../api is
// stubbed (the one Wails seam).
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import {InviteCard} from './InviteCard'
import type {Invitation} from '../api'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({
    getInvitation: vi.fn(),
    respondToInvitation: vi.fn(),
    removeCancelledMeeting: vi.fn(),
    applyMeetingReply: vi.fn(),
}))

// The mock is built from the real api rather than hand-listed here, so a method reached with no spy
// fails the test by name instead of throwing a TypeError into the nearest catch and passing. The
// afterEach below reports any that were reached. See src/test/apiMock.ts.
const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

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
    // Unmounting can reach the api too, so this is read after cleanup rather than before it.
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
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

    it('warns instead of resending when the same answer is clicked again', async () => {
        apiSpies.getInvitation.mockResolvedValue(makeInvitation('ACCEPTED'))
        render(<InviteCard messageId="m1"/>)

        fireEvent.click(await screen.findByText('Accept'))

        await screen.findByText(/sending it\s+again is unnecessary/)
        expect(apiSpies.respondToInvitation).not.toHaveBeenCalled()

        fireEvent.click(screen.getByText('Cancel'))
        expect(screen.queryByText(/unnecessary/)).toBeNull()
        expect(apiSpies.respondToInvitation).not.toHaveBeenCalled()
    })

    it('sends again once the warning is confirmed', async () => {
        apiSpies.getInvitation.mockResolvedValue(makeInvitation('ACCEPTED'))
        apiSpies.respondToInvitation.mockResolvedValue(undefined)
        render(<InviteCard messageId="m1"/>)

        fireEvent.click(await screen.findByText('Accept'))
        fireEvent.click(await screen.findByText('Send again'))

        await waitFor(() => expect(apiSpies.respondToInvitation).toHaveBeenCalledWith('m1', 'ACCEPTED'))
    })

    it('sends a changed answer immediately without warning', async () => {
        apiSpies.getInvitation.mockResolvedValue(makeInvitation('ACCEPTED'))
        apiSpies.respondToInvitation.mockResolvedValue(undefined)
        render(<InviteCard messageId="m1"/>)

        fireEvent.click(await screen.findByText('Decline'))

        await waitFor(() => expect(apiSpies.respondToInvitation).toHaveBeenCalledWith('m1', 'DECLINED'))
        expect(screen.queryByText(/unnecessary/)).toBeNull()
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

// The mock covers the api in both directions: the afterEach above catches a method reached with no
// spy; this catches the opposite, a spy declared under a name the api does not have, which binds to
// nothing, so every test configuring it would be configuring a stub the code can never call.
describe('the api mock', () => {
    it('declares no spy the real api does not have', async () => {
        const actual = await vi.importActual<typeof import('../api')>('../api')
        expect(spiesNotInApi(actual, apiSpies as unknown as Record<string, unknown>)).toEqual([])
    })
})
