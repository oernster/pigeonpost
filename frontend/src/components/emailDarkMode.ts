// Colour treatment for a rendered email, applied to the frame's document after it is written.
//
// A single document-wide invert cannot render HTML email correctly, because one message routinely mixes
// light-designed and dark-designed regions. Steam's wishlist mail is the worked example: a dark panel
// (bgcolor #212429, white text) sitting inside a white wrapper with a white footer. Inverting the whole
// document darkens the wrapper and the footer correctly and flips the dark panel to light, so half the
// message ends up upside down. Classifying the whole message as light or dark does not help either: whichever
// way that coin lands, the other half is wrong.
//
// So the treatment is per element. The document is inverted at the root, then any region the sender already
// designed dark is inverted a second time, which returns it to exactly the colours its author chose. Because
// a CSS filter on an element and one on its descendants compound, what matters at any point in the tree is
// the PARITY of the filters above it: odd means the region is currently shown inverted, even means it is
// shown as authored. The walk carries that parity down and flips an element only where its parity disagrees
// with the background the author gave it. The rule it enforces is uniform: every region ends up rendering
// dark, either because a light design was inverted or because a dark design was left alone.
//
// The same walk repairs text that has lost its contrast, which is a separate defect the invert only made
// more visible: see repairTextContrast.

// INVERT_FILTER darkens a light-designed region. The 180deg hue-rotate keeps hues recognisable rather than
// turning them complementary, and applying the identical filter twice returns a region to its true colours.
const INVERT_FILTER = 'invert(1) hue-rotate(180deg)'

// DARK_BACKGROUND_MAX_LUMINANCE is the relative luminance at or below which a background counts as one the
// sender already designed dark, so it is left rendering as authored instead of being inverted. Relative
// luminance is not perceptual and compresses hard at the dark end, so this sits well below the midpoint: it
// admits near-blacks and deep brand colours (Steam's #212429 is 0.017, a deep navy about 0.045) while a
// mid-grey (#808080, 0.216) or a light grey still counts as light and gets inverted. A mid-grey inverts to
// roughly itself, so the boundary is forgiving in both directions.
const DARK_BACKGROUND_MAX_LUMINANCE = 0.15

// MIN_TEXT_CONTRAST is the contrast ratio below which text counts as unreadable against its own background
// and is repaired. It is deliberately far under any accessibility target (WCAG's floor for large text is 3):
// the aim is only to rescue text that is effectively invisible, not to second-guess a sender's palette. White
// on ClearScore's #EAF5F5 fallback is 1.1 and is caught; ordinary low-contrast styling is left alone.
const MIN_TEXT_CONTRAST = 2

// The inks a repaired text colour is set to, chosen against the background it sits on. They match the frame's
// paper tokens, so repaired text reads as the frame's own default text rather than as a third colour.
const DARK_INK = '#1a1a1a'
const LIGHT_INK = '#e6e6e6'

// MID_LUMINANCE splits backgrounds into ones that want dark ink and ones that want light ink.
const MID_LUMINANCE = 0.5

// MEDIA_TAGS are the leaf replaced elements that must always end up at their true colours: a photo or a logo
// inverted to its negative is worse than useless. A container is never treated as media, because flipping a
// content-bearing box flips its whole subtree with it.
const MEDIA_TAGS = new Set(['img', 'picture', 'video', 'svg', 'canvas'])

// MEDIA_FRAME is drawn around media that had to be flipped back to true colour inside an inverted region. A
// genuinely dark image (a book cover, a dark logo) was designed to sit on the sender's light page, so once
// that page inverts to dark the image has no edge and reads as a dark block on a dark surround. Media inside
// a region the sender designed dark needs no frame: the author already composed it against a dark backdrop.
const MEDIA_FRAME = '2px solid #808080'

// TREATED_MARKER records that a document has already been walked, so a second attach (an engine that fires
// load after the document was written) cannot apply the treatment twice.
const TREATED_MARKER = 'ppTreated'

// The sRGB relative-luminance coefficients and transfer-function constants, from the sRGB specification as
// used by WCAG. They are fixed properties of the colour space, not tunable values.
const LUMINANCE_COEFFICIENTS = {red: 0.2126, green: 0.7152, blue: 0.0722}
const SRGB_LOW_THRESHOLD = 0.03928
const SRGB_LOW_DIVISOR = 12.92
const SRGB_OFFSET = 0.055
const SRGB_SCALE = 1.055
const SRGB_EXPONENT = 2.4
const CHANNEL_MAX = 255
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

interface Rgb {
    r: number
    g: number
    b: number
    a: number
}

// Surface is the background a piece of text actually sits on: the nearest opaque colour above it in the tree,
// plus whether any element between here and there paints a background image. An image hides the colour, so
// text over one is never judged against that colour.
interface Surface {
    color: Rgb
    hasImage: boolean
}

