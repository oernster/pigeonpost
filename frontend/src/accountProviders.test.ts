import {describe, expect, it} from 'vitest'
import {
    DEFAULT_IN_PORT,
    DEFAULT_IN_PORT_POP3,
    PROTOCOL_OPTIONS,
    PROVIDERS,
    SECURITY_OPTIONS,
    domainOf,
    incomingHostPrefix,
    normaliseSigUrl,
} from './accountProviders'

describe('normaliseSigUrl', () => {
    it('returns an empty string unchanged', () => {
        expect(normaliseSigUrl('  ')).toBe('')
    })

    it('leaves a scheme-qualified url unchanged', () => {
        expect(normaliseSigUrl('https://example.com')).toBe('https://example.com')
    })

    it('adds https to a bare host', () => {
        expect(normaliseSigUrl('example.com')).toBe('https://example.com')
    })
})

describe('incomingHostPrefix', () => {
    it('uses pop for POP3', () => {
        expect(incomingHostPrefix('pop3')).toBe('pop')
    })

    it('uses imap for anything else', () => {
        expect(incomingHostPrefix('imap')).toBe('imap')
    })
})

describe('domainOf', () => {
    it('returns the domain part of an address', () => {
        expect(domainOf('user@example.com')).toBe('example.com')
    })

    it('returns an empty string when there is no @', () => {
        expect(domainOf('not-an-address')).toBe('')
    })
})

describe('preset data', () => {
    it('exposes the generic endpoint defaults', () => {
        expect(DEFAULT_IN_PORT).toBe(993)
        expect(DEFAULT_IN_PORT_POP3).toBe(995)
    })

    it('lists the protocol and security options', () => {
        expect(PROTOCOL_OPTIONS.map((o) => o.value)).toEqual(['imap', 'pop3'])
        expect(SECURITY_OPTIONS.map((o) => o.value)).toEqual(['tls', 'starttls', 'none'])
    })

    it('includes the known provider presets', () => {
        expect(PROVIDERS.map((p) => p.id)).toContain('gmail')
        expect(PROVIDERS.every((p) => p.inHost !== '' && p.outHost !== '')).toBe(true)
    })
})

describe('app password guidance', () => {
    it('links only to https app-password pages', () => {
        for (const p of PROVIDERS) {
            if (p.appPasswordUrl !== undefined) {
                expect(p.appPasswordUrl.startsWith('https://')).toBe(true)
            }
        }
    })

    it('warns the normal password will not work wherever an app-password page is linked', () => {
        for (const p of PROVIDERS) {
            if (p.appPasswordUrl !== undefined) {
                expect(p.note.toLowerCase()).toContain('will not work')
            }
        }
    })

    it('links the providers whose normal password is rejected and that expose a public page', () => {
        const linked = PROVIDERS.filter((p) => p.appPasswordUrl !== undefined).map((p) => p.id)
        expect(linked).toEqual(['gmail', 'icloud', 'yahoo'])
    })
})
