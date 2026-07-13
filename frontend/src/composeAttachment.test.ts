import {describe, expect, it} from 'vitest'
import {bodyMentionsAttachment} from './composeAttachment'

describe('bodyMentionsAttachment', () => {
    it('matches the word attached in the user body', () => {
        expect(bodyMentionsAttachment('', '<p>See the attached file</p>')).toBe(true)
    })

    it('matches attach mentioned in the subject', () => {
        expect(bodyMentionsAttachment('Re: attached', '<p>Thanks</p>')).toBe(true)
    })

    it('ignores an attachment mentioned only in the quoted reply', () => {
        const html = '<p>Thanks</p><blockquote><p>Please see attached</p></blockquote>'
        expect(bodyMentionsAttachment('Re: report', html)).toBe(false)
    })

    it('ignores nested quoted chains that mention an attachment', () => {
        const html = '<p>Ok</p><blockquote><p>reply</p><blockquote><p>attached earlier</p></blockquote></blockquote>'
        expect(bodyMentionsAttachment('Re: report', html)).toBe(false)
    })

    it('still warns when the user writes above a quote that mentions it', () => {
        const html = '<p>See attached please</p><blockquote><p>old thread</p></blockquote>'
        expect(bodyMentionsAttachment('', html)).toBe(true)
    })

    it('is false when nothing mentions an attachment', () => {
        expect(bodyMentionsAttachment('Re: hello', '<p>Thanks for the update</p>')).toBe(false)
    })

    it('handles an empty body', () => {
        expect(bodyMentionsAttachment('', '')).toBe(false)
    })
})
