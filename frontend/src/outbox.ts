import {Message, OutboxItem} from './api'

// OUTBOX_FOLDER_ID is the synthetic folder id for an account's queue of unsent mail. It is not a real
// server folder: the app shows it in the sidebar only while the account has queued items, and maps
// those items to message rows under it.
export const OUTBOX_FOLDER_ID = '__outbox__'

// isOutboxMessage reports whether a message row represents a queued outbox item rather than real mail.
export function isOutboxMessage(message: Message): boolean {
    return message.folderId === OUTBOX_FOLDER_ID
}

// snippetLimit caps the outbox row's preview text, matching the stored-snippet scale of real mail rows.
const snippetLimit = 200

// outboxItemToMessage maps a queued item to the message shape the list and reader render. The sender
// column shows the recipients (this is outgoing mail), and the plain body doubles as the snippet. A
// held item (an undo window or a scheduled send-later) leads its snippet with when it sends, so the
// Outbox states the schedule at a glance. A permanently failed item is marked so it does not read as
// merely waiting: the subject is prefixed and the snippet leads with the failure reason, so the user
// sees it did not send and why.
export function outboxItemToMessage(item: OutboxItem): Message {
    const recipients = item.to.join(', ')
    const preview = item.body.replace(/\s+/g, ' ').trim()
    const sendsAt = item.holdMs > 0 ? `Sends ${new Date(item.holdMs).toLocaleString()}. ` : ''
    const snippet = item.failed
        ? `Failed to send: ${item.failure}`.slice(0, snippetLimit)
        : (sendsAt + preview).slice(0, snippetLimit)
    return {
        id: item.id,
        folderId: OUTBOX_FOLDER_ID,
        accountId: '',
        subject: item.failed ? `(Not sent) ${item.subject}` : item.subject,
        fromName: recipients ? `To: ${recipients}` : '(no recipient)',
        fromAddress: item.to[0] ?? '',
        to: item.to.map((address) => ({name: '', address})),
        cc: [],
        date: new Date(item.createdMs).toISOString(),
        size: 0,
        read: true,
        flagged: false,
        hasAttachments: false,
        // A queued outgoing message is never itself replied-to or forwarded.
        answered: false,
        forwarded: false,
        snippet,
        tagColours: [],
        snoozedUntilMs: 0,
    }
}
