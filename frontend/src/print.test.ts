import {describe, expect, it} from 'vitest'
import {printDocument, printFrameId, printFrameStyle, printReadyMarkerId} from './print'

describe('printFrameId', () => {
    it('is a stable identifier for the hidden print iframe', () => {
        expect(printFrameId).toBe('pp-print-frame')
    })
})

describe('printReadyMarkerId', () => {
    it('is a stable identifier for the print-ready marker', () => {
        expect(printReadyMarkerId).toBe('pp-print-ready')
    })
})

describe('printFrameStyle', () => {
    it('gives the print frame a real, page-sized off-screen box rather than a zero size', () => {
        expect(printFrameStyle).toContain('width:210mm')
        expect(printFrameStyle).toContain('height:297mm')
        expect(printFrameStyle).toContain('left:-10000px')
        expect(printFrameStyle).not.toContain('width:0')
        expect(printFrameStyle).not.toContain('height:0')
    })

    it('pins the print frame to a light colour scheme', () => {
        expect(printFrameStyle).toContain('color-scheme:light')
    })
})

describe('printDocument', () => {
    it('renders the subject, sender and date, wrapping the body', () => {
        const doc = printDocument('Hello', 'alice@example.com', '1 Jan 2026', '<p>Body</p>')
        expect(doc).toContain('<title>Hello</title>')
        expect(doc).toContain('From: alice@example.com')
        expect(doc).toContain('Date: 1 Jan 2026')
        expect(doc).toContain('<p>Body</p>')
    })

    it('omits the date row when no date is given', () => {
        const doc = printDocument('Hello', 'alice@example.com', '', '<p>Body</p>')
        expect(doc).not.toContain('Date:')
    })

    it('pins the document to a light colour scheme so an email dark mode does not print white on white', () => {
        const doc = printDocument('Hello', 'alice@example.com', '', '<p>Body</p>')
        expect(doc).toContain('name="color-scheme" content="light"')
        expect(doc).toContain(':root{color-scheme:light}')
    })

    it('stamps the print-ready marker on the document so the frame prints only the real content', () => {
        const doc = printDocument('Hello', 'alice@example.com', '', '<p>Body</p>')
        expect(doc).toContain(`id="${printReadyMarkerId}"`)
    })
})
