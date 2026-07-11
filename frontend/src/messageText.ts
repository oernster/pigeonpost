// messageText holds the pure text helpers for messages: HTML escaping, subject prefixing, safe .eml
// filenames and the post-removal selection neighbour. No React, no api runtime, so each is unit-tested
// in isolation.
import {Message} from './api'

export function escapeHtml(s: string): string {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

// subjectWithPrefix adds "Re:"/"Fwd:" unless the subject already starts with it.
export function subjectWithPrefix(prefix: string, subject: string): string {
    const s = subject || '(no subject)'
    return s.toLowerCase().startsWith(prefix.toLowerCase()) ? s : `${prefix} ${s}`
}

// emlFilename builds a safe .eml filename from a message subject, replacing characters a filesystem
// rejects and falling back to a default when the subject is empty.
export function emlFilename(subject: string): string {
    const cleaned = subject.replace(/[\\/:*?"<>|\x00-\x1f]/g, '-').trim()
    return `${cleaned || 'message'}.eml`
}

// neighbourAfterRemoval returns the message that selection should land on once the message with
// removedId is deleted from list: the following message, or the preceding one when it was last, or
// null when it was the only message. This keeps keyboard triage moving without a manual re-select.
export function neighbourAfterRemoval(list: Message[], removedId: string): Message | null {
    const idx = list.findIndex((m) => m.id === removedId)
    if (idx === -1) {
        return null
    }
    if (idx + 1 < list.length) {
        return list[idx + 1]
    }
    if (idx - 1 >= 0) {
        return list[idx - 1]
    }
    return null
}
