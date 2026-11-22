import emojiData from 'emojibase-data/en/compact.json'
import githubShortcodes from 'emojibase-data/en/shortcodes/github.json'

// emoji is the one home for what PigeonPost knows about emoji: the name of each glyph (so hovering one can
// say what it is) and the text forms that stand for one, emoticons such as :) and shortcodes such as
// :thumbsup:, which compose turns into the glyph as they are typed and the reader shows as the glyph.
//
// Every table is derived from the Unicode CLDR data that emojibase publishes rather than typed here, so a
// new Unicode release reaches the app by updating the package, not by editing a list. Two choices sit on
// top of the data and are made here, each named below: which of its emoticons are safe to act on and the
// nose form that people commonly type but the data leaves out.

// EmojiRecord is the slice of an emojibase record this module reads. Skin-tone variants carry the same
// shape, nested under their base emoji.
interface EmojiRecord {
    hexcode: string
    label: string
    unicode: string
    emoticon?: string | string[]
    skins?: EmojiRecord[]
}

// A text match only counts when it stands on its own: at the start of the text or after whitespace; then
// followed by the end of the text, whitespace or sentence punctuation. Requiring whitespace in front is
// what keeps a URL safe: "https://" holds ":/" and a path can hold ":smile:", yet neither is preceded by
// a space. TRAILING_BOUNDARY_SOURCE is shared with the compose input rules, so a token converts on the same
// characters there as here.
export const TRAILING_BOUNDARY_SOURCE = '[\\s.,!?;]'
const LEADING_BOUNDARY_SOURCE = '(^|\\s)'

// A shortcode is a lower-case name between colons, the form GitHub and Slack use.
export const SHORTCODE_NAME_SOURCE = '[a-z0-9_+\\-]+'

// The CLDR data lists emoticons such as "XP", "xo", "8)" and "D:", which read as ordinary text far more
// often than as faces: "Windows XP", "xo" signing off a letter, "8)" in a numbered list, "Plan D: ...". An
// emoticon is therefore only acted on when it starts with a symbol (":", ";", "<" and the like), never a
// letter or a digit.
const STARTS_WITH_SYMBOL = /^[^\p{L}\p{N}]/u

// NOSE_EYES and NOSE are the nose form: the data lists ":)" and ";)" but people as often type ":-)" and
// ";-)". A two-character emoticon whose eyes are one of these gains the nosed spelling too.
const NOSE_EYES = [':', ';']
const NOSE = '-'
const PLAIN_EMOTICON_LENGTH = 2

// VARIATION_SELECTOR is U+FE0F, which asks for the colourful presentation of a glyph. Mail arrives with
// and without it ("❤" and "❤️" are the same heart), so names are keyed with it removed.
const VARIATION_SELECTOR = /️/g

function nameKey(glyph: string): string {
    return glyph.replace(VARIATION_SELECTOR, '')
}

function asList(value: string | string[] | undefined): string[] {
    if (value === undefined) {
        return []
    }
    return Array.isArray(value) ? value : [value]
}

// noseForms answers the nosed spelling of an emoticon that has one; nothing for any other.
function noseForms(emoticon: string): string[] {
    const plain = emoticon.length === PLAIN_EMOTICON_LENGTH && NOSE_EYES.includes(emoticon[0])
    return plain ? [emoticon[0] + NOSE + emoticon[1]] : []
}

const records = emojiData as readonly EmojiRecord[]

// NAMES maps a glyph (variation selector removed) to its CLDR name; GLYPHS maps a hexcode to its glyph,
// which is how the shortcode dataset refers to an emoji.
const variants = records.flatMap((record) => [record, ...(record.skins ?? [])])
const NAMES = new Map(variants.map((v) => [nameKey(v.unicode), v.label]))
const GLYPHS = new Map(variants.map((v) => [v.hexcode, v.unicode]))

// EMOTICONS is built so a spelling the data lists always beats a derived nose form; among listed
// spellings the first record to claim one keeps it. A Map keeps the last entry given for a key, so the
// derived forms go in first and each group goes in reversed.
type Entry = [string, string]
const listed: Entry[] = records.flatMap((record) =>
    asList(record.emoticon)
        .filter((e) => STARTS_WITH_SYMBOL.test(e))
        .map((e): Entry => [e, record.unicode]))
