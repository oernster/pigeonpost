// The pure colour arithmetic behind the reader's email colour treatment: colour parsing, alpha
// compositing, sRGB relative luminance, contrast ratio and the judgements built on them. Everything
// here is a plain function of its inputs, no DOM access and no framework, so it carries the front
// end's 100% coverage gate. The DOM walk that applies these judgements per element lives in
// components/emailDarkMode.ts.

// DARK_BACKGROUND_MAX_LUMINANCE is the relative luminance at or below which a background counts as one the
// sender already designed dark, so it is left rendering as authored instead of being inverted. Relative
// luminance is not perceptual and compresses hard at the dark end, so this sits well below the midpoint: it
// admits near-blacks and deep brand colours (Steam's #212429 is 0.017, a deep navy about 0.045) while a
// mid-grey (#808080, 0.216) or a light grey still counts as light and gets inverted. A mid-grey inverts to
// roughly itself, so the boundary is forgiving in both directions.
const DARK_BACKGROUND_MAX_LUMINANCE = 0.15

// The sRGB relative-luminance coefficients and transfer-function constants, from the sRGB specification as
// used by WCAG. They are fixed properties of the colour space, not tunable values.
const LUMINANCE_COEFFICIENTS = {red: 0.2126, green: 0.7152, blue: 0.0722}
const SRGB_LOW_THRESHOLD = 0.03928
const SRGB_LOW_DIVISOR = 12.92
const SRGB_OFFSET = 0.055
const SRGB_SCALE = 1.055
const SRGB_EXPONENT = 2.4
export const CHANNEL_MAX = 255
// The offset in the WCAG contrast-ratio formula, which keeps the ratio finite for two blacks.
const CONTRAST_OFFSET = 0.05

// PAINTING_BACKGROUND matches a background-image value that can actually paint inside the frame, which is
// what decides whether the colour underneath is the one the reader sees. The frame's CSP admits data: images
// only, and a remote CSS url is stripped to an empty url() before the body ever reaches the frame. An empty
// url() is not nothing: the browser resolves it against the document, so the computed background-image reads
// as a url rather than "none" even though no pixel is ever painted. Testing for "not none" therefore reports
// an image on exactly the elements that have lost theirs, which is the opposite of what is needed, so only a
// data: image or a CSS gradient counts.
const PAINTING_BACKGROUND = /url\(\s*["']?data:|gradient\(/i

export interface Rgb {
    r: number
    g: number
    b: number
    a: number
}

const COLOR_PATTERN = /^rgba?\(([^)]+)\)/

// parseColor reads a computed colour. getComputedStyle always resolves to rgb() or rgba(), so no named or hex
// form has to be understood here. An unparseable value (including the "transparent" keyword, which resolves
// to rgba(0, 0, 0, 0) and is handled by its zero alpha) yields null.
export function parseColor(value: string): Rgb | null {
    const match = COLOR_PATTERN.exec(value.trim())
    if (!match) {
        return null
    }
    const parts = match[1].split(',').map((p) => Number(p.trim()))
    if (parts.length < 3 || parts.slice(0, 3).some((n) => !Number.isFinite(n))) {
        return null
    }
    const alpha = parts.length > 3 && Number.isFinite(parts[3]) ? parts[3] : 1
    return {r: parts[0], g: parts[1], b: parts[2], a: alpha}
}

// composite lays a possibly translucent colour over an opaque one and returns the opaque result, so a tinted
// overlay is judged by what the reader actually sees rather than by its own nominal colour.
export function composite(top: Rgb, bottom: Rgb): Rgb {
    const alpha = Math.min(Math.max(top.a, 0), 1)
    return {
        r: top.r * alpha + bottom.r * (1 - alpha),
        g: top.g * alpha + bottom.g * (1 - alpha),
        b: top.b * alpha + bottom.b * (1 - alpha),
        a: 1,
    }
}

function channelLuminance(value: number): number {
    const scaled = value / CHANNEL_MAX
    if (scaled <= SRGB_LOW_THRESHOLD) {
        return scaled / SRGB_LOW_DIVISOR
    }
    return Math.pow((scaled + SRGB_OFFSET) / SRGB_SCALE, SRGB_EXPONENT)
}

// relativeLuminance is the sRGB relative luminance of an opaque colour, 0 for black and 1 for white.
export function relativeLuminance(color: Rgb): number {
    return (
        LUMINANCE_COEFFICIENTS.red * channelLuminance(color.r) +
        LUMINANCE_COEFFICIENTS.green * channelLuminance(color.g) +
        LUMINANCE_COEFFICIENTS.blue * channelLuminance(color.b)
    )
}

// contrastRatio is the WCAG contrast ratio between two luminances, from 1 (identical) to 21 (black on white).
export function contrastRatio(first: number, second: number): number {
    const lighter = Math.max(first, second)
    const darker = Math.min(first, second)
    return (lighter + CONTRAST_OFFSET) / (darker + CONTRAST_OFFSET)
}

// isDarkBackground reports whether a background is one the sender already designed dark, so that a region
// carrying it should render as authored rather than be inverted.
export function isDarkBackground(color: Rgb): boolean {
    return relativeLuminance(color) <= DARK_BACKGROUND_MAX_LUMINANCE
}

// paintsBackground reports whether a computed background-image value actually covers the colour beneath it.
export function paintsBackground(value: string): boolean {
    return value !== '' && value !== 'none' && PAINTING_BACKGROUND.test(value)
}
