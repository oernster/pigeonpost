import {Extension, InputRule, type Editor} from '@tiptap/core'
import type {Node as ProseNode} from '@tiptap/pm/model'
import type {Transaction} from '@tiptap/pm/state'
import {
    EMOTICON_SOURCE, SHORTCODE_NAME_SOURCE, TRAILING_BOUNDARY_SOURCE, emojiReplacements, emoticonEmoji,
    shortcodeEmoji,
} from './emoji'

// EmojiInput turns emoticons and shortcodes into emoji as they are typed in a message body, with the
// tables and the standing-alone rule the reader uses (see emoji), so a face converts on the same terms
// whichever side of the send it is on.
//
//   * An emoticon converts when the character after it is typed: a space or sentence punctuation. It
//     waits for that character because ":p" is also the start of ":path" and ":D" of ":Done".
//   * A shortcode converts the moment its closing colon is typed, the way Slack and GitHub do it.
//   * Both are undoable input rules: Backspace straight after a conversion puts the text back.
//   * Enter converts an emoticon just before the caret; translateTrailingEmoji converts one left at the
//     very end of a line when the message is sent. Those are the two places no following character is
//     typed.
//
// Nothing converts inside code (TipTap's input rules already respect that) nor inside a quoted message,
// which is someone else's text.

const EMOTICON_RULE = new RegExp(`(?:^|\\s)(${EMOTICON_SOURCE})(${TRAILING_BOUNDARY_SOURCE})$`)
const SHORTCODE_RULE = new RegExp(`(?:^|\\s)(:${SHORTCODE_NAME_SOURCE}):$`)
const SHORTCODE_CLOSE = ':'

// EMOJI_INPUT_PRIORITY runs this extension's Enter ahead of the editor's own, which splits the paragraph:
// the emoticon must be converted while the caret still sits after it. TipTap's default priority is 100.
const EMOJI_INPUT_PRIORITY = 1000

// LEAF_TEXT stands in for an inline leaf (a hard break, an image) when a block's text is read, so every
// character of the text keeps the same offset as its position in the document.
const LEAF_TEXT = '￼'

function blockText(node: ProseNode, upTo: number): string {
    return node.textBetween(0, upTo, undefined, LEAF_TEXT)
}

// typedLength is how much text the input rule's match took from what was just typed: the match covers
// the text before the caret plus the typed text, while range ends at the caret.
function typedLength(match: RegExpMatchArray, range: {from: number, to: number}): number {
    return match[0].length - (range.to - range.from)
}

// inQuote reports whether a position sits inside a quoted message.
function inQuote(doc: ProseNode, pos: number): boolean {
    const $pos = doc.resolve(pos)
    for (let depth = $pos.depth; depth > 0; depth--) {
        if ($pos.node(depth).type.name === 'blockquote') {
            return true
        }
    }
    return false
}

const emoticonRule = new InputRule({
    find: EMOTICON_RULE,
    handler: ({state, range, match}) => {
        const glyph = emoticonEmoji(match[1])
        if (glyph === null || typedLength(match, range) !== match[2].length || inQuote(state.doc, range.to)) {
            return null
        }
        state.tr.insertText(glyph + match[2], range.to - match[1].length, range.to)
    },
})

const shortcodeRule = new InputRule({
    find: SHORTCODE_RULE,
    handler: ({state, range, match}) => {
        const glyph = shortcodeEmoji(match[1].slice(SHORTCODE_CLOSE.length))
        if (glyph === null || typedLength(match, range) !== SHORTCODE_CLOSE.length || inQuote(state.doc, range.to)) {
            return null
        }
        state.tr.insertText(glyph, range.to - match[1].length, range.to)
    },
})

// convertBlockEnd adds to tr the conversion of a token ending exactly at offset end of the textblock
// starting at blockStart; it answers whether there was one.
function convertBlockEnd(tr: Transaction, node: ProseNode, blockStart: number, end: number): boolean {
    const codeMark = tr.doc.type.schema.marks.code
    const last = emojiReplacements(blockText(node, end)).at(-1)
    if (!last || last.end !== end) {
        return false
    }
    const from = blockStart + last.start
    const to = blockStart + last.end
    if (node.type.spec.code || (codeMark && tr.doc.rangeHasMark(from, to, codeMark)) || inQuote(tr.doc, from)) {
        return false
    }
    tr.insertText(last.glyph, from, to)
    return true
}

// convertBeforeCaret converts an emoticon that ends at the caret, for Enter.
function convertBeforeCaret(editor: Editor): void {
    const {state} = editor
    const {selection} = state
    if (!selection.empty) {
        return
    }
    const {$from} = selection
    const tr = state.tr
    if (convertBlockEnd(tr, $from.parent, $from.start(), $from.parentOffset)) {
        editor.view.dispatch(tr)
    }
}

// translateTrailingEmoji converts every emoticon or shortcode left at the end of a line of the body, so
// "Thanks :)" sent without a space after it goes as the emoji too. Lines are worked last to first so an
// earlier conversion cannot shift the position of a later one.
export function translateTrailingEmoji(editor: Editor): void {
    const tr = editor.state.tr
    const blocks: {node: ProseNode, start: number}[] = []
    tr.doc.descendants((node, pos) => {
        if (node.isTextblock) {
            blocks.push({node, start: pos + 1})
            return false
        }
        return true
    })
    let changed = false
    for (const {node, start} of blocks.reverse()) {
        changed = convertBlockEnd(tr, node, start, node.content.size) || changed
    }
    if (changed) {
        editor.view.dispatch(tr)
    }
}

export const EmojiInput = Extension.create({
    name: 'emojiInput',
    priority: EMOJI_INPUT_PRIORITY,
    addInputRules() {
        return [emoticonRule, shortcodeRule]
    },
    addKeyboardShortcuts() {
        return {
            // Converting does not consume Enter: answering false lets the editor split the line as usual.
            Enter: () => {
                convertBeforeCaret(this.editor)
                return false
            },
        }
    },
})
