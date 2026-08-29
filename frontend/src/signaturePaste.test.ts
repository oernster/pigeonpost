import {describe, expect, it} from 'vitest'
import {Editor} from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import {EDITOR_LINK_OPTIONS, EDITOR_PASTE_PROPS} from './richText'

// These exercise the signature editor's real configuration against the shapes a clipboard actually
// delivers. Pasting a signature copied from another account used to arrive with a blank line above it
// and two below, because the CF_HTML wrapper's newlines parse as content; the between-block newlines of
// pretty-printed markup did the same between every line.

const NL = String.fromCharCode(10)
const CRLF = String.fromCharCode(13, 10)
const SIGNATURE = '<p>Oliver Ernster</p><p>Principal Engineer</p><p>Decision Architecture</p>'

if (typeof (globalThis as Record<string, unknown>).ClipboardEvent === 'undefined') {
    ;(globalThis as Record<string, unknown>).ClipboardEvent = class extends Event {}
}

function pasteInto(html: string): string {
    const element = document.createElement('div')
    document.body.appendChild(element)
    const editor = new Editor({
        element,
        extensions: [StarterKit.configure({link: EDITOR_LINK_OPTIONS})],
        content: '',
        editorProps: EDITOR_PASTE_PROPS,
    })
    editor.view.pasteHTML(html)
    return editor.getHTML()
}

describe('pasting a signature', () => {
    it('takes plain fragment markup unchanged', () => {
        expect(pasteInto(SIGNATURE)).toBe(SIGNATURE)
    })

    it('adds no blank lines around a CF_HTML clipboard payload', () => {
        // A copy out of another signature editor stamps data-pm-slice on the first block; Windows then
        // wraps the fragment in CF_HTML, whose newlines used to arrive as a blank line above the pasted
        // signature and two below it.
        const copied = SIGNATURE.replace('<p>', '<p data-pm-slice="0 0 []">')
        const clipboard =
            '<html><body>' + CRLF + '<!--StartFragment-->' + copied + '<!--EndFragment-->' + CRLF + '</body>' + CRLF + '</html>'
        expect(pasteInto(clipboard)).toBe(SIGNATURE)
    })

    it('adds no blank lines between the lines of pretty-printed markup', () => {
        const pretty = '<p>Oliver Ernster</p>' + NL + '<p>Principal Engineer</p>' + NL + '<p>Decision Architecture</p>'
        expect(pasteInto(pretty)).toBe(SIGNATURE)
    })
})
