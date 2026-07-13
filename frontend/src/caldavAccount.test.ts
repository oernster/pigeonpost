import {describe, expect, it} from 'vitest'
import {CalDAVAccountForm, emptyCalDAVAccountForm, validateCalDAVAccountForm} from './caldavAccount'

// A ready-to-submit form, cloned and mutated per case so each assertion isolates the one field it probes.
function validForm(): CalDAVAccountForm {
    return {displayName: 'Fastmail', baseUrl: 'https://caldav.fastmail.com', username: 'me@example.com', password: 'app-pw'}
}

describe('emptyCalDAVAccountForm', () => {
    it('is blank in every field', () => {
        expect(emptyCalDAVAccountForm()).toEqual({displayName: '', baseUrl: '', username: '', password: ''})
    })
})

describe('validateCalDAVAccountForm', () => {
    it('accepts a complete form', () => {
        expect(validateCalDAVAccountForm(validForm())).toBe('')
    })

    it('accepts a plain http (not only https) base URL', () => {
        expect(validateCalDAVAccountForm({...validForm(), baseUrl: 'http://localhost:5232'})).toBe('')
    })

    it('rejects a blank display name', () => {
        expect(validateCalDAVAccountForm({...validForm(), displayName: ''})).toBe('Enter a name for this account.')
    })

    it('rejects a whitespace-only display name', () => {
        expect(validateCalDAVAccountForm({...validForm(), displayName: '   '})).toBe('Enter a name for this account.')
    })

    it('rejects a blank server address', () => {
        expect(validateCalDAVAccountForm({...validForm(), baseUrl: '   '})).toBe('Enter the server address.')
    })

    it('rejects a server address without an http(s) scheme', () => {
        expect(validateCalDAVAccountForm({...validForm(), baseUrl: 'caldav.fastmail.com'}))
            .toBe('The server address must start with http:// or https://.')
    })

    it('rejects a blank username', () => {
        expect(validateCalDAVAccountForm({...validForm(), username: '  '})).toBe('Enter the username.')
    })

    it('rejects a blank password and does not trim it', () => {
        expect(validateCalDAVAccountForm({...validForm(), password: ''})).toBe('Enter the password.')
        // A password of only spaces is a real (if unusual) secret, so it is accepted, not rejected as blank.
        expect(validateCalDAVAccountForm({...validForm(), password: '   '})).toBe('')
    })
})