// PAPER is the frame's own background, the surface everything starts on.
const PAPER: Surface = {color: {r: CHANNEL_MAX, g: CHANNEL_MAX, b: CHANNEL_MAX, a: 1}, hasImage: false}

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

function isMedia(el: Element): boolean {
    return MEDIA_TAGS.has(el.tagName.toLowerCase())
}

// hasOwnText reports whether an element directly holds text, rather than only holding other elements. Only
// such an element has a text colour worth repairing; a wrapper's colour is repaired on whichever descendant
// actually carries the words.
function hasOwnText(el: Element): boolean {
    return Array.from(el.childNodes).some(
        (node) => node.nodeType === node.TEXT_NODE && (node.textContent ?? '').trim() !== '',
    )
}

// applyFilter adds the invert to an element, preserving any filter the sender set rather than dropping it.
function applyFilter(el: HTMLElement, current: string): void {
    const existing = current && current !== 'none' ? `${current} ` : ''
    el.style.setProperty('filter', `${existing}${INVERT_FILTER}`, 'important')
}

// repairTextContrast rescues text that is unreadable against its own background. The common cause is a
// background image that did not render: a sender colours a heading white to sit on a dark photo and leaves a
// pale background-colour as the fallback, so with the image absent the text is white on near-white. That is
// invisible in the light theme too; the invert only turns it from white-on-white into black-on-black. The
// repair is skipped wherever a background image is actually painting, because then the colour underneath is
// not what the reader sees and the sender's choice is the right one.
//
// It runs in the authored colour space, before any of the parity filters take effect, so a repaired colour is
// inverted along with the background it was measured against and stays readable either way.
function repairTextContrast(el: HTMLElement, color: string, surface: Surface): void {
    if (surface.hasImage || !hasOwnText(el)) {
        return
    }
    const parsed = parseColor(color)
    if (!parsed) {
        return
    }
    const backgroundLuminance = relativeLuminance(surface.color)
    const textLuminance = relativeLuminance(composite(parsed, surface.color))
    if (contrastRatio(textLuminance, backgroundLuminance) >= MIN_TEXT_CONTRAST) {
        return
    }
    el.style.setProperty('color', backgroundLuminance > MID_LUMINANCE ? DARK_INK : LIGHT_INK, 'important')
}

// treatElement applies the treatment to one element and recurses into its children. parity counts the filters
// in force above this element: odd means the region currently renders inverted, even means it renders as the
// sender authored it.
function treatElement(el: HTMLElement, view: Window, parity: number, surface: Surface, dark: boolean): void {
    const style = view.getComputedStyle(el)
    let nextParity = parity
    let nextSurface = surface

    const hasImage = paintsBackground(style.backgroundImage)
    const own = parseColor(style.backgroundColor)
    if (own && own.a > 0) {
        const blended = composite(own, surface.color)
        nextSurface = {color: blended, hasImage}
        // Flip where the parity disagrees with the design: a light background must render inverted (odd) and
        // a dark one must render as authored (even). A background image that actually paints also renders as
        // authored, for the same reason leaf media does: the sender composed the picture and the text over it
        // together, and inverting a photograph to its negative is worse than leaving a region bright.
        const wantsAuthored = isDarkBackground(blended) || hasImage
        if (dark && (nextParity % 2 === 0) !== wantsAuthored) {
            applyFilter(el, style.filter)
            nextParity += 1
        }
    } else if (hasImage) {
        nextSurface = {color: surface.color, hasImage: true}
    }

    // Media must always land on an even parity, so a photo or a logo shows its true colours.
    if (dark && isMedia(el) && nextParity % 2 === 1) {
        applyFilter(el, style.filter)
        el.style.setProperty('border', MEDIA_FRAME, 'important')
        el.style.setProperty('box-sizing', 'border-box', 'important')
        nextParity += 1
    }

    repairTextContrast(el, style.color, nextSurface)

    for (const child of Array.from(el.children)) {
        treatElement(child as HTMLElement, view, nextParity, nextSurface, dark)
    }
}

// applyEmailColorTreatment walks a written email document and colours it for the app's theme. In the dark
// theme it inverts the document and then corrects region by region, so a message that mixes light and dark
// designs renders dark throughout. In both themes it repairs text that cannot be read against its own
// background. It is safe to call more than once on the same document: the second call does nothing.
export function applyEmailColorTreatment(doc: Document, dark: boolean): void {
    const view = doc.defaultView
    const root = doc.documentElement
    if (!view || !root || !doc.body || root.dataset[TREATED_MARKER] !== undefined) {
        return
    }
    root.dataset[TREATED_MARKER] = ''
    if (dark) {
        root.style.setProperty('filter', INVERT_FILTER, 'important')
    }
    treatElement(doc.body, view, dark ? 1 : 0, PAPER, dark)
}
