import {Message, OutboxItem} from './api'

// OUTBOX_FOLDER_ID is the synthetic folder id for an account's queue of unsent mail. It is not a real
// server folder: the app shows it in the sidebar only while the account has queued items, and maps
// those items to message rows under it.
export const OUTBOX_FOLDER_ID = '__outbox__'

// isOutboxMessage reports whether a message row represents a queued outbox item rather than real mail.
export function isOutboxMessage(message: Message): boolean {
    return message.folderId === OUTBOX_FOLDER_ID
}

// outboxItemToMessage maps a queued item to the message shape the list and reader render. The sender
// column shows the recipients (this is outgoing mail), and the plain body doubles as the snippet.
export function outboxItemToMessage(item: OutboxItem): Message {
    const recipients = item.to.join(', ')
    const preview = item.body.replace(/\s+/g, ' ').trim()
    return {
        id: item.id,
        folderId: OUTBOX_FOLDER_ID,
        subject: item.subject,
        fromName: recipients ? `To: ${recipients}` : '(no recipient)',
        fromAddress: item.to[0] ?? '',
        to: item.to.map((address) => ({name: '', address})),
        cc: [],
        date: new Date(item.createdMs).toISOString(),
        size: 0,
        read: true,
        flagged: false,
        hasAttachments: false,
        snippet: preview.slice(0, 200),
    }
}
