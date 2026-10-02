// The Outbox row built from a queued item. It once always reported no attachments, so a scheduled send
// that had lost its file looked exactly like one that still carried it.
import {describe, expect, it} from 'vitest'
import type {OutboxItem} from './api'
import {outboxItemToMessage} from './outbox'

function makeItem(overrides: Partial<OutboxItem> = {}): OutboxItem {
    return {
        id: 'o1', accountId: 'acct', kind: 'send', subject: 'amusement', to: ['peter@example.com'],
        body: 'See attached', createdMs: 0, holdMs: 0, failed: false, failure: '', attachments: [],
        ...overrides,
    } as OutboxItem
}

describe('outboxItemToMessage: attachments', () => {
    it('shows the paperclip when the queued message carries a file', () => {
        expect(outboxItemToMessage(makeItem({attachments: ['photo.jpg']})).hasAttachments).toBe(true)
    })

    it('shows none when the queued message carries no file', () => {
        expect(outboxItemToMessage(makeItem()).hasAttachments).toBe(false)
    })
})
