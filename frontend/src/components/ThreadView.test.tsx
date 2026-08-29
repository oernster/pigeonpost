// The thread view: one conversation shown whole in the reader, gathered across the account's folders. Both
// ../api seams are mocked (the conversation lookup and the per-message body fetch), because what is under
// test is the surface, not the store behind it.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import {ThreadView} from './ThreadView'
import type {ConversationEntry, Message} from '../api'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({conversation: vi.fn(), messageBody: vi.fn(), openExternal: vi.fn()}))
// The mock is built from the real api rather than hand-listed here, so a method reached with no spy
// fails the test by name instead of throwing a TypeError into the nearest catch and passing. The
// afterEach below reports any that were reached. See src/test/apiMock.ts.
const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

function makeMessage(id: string, overrides: Partial<Message> = {}): Message {
    return {
        id, folderId: 'f1', accountId: 'a1', snoozedUntilMs: 0, subject: 'Lunch on Friday',
        fromName: 'Bob Smith', fromAddress: 'bob@example.com', to: [], cc: [],
        date: '2026-08-28T09:00:00.000Z', size: 1, read: true, flagged: false, hasAttachments: false,
        answered: false, forwarded: false, snippet: 'the snippet', tagColours: [], ...overrides,
    } as Message
}

function entry(id: string, folderName: string, folderKind: string, overrides: Partial<Message> = {}): ConversationEntry {
    return {message: makeMessage(id, overrides), folderName, folderKind} as ConversationEntry
}

const THREAD = [
    entry('m1', 'Inbox', 'inbox'),
    entry('m2', 'Sent', 'sent', {
        subject: 'Re: Lunch on Friday',
        to: [{name: 'Bob Smith', address: 'bob@example.com'}],
        date: '2026-08-28T10:00:00.000Z',
    }),
    entry('m3', 'Inbox', 'inbox', {subject: 'Re: Lunch on Friday', date: '2026-08-28T11:00:00.000Z'}),
]

function renderView(overrides: Record<string, unknown> = {}) {
    const props = {
        headMessageId: 'm1',
        subject: 'Lunch on Friday',
        onClose: vi.fn(),
        onOpenMessage: vi.fn(),
        autoLoadImages: false,
        dark: false,
        ...overrides,
    }
    return {props, ...render(<ThreadView {...props}/>)}
}

beforeEach(() => {
    apiSpies.conversation.mockReset().mockResolvedValue(THREAD)
    apiSpies.messageBody.mockReset().mockResolvedValue({plain: 'the whole message', html: '', hasInvite: false, attachments: []})
    apiSpies.openExternal.mockReset()
})

afterEach(() => {
    cleanup()
    // Unmounting can reach the api too, so this is read after cleanup rather than before it.
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

describe('ThreadView', () => {
    it('shows every message of the conversation, naming the folder each one lives in', async () => {
        renderView()
        await waitFor(() => expect(screen.getByText('3 messages')).toBeTruthy())
        // The reply that lives in Sent is the whole point: the reading list can never show it beside the
        // message it answers.
        expect(screen.getByText('Sent')).toBeTruthy()
        expect(screen.getAllByText('Inbox')).toHaveLength(2)
    })

    it('names the party a message is with rather than always the sender', async () => {
        renderView()
        await waitFor(() => expect(screen.getByText('3 messages')).toBeTruthy())
        // Every row reads as being with Bob: the two received from him, plus the one in Sent named by
        // its recipient, which would otherwise read as being from you.
        expect(screen.getAllByText('Bob Smith')).toHaveLength(3)
    })

    it('opens the newest message for reading and leaves the rest closed', async () => {
        const {container} = renderView()
        await waitFor(() => expect(apiSpies.messageBody).toHaveBeenCalled())
        expect(apiSpies.messageBody).toHaveBeenCalledWith('m3')
        expect(apiSpies.messageBody).not.toHaveBeenCalledWith('m1')
        await waitFor(() => expect(container.querySelector('.reader-text')?.textContent).toContain('the whole message'))
    })

    it('fetches a message body only when its row is opened', async () => {
        const {container} = renderView()
        await waitFor(() => expect(screen.getByText('3 messages')).toBeTruthy())
        expect(apiSpies.messageBody).not.toHaveBeenCalledWith('m1')
        fireEvent.click(screen.getAllByRole('button', {expanded: false})[0])
        await waitFor(() => expect(apiSpies.messageBody).toHaveBeenCalledWith('m1'))
        // Both messages then read in place, which is the point of the surface: the received message and
        // the answer sent back, open together.
        await waitFor(() => expect(container.querySelectorAll('.reader-text')).toHaveLength(2))
    })

    it('closes a message again without losing the thread', async () => {
        const {container} = renderView()
        await waitFor(() => expect(container.querySelector('.reader-text')).not.toBeNull())
        fireEvent.click(screen.getByRole('button', {expanded: true}))
        await waitFor(() => expect(container.querySelector('.reader-text')).toBeNull())
        expect(screen.getByText('3 messages')).toBeTruthy()
    })

    it('lifts a message out into a reader tab of its own', async () => {
        const {props} = renderView()
        await waitFor(() => expect(screen.getByText('3 messages')).toBeTruthy())
        fireEvent.click(screen.getAllByRole('button', {name: 'Open'})[0])
        expect(props.onOpenMessage).toHaveBeenCalledTimes(1)
        expect(props.onOpenMessage.mock.calls[0][0].id).toBe('m1')
    })

    it('goes back to where it was opened from', async () => {
        const {props} = renderView()
        await waitFor(() => expect(screen.getByText('3 messages')).toBeTruthy())
        fireEvent.click(screen.getByRole('button', {name: 'Back'}))
        expect(props.onClose).toHaveBeenCalledTimes(1)
    })

    it('says so plainly when the cache holds nothing else of the conversation', async () => {
        apiSpies.conversation.mockResolvedValue([])
        renderView()
        await waitFor(() => expect(screen.getByText(/no other messages in the cache/)).toBeTruthy())
    })

    it('labels the thread while it is still being gathered', () => {
        apiSpies.conversation.mockReturnValue(new Promise(() => undefined))
        renderView()
        expect(screen.getByText('Lunch on Friday')).toBeTruthy()
        expect(screen.getByText(/Gathering the conversation/)).toBeTruthy()
    })

    it('survives a body that fails to load', async () => {
        apiSpies.messageBody.mockRejectedValue(new Error('gone'))
        renderView()
        await waitFor(() => expect(screen.getByText('3 messages')).toBeTruthy())
        // The row stays open and falls back to the stored snippet rather than tearing the thread down.
        await waitFor(() => expect(screen.getAllByText('the snippet').length).toBeGreaterThan(0))
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
