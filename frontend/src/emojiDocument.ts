import {translateEmoji} from './emoji'

// translateEmojiInDocument shows a received HTML message's emoticons and shortcodes as their emoji, by
// rewriting the text of the rendered document. It is display only: the stored message, a reply's quote
// and a forward keep what the sender wrote. Only text is touched, never an attribute, so an href or an
// image source cannot change. Text inside code, preformatted blocks, keyboard and sample markup is left
// as written, where ":)" or ":-P" is far more likely to be code than a face; so is anything inside a
// style, script or form control.
const VERBATIM = 'code, pre, kbd, samp, style, script, textarea'
const SHOW_TEXT = 4

export function translateEmojiInDocument(doc: Document): void {
    const root = doc.body
    if (!root) {
        return
    }
    const walker = doc.createTreeWalker(root, SHOW_TEXT)
    const changes: [Text, string][] = []
    for (let node = walker.nextNode(); node !== null; node = walker.nextNode()) {
        const text = node as Text
        if (text.parentElement?.closest(VERBATIM)) {
            continue
        }
        const translated = translateEmoji(text.data)
        if (translated !== text.data) {
            changes.push([text, translated])
        }
    }
    // Written after the walk, so the walker never steps over a node it has just changed.
    for (const [text, translated] of changes) {
        text.data = translated
    }
}
