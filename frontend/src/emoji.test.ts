import {describe, expect, it} from 'vitest'
import {emojiBeside, emojiName, emojiReplacements, emoticonEmoji, shortcodeEmoji, translateEmoji} from './emoji'

describe('emojiName', () => {
    it('answers the CLDR name of an emoji', () => {
        expect(emojiName('🙂')).toBe('slightly smiling face')
        expect(emojiName('🎉')).toBe('party popper')
    })

    it('names a heart with or without its variation selector', () => {
        expect(emojiName('❤️')).toBe('red heart')
        expect(emojiName('❤')).toBe('red heart')
    })

    it('names a skin-toned variant and a flag', () => {
        expect(emojiName('👋🏽')).toBe('waving hand: medium skin tone')
        expect(emojiName('🇬🇧')).toBe('flag: United Kingdom')
    })

    it('answers null for text that is not an emoji', () => {
        expect(emojiName('a')).toBeNull()
        expect(emojiName('')).toBeNull()
    })
})

describe('emoticonEmoji', () => {
    it('answers the glyph for an emoticon the data lists', () => {
        expect(emoticonEmoji(':)')).toBe('🙂')
        expect(emoticonEmoji('<3')).toBe('❤️')
        expect(emoticonEmoji('>:)')).toBe('😈')
    })

    it('adds the nose form people commonly type', () => {
        expect(emoticonEmoji(':-)')).toBe('🙂')
        expect(emoticonEmoji(';-)')).toBe('😉')
        expect(emoticonEmoji(':-(')).toBe(emoticonEmoji(':('))
    })

    it('never acts on an emoticon that starts with a letter or a digit', () => {
        for (const textual of ['XP', 'xo', 'xD', '8)', 'D:', 'Dx', '8D', 'O:)']) {
            expect(emoticonEmoji(textual)).toBeNull()
        }
    })
})

describe('shortcodeEmoji', () => {
    it('answers the glyph for a GitHub shortcode, including its aliases', () => {
        expect(shortcodeEmoji('tada')).toBe('🎉')
        expect(shortcodeEmoji('thumbsup')).toBe('👍')
        expect(shortcodeEmoji('+1')).toBe('👍')
    })

    it('answers null for an unknown name', () => {
        expect(shortcodeEmoji('not_an_emoji_name')).toBeNull()
    })
})

describe('emojiReplacements and translateEmoji', () => {
    it('translates emoticons and shortcodes standing on their own', () => {
        expect(translateEmoji('Thanks :) see you :tada:')).toBe('Thanks 🙂 see you 🎉')
        expect(translateEmoji(':-D great')).toBe('😄 great')
    })

    it('accepts sentence punctuation straight after a token', () => {
        expect(translateEmoji('Done :).')).toBe('Done 🙂.')
        expect(translateEmoji('Yes :thumbsup:!')).toBe('Yes 👍!')
    })

    it('matches the longest emoticon rather than a part of it', () => {
        expect(translateEmoji('evil >:) plan')).toBe('evil 😈 plan')
    })

    it('translates adjacent tokens separated by a space', () => {
        expect(translateEmoji(':) :)')).toBe('🙂 🙂')
    })

    it('never touches a web address', () => {
        const text = 'see https://example.com/:smile:/x and http://a.b/:) ok'
        expect(translateEmoji(text)).toBe(text)
    })

    it('leaves times, unknown names and glued text as written', () => {
        for (const text of ['at 10:30:45 today', 'a :notreal: b', 'smile:)', ':)x', 'Windows XP', 'items 1 to 8)']) {
            expect(translateEmoji(text)).toBe(text)
        }
    })

    it('reports where each token sits', () => {
        expect(emojiReplacements('hi :)')).toEqual([{start: 3, end: 5, glyph: '🙂'}])
        expect(emojiReplacements(':tada:')).toEqual([{start: 0, end: 6, glyph: '🎉'}])
    })
})

describe('emojiBeside', () => {
    it('answers the emoji ending and starting at a caret offset', () => {
        const text = 'a🙂🎉b'
        expect(emojiBeside(text, 1)).toEqual([{start: 1, end: 3, grapheme: '🙂'}])
        expect(emojiBeside(text, 3)).toEqual([
            {start: 1, end: 3, grapheme: '🙂'},
            {start: 3, end: 5, grapheme: '🎉'},
        ])
    })

    it('treats a flag or a skin-toned hand as one character', () => {
        expect(emojiBeside('🇬🇧!', 4)).toEqual([{start: 0, end: 4, grapheme: '🇬🇧'}])
        expect(emojiBeside('👋🏽', 0)).toEqual([{start: 0, end: 4, grapheme: '👋🏽'}])
    })

    it('answers nothing beside plain text', () => {
        expect(emojiBeside('plain text', 3)).toEqual([])
    })
})
