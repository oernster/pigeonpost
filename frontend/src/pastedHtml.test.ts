import {describe, expect, it} from 'vitest'
import {normalisePastedHtml} from './pastedHtml'

const NL = String.fromCharCode(10)
const CRLF = String.fromCharCode(13, 10)

const SIGNATURE = '<p>Oliver Ernster</p><p>Principal Engineer</p><p><a href="https://ernster.dev">ernster.dev</a></p>'

describe('normalisePastedHtml', () => {
    it('leaves markup with no wrapper and no between-block newlines alone', () => {
        expect(normalisePastedHtml(SIGNATURE)).toBe(SIGNATURE)
    })

    it('keeps only the CF_HTML fragment, dropping the newlines around its markers', () => {
        const clipboard =
            '<html><body>' + CRLF + '<!--StartFragment-->' + SIGNATURE + '<!--EndFragment-->' + CRLF + '</body>' + CRLF + '</html>'
        expect(normalisePastedHtml(clipboard)).toBe(SIGNATURE)
    })

    it('ignores a start marker with no end marker', () => {
        expect(normalisePastedHtml('<!--StartFragment-->' + SIGNATURE)).toBe('<!--StartFragment-->' + SIGNATURE)
    })

    it('ignores an end marker that precedes the start marker', () => {
        const scrambled = '<!--EndFragment--><p>a</p><!--StartFragment-->'
        expect(normalisePastedHtml(scrambled)).toBe(scrambled)
    })

    it('removes a newline between two block tags', () => {
        expect(normalisePastedHtml('<p>one</p>' + NL + '<p>two</p>')).toBe('<p>one</p><p>two</p>')
    })

    it('removes a newline after an opening block tag and before a closing one', () => {
        const pretty = '<body>' + NL + '  <div>' + NL + '    <p>one</p>' + NL + '  </div>' + NL + '</body>'
        expect(normalisePastedHtml(pretty)).toBe('<body><div><p>one</p></div></body>')
    })

    it('trims leading and trailing whitespace', () => {
        expect(normalisePastedHtml(CRLF + '  <p>one</p>  ' + NL)).toBe('<p>one</p>')
    })

    it('keeps a space between two inline marks', () => {
        const inline = '<p><strong>Oliver</strong> <em>Ernster</em></p>'
        expect(normalisePastedHtml(inline)).toBe(inline)
    })

    it('keeps a space inside a paragraph that runs onto the next line', () => {
        expect(normalisePastedHtml('<p>one' + NL + 'two</p>')).toBe('<p>one' + NL + 'two</p>')
    })

    it('returns an empty string for empty markup', () => {
        expect(normalisePastedHtml('')).toBe('')
    })
})
