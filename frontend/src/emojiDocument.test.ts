import {describe, expect, it} from 'vitest'
import {translateEmojiInDocument} from './emojiDocument'

function documentWith(body: string): Document {
    const doc = document.implementation.createHTMLDocument('')
    doc.body.innerHTML = body
    return doc
}

describe('translateEmojiInDocument', () => {
    it('shows emoticons and shortcodes in ordinary text as emoji', () => {
        const doc = documentWith('<p>Thanks :) <b>see you :tada:</b></p>')
        translateEmojiInDocument(doc)
        expect(doc.body.innerHTML).toBe('<p>Thanks 🙂 <b>see you 🎉</b></p>')
    })

    it('changes text only, never an attribute', () => {
        const doc = documentWith('<a href="https://x.test/ :) " title=":)">go :)</a>')
        translateEmojiInDocument(doc)
        const anchor = doc.querySelector('a') as HTMLAnchorElement
        expect(anchor.getAttribute('href')).toBe('https://x.test/ :) ')
        expect(anchor.getAttribute('title')).toBe(':)')
        expect(anchor.textContent).toBe('go 🙂')
    })

    it('leaves code, preformatted text and styles as written', () => {
        const verbatim = '<code>a :)</code><pre>b :P</pre><kbd>c :)</kbd><samp>d :)</samp><style>e :) {}</style>'
        const doc = documentWith(verbatim + '<p><code><span>f :)</span></code></p>')
        translateEmojiInDocument(doc)
        expect(doc.body.innerHTML).toBe(verbatim + '<p><code><span>f :)</span></code></p>')
    })

    it('is a no-op the second time and on a document with no body', () => {
        const doc = documentWith('<p>hi :)</p>')
        translateEmojiInDocument(doc)
        translateEmojiInDocument(doc)
        expect(doc.body.innerHTML).toBe('<p>hi 🙂</p>')
        const bodiless = document.implementation.createDocument(null, 'root', null)
        expect(() => translateEmojiInDocument(bodiless)).not.toThrow()
    })
})
