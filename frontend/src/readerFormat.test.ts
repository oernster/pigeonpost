import {describe, expect, it} from 'vitest'
import {formatAddress, formatAddressList, formatBytes, readableInk} from './readerFormat'

describe('formatAddress', () => {
    it('renders a named correspondent as Name <address>', () => {
        expect(formatAddress({name: 'Alice', address: 'alice@example.com'})).toBe('Alice <alice@example.com>')
    })

    it('renders just the address when there is no name', () => {
        expect(formatAddress({name: '', address: 'bob@example.com'})).toBe('bob@example.com')
    })
})

describe('formatAddressList', () => {
    it('joins correspondents and drops empty entries', () => {
        const list = [
            {name: 'Alice', address: 'alice@example.com'},
            {name: '', address: 'bob@example.com'},
            {name: '', address: ''},
        ]
        expect(formatAddressList(list)).toBe('Alice <alice@example.com>, bob@example.com')
    })
})

describe('formatBytes', () => {
    it('renders bytes below a kilobyte as plain bytes', () => {
        expect(formatBytes(500)).toBe('500 B')
    })

    it('renders kilobytes to one decimal place', () => {
        expect(formatBytes(1536)).toBe('1.5 KB')
    })

    it('renders megabytes', () => {
        expect(formatBytes(1048576)).toBe('1.0 MB')
    })

    it('renders gigabytes', () => {
        expect(formatBytes(1073741824)).toBe('1.0 GB')
    })

    it('caps at gigabytes for very large sizes', () => {
        expect(formatBytes(1099511627776)).toBe('1024.0 GB')
    })
})

describe('readableInk', () => {
    it('returns black for a malformed hex value', () => {
        expect(readableInk('#fff')).toBe('#000000')
    })

    it('returns black ink on a light background', () => {
        expect(readableInk('#ffffff')).toBe('#000000')
    })

    it('returns white ink on a dark background', () => {
        expect(readableInk('#000000')).toBe('#ffffff')
    })
})
