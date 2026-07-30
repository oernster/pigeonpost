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
//
// The colour arithmetic the walk decides with lives in ../emailColors, where it carries the front end's
// 100% coverage gate.
import {
    CHANNEL_MAX,
    composite,
    contrastRatio,
    isDarkBackground,
    paintsBackground,
    parseColor,
    relativeLuminance,
    type Rgb,
} from '../emailColors'

// INVERT_FILTER darkens a light-designed region. The 180deg hue-rotate keeps hues recognisable rather than
// turning them complementary, and applying the identical filter twice returns a region to its true colours.
const INVERT_FILTER = 'invert(1) hue-rotate(180deg)'

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

// Surface is the background a piece of text actually sits on: the nearest opaque colour above it in the tree,
// plus whether any element between here and there paints a background image. An image hides the colour, so
// text over one is never judged against that colour.
interface Surface {
    color: Rgb
    hasImage: boolean
}

// PAPER is the frame's own background, the surface everything starts on.
const PAPER: Surface = {color: {r: CHANNEL_MAX, g: CHANNEL_MAX, b: CHANNEL_MAX, a: 1}, hasImage: false}

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