const derived: Entry[] = listed.flatMap(([e, glyph]) => noseForms(e).map((form): Entry => [form, glyph]))
const EMOTICONS = new Map([...derived.reverse(), ...listed.reverse()])

// SHORTCODES maps a GitHub shortcode name to its glyph. A hexcode the main data does not carry (none in
// this release) is dropped rather than mapped to nothing.
const SHORTCODES = new Map(
    Object.entries(githubShortcodes as Record<string, string | string[]>)
        .flatMap(([hexcode, names]) => asList(names).map((name) => [name, GLYPHS.get(hexcode)]))
        .filter((entry): entry is Entry => entry[1] !== undefined),
)

function escapeRegExp(text: string): string {
    return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

// EMOTICON_SOURCE is every emoticon as one alternation, longest first so ">:)" is matched whole rather
// than as a stray ">" before ":)".
export const EMOTICON_SOURCE = [...EMOTICONS.keys()]
    .sort((a, b) => b.length - a.length)
    .map(escapeRegExp)
    .join('|')

// TOKEN_RE finds a shortcode or an emoticon standing on its own. Group 1 is the leading boundary (empty or
// one whitespace character), group 2 a shortcode's name, group 3 an emoticon.
const TOKEN_RE = new RegExp(
    `${LEADING_BOUNDARY_SOURCE}(?::(${SHORTCODE_NAME_SOURCE}):|(${EMOTICON_SOURCE}))(?=$|${TRAILING_BOUNDARY_SOURCE})`,
    'g',
)

// emojiName answers the CLDR name of a glyph ("slightly smiling face"); null when it is not an emoji.
export function emojiName(glyph: string): string | null {
    return NAMES.get(nameKey(glyph)) ?? null
}

// emoticonEmoji answers the glyph an emoticon stands for; null when it is not one this module acts on.
export function emoticonEmoji(emoticon: string): string | null {
    return EMOTICONS.get(emoticon) ?? null
}

// shortcodeEmoji answers the glyph a shortcode name (without its colons) stands for; null when unknown.
export function shortcodeEmoji(name: string): string | null {
    return SHORTCODES.get(name) ?? null
}

// EmojiReplacement is one token found in a text: the span it occupies and the glyph that replaces it.
export interface EmojiReplacement {
    start: number
    end: number
    glyph: string
}

// emojiReplacements finds every emoticon and known shortcode in text that stands on its own, in order. A
// colon-delimited name that is not a known shortcode is left alone, so "10:30:45" and ":unknown:" stay
// as they were written.
export function emojiReplacements(text: string): EmojiReplacement[] {
    const found: EmojiReplacement[] = []
    for (const match of text.matchAll(TOKEN_RE)) {
        const glyph = match[2] !== undefined ? shortcodeEmoji(match[2]) : emoticonEmoji(match[3])
        if (glyph !== null) {
            const start = match.index + match[1].length
            found.push({start, end: start + match[0].length - match[1].length, glyph})
        }
    }
    return found
}

// translateEmoji answers text with every emoticon and shortcode in it replaced by its glyph.
export function translateEmoji(text: string): string {
    let result = ''
    let cursor = 0
    for (const {start, end, glyph} of emojiReplacements(text)) {
        result += text.slice(cursor, start) + glyph
        cursor = end
    }
    return result + text.slice(cursor)
}

// GraphemeSpan is one user-perceived character of a text and where it sits: a flag, a family or a
// skin-toned hand is one grapheme made of several code points.
export interface GraphemeSpan {
    start: number
    end: number
    grapheme: string
}

const segmenter = new Intl.Segmenter(undefined, {granularity: 'grapheme'})

// emojiBeside answers the emoji graphemes touching offset in text: the one ending there and the one
// starting there, either of which a pointer resting at that caret position may be over. The caller picks
// between them by where each is drawn.
export function emojiBeside(text: string, offset: number): GraphemeSpan[] {
    const spans: GraphemeSpan[] = []
    for (const {segment, index} of segmenter.segment(text)) {
        const end = index + segment.length
        if ((end === offset || index === offset) && emojiName(segment) !== null) {
            spans.push({start: index, end, grapheme: segment})
        }
        if (index > offset) {
            break
        }
    }
    return spans
}
