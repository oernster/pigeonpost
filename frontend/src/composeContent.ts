// ComposeContent is everything a compose window holds that would be lost if it were thrown away: the
// address fields and subject as typed; the body as both text and HTML; how many attachments of any
// kind it carries (picked files, pasted files and attached messages alike).
export interface ComposeContent {
    to: string
    cc: string
    bcc: string
    subject: string
    bodyText: string
    bodyHtml: string
    attachmentCount: number
}

// inlineImageRe finds an image embedded in the editor HTML. A pasted or dropped picture has no text of
// its own, so a body holding only an image reads as empty to getText and must be found in the HTML.
const inlineImageRe = /<img\b/i

// hasComposedContent reports whether a compose holds anything worth confirming before it is discarded.
// Files and images count as much as words: a compose holding nothing but an attachment or a pasted
// picture once closed on Cancel without asking. The files went with it.
export function hasComposedContent(c: ComposeContent): boolean {
    return c.to.trim() !== '' || c.cc.trim() !== '' || c.bcc.trim() !== '' || c.subject.trim() !== '' ||
        c.bodyText.trim() !== '' || inlineImageRe.test(c.bodyHtml) || c.attachmentCount > 0
}
