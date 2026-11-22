import {afterEach, describe, expect, it} from 'vitest'
import {Editor} from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import {EmojiInput, translateTrailingEmoji} from './composeEmoji'

// These run a real TipTap editor, so the input rules are exercised the way a keystroke reaches them:
// through ProseMirror's handleTextInput, one character at a time, falling back to a plain insert when no
// rule claims the character.
let editor: Editor | null = null

function editorWith(content: string): Editor {
    editor = new Editor({extensions: [StarterKit, EmojiInput], content})
    editor.commands.focus('end')
    return editor
}

afterEach(() => {
    editor?.destroy()
    editor = null
})

function type(target: Editor, text: string) {
    for (const character of text) {
        const {from, to} = target.state.selection
        const view = target.view
        const deflt = () => view.state.tr.insertText(character, from, to)
        const handled = view.someProp('handleTextInput', (f) => f(view, from, to, character, deflt))
        if (!handled) {
            view.dispatch(deflt())
        }
    }
}

function pressEnter(target: Editor) {
    const view = target.view
    const event = new KeyboardEvent('keydown', {key: 'Enter'})
    if (!view.someProp('handleKeyDown', (f) => f(view, event))) {
        throw new Error('Enter was not handled')
    }
}

function pressBackspace(target: Editor) {
    const view = target.view
    view.someProp('handleKeyDown', (f) => f(view, new KeyboardEvent('keydown', {key: 'Backspace'})))
}

describe('EmojiInput: typing', () => {
    it('converts an emoticon when the character after it is typed', () => {
        const e = editorWith('<p></p>')
        type(e, 'Hi :)')
        expect(e.getText()).toBe('Hi :)')
        type(e, ' there :-D.')
        expect(e.getText()).toBe('Hi 🙂 there 😄.')
    })

    it('converts a shortcode as its closing colon is typed', () => {
        const e = editorWith('<p></p>')
        type(e, 'Yes :thumbsup:')
        expect(e.getText()).toBe('Yes 👍')
    })

    it('leaves an unknown shortcode, a glued emoticon and a letter emoticon as typed', () => {
        const e = editorWith('<p></p>')
        type(e, ':nope: a:) Windows XP ')
        expect(e.getText()).toBe(':nope: a:) Windows XP ')
    })

    it('puts the text back on Backspace straight after a conversion', () => {
        const e = editorWith('<p></p>')
        type(e, ':) ')
        expect(e.getText()).toBe('🙂 ')
        pressBackspace(e)
        expect(e.getText()).toBe(':) ')
    })

    it('does not convert inside a quoted message', () => {
        const e = editorWith('<blockquote><p>quoted</p></blockquote>')
        type(e, ' :) :tada:')
        expect(e.getText().trim()).toBe('quoted :) :tada:')
    })

    it('does not claim a character typed over a selection longer than the emoticon rule expects', () => {
        const e = editorWith('<p>ab :)</p>')
        const view = e.view
        const end = e.state.selection.from
        const handled = view.someProp('handleTextInput', (f) => f(view, end, end, '  ', () => view.state.tr))
        expect(handled).toBeFalsy()
        const shortcode = editorWith('<p>:tada</p>')
        const at = shortcode.state.selection.from
        const claimed = shortcode.view.someProp('handleTextInput',
            (f) => f(shortcode.view, at, at, 'x:', () => shortcode.view.state.tr))
        expect(claimed).toBeFalsy()
    })
})

describe('EmojiInput: Enter', () => {
    it('converts the emoticon before the caret and still starts a new line', () => {
        const e = editorWith('<p>Thanks :)</p>')
        pressEnter(e)
        expect(e.getHTML()).toBe('<p>Thanks 🙂</p><p></p>')
    })

    it('leaves a line with no emoticon at its caret alone', () => {
        const e = editorWith('<p>Thanks</p>')
        pressEnter(e)
        expect(e.getHTML()).toBe('<p>Thanks</p><p></p>')
    })

    it('converts nothing when text is selected', () => {
        const e = editorWith('<p>Thanks :)</p>')
        e.commands.setTextSelection({from: 1, to: 4})
        pressEnter(e)
        expect(e.getText()).not.toContain('🙂')
    })
})

describe('translateTrailingEmoji', () => {
    it('converts an emoticon or shortcode left at the end of any line', () => {
        const e = editorWith('<p>Thanks :)</p><p>See you :tada:</p><ul><li><p>ok :P</p></li></ul>')
        translateTrailingEmoji(e)
        // The closing empty paragraph is StarterKit's own trailing node, which follows a list at the end.
        expect(e.getHTML()).toBe('<p>Thanks 🙂</p><p>See you 🎉</p><ul><li><p>ok 😛</p></li></ul><p></p>')
    })

    it('leaves a token that is not at the end of its line', () => {
        const e = editorWith('<p>a :) b</p>')
        translateTrailingEmoji(e)
        expect(e.getText()).toBe('a :) b')
    })

    it('leaves quoted text, code blocks and inline code as written', () => {
        const html = '<blockquote><p>theirs :)</p></blockquote><pre><code>x :)</code></pre><p><code>y :)</code></p>'
        const e = editorWith(html)
        translateTrailingEmoji(e)
        expect(e.getHTML()).toBe(html)
    })
})
