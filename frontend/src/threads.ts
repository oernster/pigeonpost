import {Message} from './api'

// This is the desktop list's live mirror of the conversation grouping. The canonical, tested
// implementation is the Go domain GroupThreads (exposed as the ListThreads API); this client copy drives
// the reading list so grouping updates instantly with optimistic changes (mark read, delete, star)
// without a round-trip. Both key a conversation by its subject with reply and forward prefixes removed.

const replyPrefixes = ['re', 'fwd', 'fw', 'aw', 'sv', 'vs', 'antw']

// skipReplyCount drops a leading "[n]" or "(n)" count some clients add to a reply prefix (Re[2]:).
function skipReplyCount(s: string): string {
    if (s === '') {
        return s
    }
    const open = s[0]
    if (open !== '[' && open !== '(') {
        return s
    }
    const close = open === '[' ? ']' : ')'
    const end = s.indexOf(close)
    if (end < 0) {
        return s
    }
    if (!/^[0-9]*$/.test(s.slice(1, end))) {
        return s
    }
    return s.slice(end + 1)
}

// stripOneReplyPrefix removes a single leading reply or forward marker, or returns the input unchanged.
function stripOneReplyPrefix(s: string): string {
    for (const prefix of replyPrefixes) {
        if (s.length >= prefix.length && s.slice(0, prefix.length).toLowerCase() === prefix) {
            const rest = skipReplyCount(s.slice(prefix.length))
            if (rest.startsWith(':')) {
                return rest.slice(1)
            }
        }
    }
    return s
}

// normaliseSubject lowercases a subject, strips every leading reply or forward prefix, and collapses
// whitespace, so subjects differing only by those prefixes and spacing thread together.
export function normaliseSubject(subject: string): string {
    let s = subject.trim()
    for (;;) {
        const stripped = stripOneReplyPrefix(s).trim()
        if (stripped === s) {
            break
        }
        s = stripped
    }
    return s.toLowerCase().split(/\s+/).filter(Boolean).join(' ')
}

// ConversationHead labels the first row of a multi-message conversation with its display subject and size.
export interface ConversationHead {
    subject: string
    count: number
}

export interface ConversationView {
    // ordered is every message flattened conversation by conversation: newest conversation first, and
    // within a conversation oldest message first, so a thread reads top to bottom.
    ordered: Message[]
    // heads maps the id of a conversation's first row to its label, for conversations of two or more.
    heads: Map<string, ConversationHead>
}

function dateMs(m: Message): number {
    const t = new Date(m.date).getTime()
    return isNaN(t) ? 0 : t
}

// arrangeByConversation groups messages into conversations by normalised subject and flattens them for
// the list, alongside the header labels for multi-message conversations.
export function arrangeByConversation(messages: Message[]): ConversationView {
    const order: string[] = []
    const groups = new Map<string, Message[]>()
    for (const m of messages) {
        const key = normaliseSubject(m.subject)
        if (!groups.has(key)) {
            order.push(key)
            groups.set(key, [])
        }
        groups.get(key)!.push(m)
    }
    const threads = order.map((key) => groups.get(key)!.slice().sort((a, b) => dateMs(a) - dateMs(b)))
    threads.sort((a, b) => dateMs(b[b.length - 1]) - dateMs(a[a.length - 1]))

    const ordered: Message[] = []
    const heads = new Map<string, ConversationHead>()
    for (const group of threads) {
        if (group.length > 1) {
            const latest = group[group.length - 1]
            heads.set(group[0].id, {subject: latest.subject || '(no subject)', count: group.length})
        }
        ordered.push(...group)
    }
    return {ordered, heads}
}
