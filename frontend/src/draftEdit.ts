// draftEdit holds the pure builder that turns a draft stored in the server Drafts mailbox back into
// compose fields, so a saved draft can be reopened and finished. No React, no api, so it is unit-tested
// in isolation.
import {escapeHtml} from './messageText'
import type {Message, MessageBody} from './api'

// DraftEdit is the subset of compose fields reopening a stored draft pre-fills. It is structurally a
// subset of ComposeModal's ComposeInitial, so the launcher passes it straight to setComposeInitial.
export interface DraftEdit {
    to: string
    cc: string
    subject: string
    bodyHtml: string
}

// addressList joins a stored draft's recipients as bare addresses, the same shape buildReply and
// buildReplyAll produce. Display names are dropped deliberately: a name containing a comma would split
// into two recipients and the "Name <address>" form fails the composer's own address check.
function addressList(list: {address: string}[] | null | undefined): string {
    return (list ?? []).map((a) => a.address.trim()).filter(Boolean).join(', ')
}

// draftBodyHtml is the saved draft's body as compose HTML: the stored HTML alternative when the draft has
// one, otherwise its plain text escaped and wrapped with its line breaks preserved. Nothing is added: the
// signature and any quoted original were already written into the text before it was saved, so seeding
// them again would duplicate them on every reopen.
function draftBodyHtml(body: MessageBody | null): string {
    if (body?.html && body.html.trim() !== '') {
        return body.html
    }
    const plain = body?.plain ?? ''
    if (plain.trim() === '') {
        return ''
    }
    return `<p>${plain.split(/\r?\n/).map(escapeHtml).join('<br>')}</p>`
}

// buildDraftEdit reopens a saved draft exactly as it was stored: its recipients, its subject and its body.
// Bcc is not recovered; a saved draft carries no Bcc header, so a Bcc typed before saving has to be typed
// again. Everything else round-trips.
export function buildDraftEdit(message: Message, body: MessageBody | null): DraftEdit {
    return {
        to: addressList(message.to),
        cc: addressList(message.cc),
        subject: message.subject,
        bodyHtml: draftBodyHtml(body),
    }
}
