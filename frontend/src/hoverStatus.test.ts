import {describe, expect, it, vi} from 'vitest'
import {createHoverChannel, emojiStatus, isOpenableHref, linkStatus} from './hoverStatus'

describe('isOpenableHref', () => {
    it('opens http, https and mailto only, whatever the case or padding', () => {
        expect(isOpenableHref('https://example.com')).toBe(true)
        expect(isOpenableHref('  HTTP://example.com')).toBe(true)
        expect(isOpenableHref('mailto:someone@example.com')).toBe(true)
        expect(isOpenableHref('javascript:alert(1)')).toBe(false)
        expect(isOpenableHref('data:text/html,x')).toBe(false)
    })
})

describe('linkStatus: web links', () => {
    it('splits the destination around its host', () => {
        expect(linkStatus('https://example.com/a?b=1', 'Read more')).toEqual({
            kind: 'link', before: 'https://', host: 'example.com', after: '/a?b=1', openable: true, shown: null,
        })
    })

    it('keeps a port with the host', () => {
        expect(linkStatus('http://example.com:8080/x', '').host).toBe('example.com:8080')
    })

    it('finds the real host behind user-info that imitates another site', () => {
        const status = linkStatus('https://bank.example@elsewhere.test/login', 'Sign in')
        expect(status.before).toBe('https://bank.example@')
        expect(status.host).toBe('elsewhere.test')
        expect(linkStatus('https://elsewhere.test:pw@elsewhere.test/', '').before).toBe('https://elsewhere.test:pw@')
    })

    it('shows an international host in its xn-- form, so a lookalike cannot pass for the real name', () => {
        expect(linkStatus('https://pаypal.com/', '').host).toBe('xn--pypal-4ve.com')
    })

    it('says when the visible text names a different site', () => {
        expect(linkStatus('https://evil.test/login', 'www.yourbank.com').shown).toBe('www.yourbank.com')
        expect(linkStatus('https://evil.test/', 'https://yourbank.com/account').shown).toBe('yourbank.com')
        expect(linkStatus('https://yourbank.com.evil.test/', 'yourbank.com').shown).toBe('yourbank.com')
    })

    it('raises nothing when the text names the same site, a parent or a subdomain of it', () => {
        expect(linkStatus('https://www.example.com/', 'example.com').shown).toBeNull()
        expect(linkStatus('https://mail.example.com/', 'example.com').shown).toBeNull()
        expect(linkStatus('https://example.com/', 'shop.example.com').shown).toBeNull()
    })

    it('raises nothing when the text does not name a site', () => {
        // '%%.com' has the shape of a site name but no parser accepts it as a host.
        for (const text of ['Click here', 'v1.2', 'see the.notes now', '', '%%.com']) {
            expect(linkStatus('https://evil.test/', text).shown).toBeNull()
        }
    })
})

describe('linkStatus: mailto and refused links', () => {
    it('reads a mailto link as its address, keeping any query after it', () => {
        expect(linkStatus('mailto:someone%40x@example.com?subject=Hi', 'Write to us')).toEqual({
            kind: 'link', before: 'mailto:', host: 'someone@x@example.com', after: '?subject=Hi',
            openable: true, shown: null,
        })
        expect(linkStatus('mailto:a@example.com', '').after).toBe('')
    })

    it('says when a mailto link is drawn as a different address', () => {
        expect(linkStatus('mailto:thief@evil.test', 'help@yourbank.com').shown).toBe('help@yourbank.com')
        expect(linkStatus('mailto:Help@YourBank.com', 'help@yourbank.com').shown).toBeNull()
    })

    it('keeps an address whose escapes do not decode as written', () => {
        expect(linkStatus('mailto:bad%E0@example.com', '').host).toBe('bad%E0@example.com')
    })

    it('marks as refused a scheme PigeonPost will not open and a web link with no host', () => {
        const refused = {kind: 'link', host: '', after: '', openable: false, shown: null}
        expect(linkStatus(' javascript:alert(1) ', 'x')).toEqual({...refused, before: 'javascript:alert(1)'})
        expect(linkStatus('http://', 'x')).toEqual({...refused, before: 'http://'})
        expect(linkStatus('https:', 'x')).toEqual({...refused, before: 'https:'})
    })
})

describe('emojiStatus', () => {
    it('names an emoji and answers null for anything else', () => {
        expect(emojiStatus('🎉')).toEqual({kind: 'emoji', glyph: '🎉', name: 'party popper'})
        expect(emojiStatus('a')).toBeNull()
    })
})

describe('createHoverChannel', () => {
    it('tells its listeners only when the reading changes', () => {
        const channel = createHoverChannel()
        const listener = vi.fn()
        const unsubscribe = channel.subscribe(listener)
        const party = emojiStatus('🎉')
        channel.report(party)
        channel.report(emojiStatus('🎉'))
        expect(listener).toHaveBeenCalledTimes(1)
        expect(channel.current()).toBe(party)
        channel.report(null)
        channel.report(null)
        expect(listener).toHaveBeenCalledTimes(2)
        expect(channel.current()).toBeNull()
        unsubscribe()
        channel.report(party)
        expect(listener).toHaveBeenCalledTimes(2)
    })
})
