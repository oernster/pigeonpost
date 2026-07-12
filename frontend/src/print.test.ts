import {describe, expect, it} from 'vitest'
import {printDocument, printFrameId} from './print'

describe('printFrameId', () => {
    it('is a stable identifier for the hidden print iframe', () => {
        expect(printFrameId).toBe('pp-print-frame')
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
})
