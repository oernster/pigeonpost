// The colour arithmetic the email treatment decides on. Getting these wrong shows up as a whole region
// rendering the wrong way round, so they are pinned directly rather than only through the rendered frame.
import {describe, expect, it} from 'vitest'
import {
    composite,
    contrastRatio,
    isDarkBackground,
    paintsBackground,
    parseColor,
    relativeLuminance,
} from './emailDarkMode'

const WHITE = {r: 255, g: 255, b: 255, a: 1}
const BLACK = {r: 0, g: 0, b: 0, a: 1}

describe('parseColor', () => {
    it('reads the rgb and rgba forms getComputedStyle resolves to', () => {
        expect(parseColor('rgb(33, 36, 41)')).toEqual({r: 33, g: 36, b: 41, a: 1})
        expect(parseColor('rgba(255, 255, 255, 0.5)')).toEqual({r: 255, g: 255, b: 255, a: 0.5})
    })

    it('treats a fully transparent colour as one with no alpha, not as black', () => {
        // The "transparent" keyword resolves to this, and an element carrying it defines no background of
        // its own: the surface above it still governs.
        expect(parseColor('rgba(0, 0, 0, 0)')?.a).toBe(0)
    })

    it('returns null for anything it cannot read', () => {
        expect(parseColor('')).toBeNull()
        expect(parseColor('none')).toBeNull()
        expect(parseColor('rgb(1, 2)')).toBeNull()
        expect(parseColor('rgb(a, b, c)')).toBeNull()
    })
})

describe('composite', () => {
    it('lays a translucent colour over an opaque one', () => {
        const half = composite({r: 0, g: 0, b: 0, a: 0.5}, WHITE)
        expect(half).toEqual({r: 127.5, g: 127.5, b: 127.5, a: 1})
    })

    it('leaves an opaque colour untouched and ignores a fully transparent one', () => {
        expect(composite(BLACK, WHITE)).toEqual({r: 0, g: 0, b: 0, a: 1})
        expect(composite({r: 0, g: 0, b: 0, a: 0}, WHITE)).toEqual(WHITE)
    })
})

describe('relativeLuminance', () => {
    it('spans black to white', () => {
        expect(relativeLuminance(BLACK)).toBe(0)
        expect(relativeLuminance(WHITE)).toBeCloseTo(1)
    })

    it('uses the sRGB low-end linear segment for very dark channels', () => {
        // Below the transfer function's threshold the curve is linear, not a power law; a near-black must
        // not round to zero or the dark test below stops discriminating.
        expect(relativeLuminance({r: 8, g: 8, b: 8, a: 1})).toBeGreaterThan(0)
        expect(relativeLuminance({r: 8, g: 8, b: 8, a: 1})).toBeLessThan(0.01)
    })
})

describe('contrastRatio', () => {
    it('runs from 1 for identical colours to 21 for black on white', () => {
        expect(contrastRatio(0, 0)).toBe(1)
        expect(contrastRatio(relativeLuminance(BLACK), relativeLuminance(WHITE))).toBeCloseTo(21)
    })

    it('is symmetric', () => {
        expect(contrastRatio(0.9, 0.1)).toBe(contrastRatio(0.1, 0.9))
    })

    it('scores white on a near-white fallback as effectively invisible', () => {
        // ClearScore's hero: white text with #EAF5F5 behind it once the background image is gone.
        const ratio = contrastRatio(relativeLuminance(WHITE), relativeLuminance({r: 234, g: 245, b: 245, a: 1}))
        expect(ratio).toBeLessThan(1.2)
    })
})

describe('paintsBackground', () => {
    it('counts only what can actually paint inside the frame', () => {
        expect(paintsBackground('url("data:image/png;base64,iVBOR")')).toBe(true)
        expect(paintsBackground('linear-gradient(to right, rgb(58, 155, 237), rgb(35, 94, 207))')).toBe(true)
    })

    it('does not count a stripped remote url, which resolves to the document and paints nothing', () => {
        // This is the case that matters: the body parser replaces every remote CSS url with an empty one, and
        // the browser resolves that against the frame's own document, so the computed value is a url even
        // though nothing renders. Reading it as an image would suppress the contrast repair on exactly the
        // elements that lost their backdrop, which is the ClearScore failure.
        expect(paintsBackground('url("http://localhost:8164/")')).toBe(false)
        expect(paintsBackground('url("https://assets.clearscore.com/transform/mcd_bg.jpg")')).toBe(false)
        expect(paintsBackground('none')).toBe(false)
        expect(paintsBackground('')).toBe(false)
    })
})

describe('isDarkBackground', () => {
    it('recognises the near-blacks and deep brand colours senders design against', () => {
        expect(isDarkBackground({r: 33, g: 36, b: 41, a: 1})).toBe(true) // Steam's #212429
        expect(isDarkBackground({r: 34, g: 32, b: 50, a: 1})).toBe(true) // Steam's #222032 price chip
        expect(isDarkBackground(BLACK)).toBe(true)
    })

    it('treats white, near-white and mid-grey as light, so they get inverted', () => {
        expect(isDarkBackground(WHITE)).toBe(false)
        expect(isDarkBackground({r: 234, g: 245, b: 245, a: 1})).toBe(false) // ClearScore's #EAF5F5
        expect(isDarkBackground({r: 128, g: 128, b: 128, a: 1})).toBe(false)
    })
})
