import {emojiBeside} from './emoji'
import {emojiStatus, linkStatus, type HoverStatus} from './hoverStatus'

// hoverProbe reads what is under the pointer in a document: a link, else an emoji. It works on any
// document, the app's own or a message frame's, so a pointer event's coordinates are read against the
// document the event came from. Duck typing stands in for instanceof because an element inside a frame
// belongs to the frame's window, whose Element is not the app's.

const TEXT_NODE = 3

interface Caret {
    node: Node
    offset: number
}

// caretAt answers the text position under a point. caretPositionFromPoint is the standard; WebKit and
// older Chromium offer only caretRangeFromPoint. jsdom offers neither, so both are optional.
function caretAt(doc: Document, x: number, y: number): Caret | null {
    const standard = doc.caretPositionFromPoint?.(x, y)
    if (standard) {
        return {node: standard.offsetNode, offset: standard.offset}
    }
    const legacy = doc.caretRangeFromPoint?.(x, y)
    return legacy ? {node: legacy.startContainer, offset: legacy.startOffset} : null
}

// emojiAtPoint answers the emoji drawn under a point. The caret position is the nearest gap between
// characters, which may sit either side of the glyph the pointer is on, so each emoji touching that gap
// is measured and only one actually drawn under the point counts: a pointer in the margin beside a line
// ending in an emoji is not over it.
function emojiAtPoint(doc: Document, x: number, y: number): HoverStatus | null {
    const caret = caretAt(doc, x, y)
    if (!caret || caret.node.nodeType !== TEXT_NODE) {
        return null
    }
    for (const span of emojiBeside((caret.node as Text).data, caret.offset)) {
        const range = doc.createRange()
        range.setStart(caret.node, span.start)
        range.setEnd(caret.node, span.end)
        const box = range.getBoundingClientRect()
        if (x >= box.left && x <= box.right && y >= box.top && y <= box.bottom) {
            return emojiStatus(span.grapheme)
        }
    }
    return null
}

// hoverStatusAt answers the tray's reading for a pointer at (x, y) over target in doc.
export function hoverStatusAt(doc: Document, target: EventTarget | null, x: number, y: number): HoverStatus | null {
    const element = target as Element | null
    if (!element || typeof element.closest !== 'function') {
        return null
    }
    // The selector guarantees the href; an element's text is never null. Neither needs a fallback.
    const anchor = element.closest('a[href]')
    if (anchor) {
        return linkStatus(anchor.getAttribute('href') as string, anchor.textContent as string)
    }
    return emojiAtPoint(doc, x, y)
}
