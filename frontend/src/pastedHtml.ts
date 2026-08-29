// pastedHtml normalises the markup the clipboard hands over, before ProseMirror parses it.
//
// Windows carries HTML on the clipboard in the CF_HTML format, which a webview reconstructs as
// <html><body>(newline)<!--StartFragment-->...<!--EndFragment-->(newline)</body>(newline)</html>.
// Those newlines are text nodes sitting at block level; ProseMirror turns them into hard breaks
// and blank paragraphs: a signature copied from one account and pasted into another arrived with a
// blank line above it and two below. Markup that was pretty-printed between its block tags does the
// same thing between every line of the signature.
//
// Nothing here touches content. It removes the clipboard's own wrapper and the whitespace that both
// carries a newline and sits between blocks, which is markup layout rather than anything a document
// can render. Inline whitespace is left exactly as it arrived, so a space between two words or two
// marks survives; ProseMirror collapses that correctly on its own.

const FRAGMENT_START = '<!--StartFragment-->'
const FRAGMENT_END = '<!--EndFragment-->'

// The block-level tags whose boundary ends a line of the document. Whitespace against one of these
// is the shape of the markup, never content.
const BLOCK_TAGS =
    'p|div|ul|ol|li|blockquote|h[1-6]|table|thead|tbody|tfoot|tr|td|th|section|article|figure|pre|hr'

// Whitespace containing a newline directly after a block's closing tag.
const AFTER_BLOCK_CLOSE = new RegExp(`(</(?:${BLOCK_TAGS})>)\\s*[\\r\\n]\\s*(?=<)`, 'gi')

// Whitespace containing a newline directly after an opening block tag, <body> or <html>.
const AFTER_BLOCK_OPEN = new RegExp(`(<(?:html|body|${BLOCK_TAGS})(?:\\s[^>]*)?>)\\s*[\\r\\n]\\s*(?=<)`, 'gi')

// Whitespace containing a newline directly before a block's closing tag or before </body>.
const BEFORE_BLOCK_CLOSE = new RegExp(`\\s*[\\r\\n]\\s*(?=</(?:html|body|${BLOCK_TAGS})>)`, 'gi')

// normalisePastedHtml returns the pasted markup with the clipboard wrapper and its between-block
// newlines removed. Markup that carries neither comes back unchanged apart from a trim.
export function normalisePastedHtml(html: string): string {
    let markup = html
    const start = markup.indexOf(FRAGMENT_START)
    const end = markup.indexOf(FRAGMENT_END)
    if (start !== -1 && end > start) {
        markup = markup.slice(start + FRAGMENT_START.length, end)
    }
    return markup
        .replace(AFTER_BLOCK_CLOSE, '$1')
        .replace(AFTER_BLOCK_OPEN, '$1')
        .replace(BEFORE_BLOCK_CLOSE, '')
        .trim()
}
