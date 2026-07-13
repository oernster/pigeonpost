// composeAttachment holds the pure logic behind the "did you forget an attachment?" reminder, so it is
// unit-tested away from the compose component and the TipTap editor.

// attachmentWord matches the whole words "attach" or "attached" (not "attachment" or "attaching"), the
// phrasing that signals an intended attachment.
const attachmentWord = /\battach(ed)?\b/i

// bodyMentionsAttachment reports whether the message the user actually wrote mentions an attachment. It
// scans the subject and the body with any quoted reply or forward removed, because a reply within a thread
// whose earlier messages mentioned an attachment must not inherit that mention: only the sender's own words
// count. The quoted original is a <blockquote> (see replyDraft), so every blockquote, nested ones included,
// is stripped before the scan. bodyHtml is the editor's HTML (not its plain text) so the blockquotes are
// still identifiable.
export function bodyMentionsAttachment(subject: string, bodyHtml: string): boolean {
    const doc = new DOMParser().parseFromString(bodyHtml, 'text/html')
    doc.querySelectorAll('blockquote').forEach((quote) => quote.remove())
    const composed = doc.body.textContent || ''
    return attachmentWord.test(subject + ' ' + composed)
}
