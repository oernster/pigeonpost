import {afterEach, describe, expect, it, vi} from 'vitest'
import {hoverStatusAt} from './hoverProbe'

// jsdom lays nothing out: it has no caret-from-point lookup and every box is empty. Each test supplies
// the caret the pointer would rest at and the box each emoji would be drawn in.
interface CaretLookups {
    caretPositionFromPoint?: (x: number, y: number) => {offsetNode: Node, offset: number} | null
    caretRangeFromPoint?: (x: number, y: number) => Range | null
}

// lookups exposes a document's caret lookups for a test to supply. The DOM typings declare both as
// always present, which jsdom's documents are not.
function lookups(doc: Document): CaretLookups {
    return doc as unknown as CaretLookups
}

function documentWith(body: string): Document {
    const doc = document.implementation.createHTMLDocument('')
    doc.body.innerHTML = body
    return doc
}

// drawAt makes every measured range report the box it is given.
function drawAt(box: {left: number, right: number, top: number, bottom: number}) {
    Range.prototype.getBoundingClientRect = () => box as DOMRect
}

const BOX = {left: 10, right: 30, top: 0, bottom: 20}
const INSIDE = {x: 20, y: 10}
const originalRect = Range.prototype.getBoundingClientRect

afterEach(() => {
    Range.prototype.getBoundingClientRect = originalRect
})

describe('hoverStatusAt: links', () => {
    it('reads the link the pointer is over, from any element inside it', () => {
        const doc = documentWith('<a href="https://example.com/x"><b>Read</b></a>')
        const status = hoverStatusAt(doc, doc.querySelector('b'), INSIDE.x, INSIDE.y)
        expect(status).toMatchObject({kind: 'link', host: 'example.com', after: '/x'})
    })

    it('answers null for a target that is not an element', () => {
        const doc = documentWith('<p>x</p>')
        expect(hoverStatusAt(doc, null, INSIDE.x, INSIDE.y)).toBeNull()
        expect(hoverStatusAt(doc, doc.body.firstChild?.firstChild ?? null, INSIDE.x, INSIDE.y)).toBeNull()
    })
})

describe('hoverStatusAt: emoji', () => {
    it('names the emoji drawn under the pointer, through the standard caret lookup', () => {
        const doc = documentWith('<p>hi 🎉</p>')
        const text = doc.querySelector('p')?.firstChild as Node
        lookups(doc).caretPositionFromPoint = () => ({offsetNode: text, offset: 3})
        drawAt(BOX)
        expect(hoverStatusAt(doc, doc.querySelector('p'), INSIDE.x, INSIDE.y))
            .toEqual({kind: 'emoji', glyph: '🎉', name: 'party popper'})
    })

    it('falls back to the older caret lookup WebKit offers', () => {
        const doc = documentWith('<p>🎉</p>')
        const text = doc.querySelector('p')?.firstChild as Node
        lookups(doc).caretRangeFromPoint = () => {
            const range = doc.createRange()
            range.setStart(text, 2)
            return range
        }
        drawAt(BOX)
        expect(hoverStatusAt(doc, doc.querySelector('p'), INSIDE.x, INSIDE.y)).toMatchObject({name: 'party popper'})
    })

    it('answers null when the emoji beside the caret is not drawn under the pointer', () => {
        const doc = documentWith('<p>🎉</p>')
        const text = doc.querySelector('p')?.firstChild as Node
        lookups(doc).caretPositionFromPoint = () => ({offsetNode: text, offset: 2})
        drawAt(BOX)
        for (const [x, y] of [[5, 10], [35, 10], [20, -5], [20, 25]]) {
            expect(hoverStatusAt(doc, doc.querySelector('p'), x, y)).toBeNull()
        }
    })

    it('answers null with no caret lookup, no caret or a caret outside text', () => {
        const doc = documentWith('<p>🎉</p>')
        const paragraph = doc.querySelector('p')
        expect(hoverStatusAt(doc, paragraph, INSIDE.x, INSIDE.y)).toBeNull()
        lookups(doc).caretPositionFromPoint = vi.fn(() => null)
        expect(hoverStatusAt(doc, paragraph, INSIDE.x, INSIDE.y)).toBeNull()
        lookups(doc).caretPositionFromPoint = () => ({offsetNode: paragraph as Node, offset: 0})
        expect(hoverStatusAt(doc, paragraph, INSIDE.x, INSIDE.y)).toBeNull()
    })
})
