// The reader's conversation strip: the thread an open message belongs to, gathered across the account's
// folders so the answer you sent sits beside the message it answers. It renders from its props alone, so
// the tests drive it directly.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {ConversationStrip} from './ConversationStrip'
import type {ConversationEntry, Message} from '../api'

function makeMessage(id: string, overrides: Partial<Message> = {}): Message {
    return {
        id, folderId: 'f1', accountId: 'a1', snoozedUntilMs: 0, subject: 'Lunch on Friday',
        fromName: 'Bob Smith', fromAddress: 'bob@example.com', to: [], cc: [],
        date: '2026-08-28T09:00:00.000Z', size: 1, read: true, flagged: false, hasAttachments: false,
        answered: false, forwarded: false, snippet: '', tagColours: [], ...overrides,
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

afterEach(() => cleanup())

describe('ConversationStrip', () => {
    it('stays out of the way for a message that stands alone', () => {
        const {container} = render(
            <ConversationStrip entries={[THREAD[0]]} currentId="m1" onOpen={vi.fn()}/>,
        )
        expect(container.querySelector('.conversation-strip')).toBeNull()
    })

    it('lists the thread with its size, in order', () => {
        render(<ConversationStrip entries={THREAD} currentId="m1" onOpen={vi.fn()}/>)
        expect(screen.getByText('Conversation (3)')).toBeInTheDocument()
        const rows = screen.getAllByRole('button')
        expect(rows).toHaveLength(3)
        expect(rows[0].textContent).toContain('Inbox')
        expect(rows[1].textContent).toContain('Sent')
    })

    it('names the sender for a received message and the recipient for one you sent', () => {
        // Every row of your own Sent folder would otherwise read as you, which says nothing about the
        // message.
        render(<ConversationStrip entries={THREAD} currentId="m1" onOpen={vi.fn()}/>)
        const rows = screen.getAllByRole('button')
        expect(rows[0].textContent).toContain('Bob Smith')
        expect(rows[1].textContent).toContain('Bob Smith')
    })

    it('falls back to "you" on a sent message with no recipient', () => {
        const orphan = entry('m9', 'Sent', 'sent', {to: []})
        render(<ConversationStrip entries={[orphan, THREAD[0]]} currentId="m1" onOpen={vi.fn()}/>)
        expect(screen.getAllByRole('button')[0].textContent).toContain('you')
    })

    it('opens another message in the thread', () => {
        const onOpen = vi.fn()
        render(<ConversationStrip entries={THREAD} currentId="m1" onOpen={onOpen}/>)
        fireEvent.click(screen.getAllByRole('button')[2])
        expect(onOpen).toHaveBeenCalledWith(THREAD[2].message)
    })

    it('marks the message being read and offers no way to reopen it', () => {
        render(<ConversationStrip entries={THREAD} currentId="m1" onOpen={vi.fn()}/>)
        const rows = screen.getAllByRole('button')
        expect(rows[0]).toBeDisabled()
        expect(rows[0]).toHaveAttribute('aria-current', 'true')
        expect(rows[1]).toBeEnabled()
    })

    it('leaves the date blank rather than showing an unparsable one', () => {
        const broken = entry('m8', 'Inbox', 'inbox', {date: 'not a date'})
        render(<ConversationStrip entries={[broken, THREAD[0]]} currentId="m1" onOpen={vi.fn()}/>)
        expect(screen.getAllByRole('button')[0].querySelector('.conversation-when')?.textContent).toBe('')
    })
})
