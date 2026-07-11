import {escapeHtml, subjectWithPrefix} from './messageText'
import type {Account, Message, MessageBody} from './api'

// ReplyDraft is the subset of compose fields a reply, reply-all or forward pre-fills. It is structurally a
// subset of ComposeModal's ComposeInitial, so App passes it straight to setComposeInitial.
export interface ReplyDraft {
    from?: string
    to?: string
    cc?: string
    subject?: string
    bodyHtml?: string
}

// ReplyContext is the account- and body-derived data a reply or reply-all needs, computed by the caller from
// its state (the reply-from address, the account signature and the quoted original) and passed in so these
// builders stay pure.
export interface ReplyContext {
    from: string
    signatureHtml: string
    quotedHtml: string
}

// sendersFor returns the addresses an account may send from: its primary address first, then its identities.
// The compose window offers these in its From dropdown.
export function sendersFor(account?: Account): {name: string; address: string}[] {
    return account ? [{name: account.displayName, address: account.email}, ...account.identities] : []
}

// signatureHtmlFor is the account's signature as HTML, inserted into a new message and above the quoted text
// on a reply or forward. Empty when the account has no signature, so nothing is added.
export function signatureHtmlFor(account?: Account): string {
    return account?.signature ?? ''
}

// quoteFor returns the quoted original for reply/forward: the fetched HTML body when available, otherwise the
// plain text (or snippet) escaped into a paragraph.
export function quoteFor(message: Message, body: MessageBody | null): string {
    if (body?.html && body.html.trim() !== '') {
        return body.html
    }
    return `<p>${escapeHtml(body?.plain || message.snippet || '')}</p>`
}

// replyFromAddress picks which of the account's own addresses a reply should be sent from: the one the
// original message was delivered to (its To or Cc), so a message to an alias is answered as that alias. It
// returns empty (the primary) when none of the account's addresses received it.
export function replyFromAddress(message: Message, senders: {address: string}[]): string {
    const mine = new Set(senders.map((s) => s.address.toLowerCase()))
    const hit = [...(message.to || []), ...(message.cc || [])].find((a) => mine.has(a.address.toLowerCase()))
    return hit ? hit.address : ''
}

// attributionLine is the "On <date>, <who> wrote:" line above the quoted text on a reply. The date is dropped
// when the message has none.
function attributionLine(message: Message): string {
    const when = message.date ? new Date(message.date).toLocaleString() : ''
    const who = message.fromName || message.fromAddress || 'the sender'
    return when ? `On ${when}, ${who} wrote:` : `${who} wrote:`
}

// buildReply constructs the compose fields for a reply to message: the sender in To, a Re: subject, and the
// body with the signature above the quoted original under the attribution line.
export function buildReply(message: Message, ctx: ReplyContext): ReplyDraft {
    const header = attributionLine(message)
    return {
        from: ctx.from,
        to: message.fromAddress,
        subject: subjectWithPrefix('Re:', message.subject),
        bodyHtml: `<p></p>${ctx.signatureHtml}<p>${escapeHtml(header)}</p><blockquote>${ctx.quotedHtml}</blockquote>`,
    }
}

// buildReplyAll addresses the sender plus everyone on the original To and Cc, dropping our own address
// (selfAddress) and any duplicates so we never reply to ourselves or twice to the same person.
export function buildReplyAll(message: Message, selfAddress: string, ctx: ReplyContext): ReplyDraft {
    const header = attributionLine(message)
    const seen = new Set<string>([selfAddress.toLowerCase()])
    const collect = (address: string, into: string[]) => {
        const key = address.trim().toLowerCase()
        if (key !== '' && !seen.has(key)) {
            seen.add(key)
            into.push(address.trim())
        }
    }
    const toList: string[] = []
    const ccList: string[] = []
    collect(message.fromAddress, toList)
    ;(message.to || []).forEach((a) => collect(a.address, toList))
    ;(message.cc || []).forEach((a) => collect(a.address, ccList))
    return {
        from: ctx.from,
        to: toList.join(', '),
        cc: ccList.join(', '),
        subject: subjectWithPrefix('Re:', message.subject),
        bodyHtml: `<p></p>${ctx.signatureHtml}<p>${escapeHtml(header)}</p><blockquote>${ctx.quotedHtml}</blockquote>`,
    }
}

// buildForward constructs a Fwd: with the original quoted under a forwarded-message header. It needs no
// reply-from address (a forward is sent from the current identity).
export function buildForward(message: Message, ctx: {signatureHtml: string; quotedHtml: string}): ReplyDraft {
    const who = message.fromName || message.fromAddress || 'unknown sender'
    return {
        to: '',
        subject: subjectWithPrefix('Fwd:', message.subject),
        bodyHtml:
            `<p></p>${ctx.signatureHtml}<p>---------- Forwarded message ----------</p>` +
            `<p>From: ${escapeHtml(who)}<br>Subject: ${escapeHtml(message.subject || '(no subject)')}</p>` +
            `<blockquote>${ctx.quotedHtml}</blockquote>`,
    }
}
