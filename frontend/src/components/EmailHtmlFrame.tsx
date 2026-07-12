import {useEffect, useRef} from 'react'

// The email renders on a paper surface with readable defaults. Most HTML email is authored only for a light
// background, so the default surface is white and the app darkens it by inverting the whole document (see
// darkModeStyle). An email that ships its own dark-mode styling is the exception: it renders natively on a
// dark paper instead, because inverting an already-dark email would turn it light again. These are the design
// tokens for both surfaces, named rather than inlined so the base stylesheet carries no bare magic numbers.
const PAPER_BACKGROUND = '#ffffff'
const PAPER_INK = '#1a1a1a'
const PAPER_BACKGROUND_DARK = '#1a1a1a'
const PAPER_INK_DARK = '#e6e6e6'
const FRAME_PADDING_PX = 12
const FRAME_FONT_SIZE_PX = 14
const FRAME_LINE_HEIGHT = 1.55
const FRAME_FONT_STACK = "-apple-system,BlinkMacSystemFont,'Segoe UI',system-ui,sans-serif"

// paperStyle gives the email a page with readable defaults and stops a wide image or table overflowing the
// reader. The message's own inline styles and <style> blocks layer on top of it. It is authored once and
// reused for the light paper, the inverted-dark base and the native-dark paper.
function paperStyle(background: string, ink: string): string {
    return (
        `html,body{margin:0;padding:${FRAME_PADDING_PX}px;background:${background};color:${ink};` +
        `font:${FRAME_FONT_SIZE_PX}px/${FRAME_LINE_HEIGHT} ${FRAME_FONT_STACK};overflow-wrap:break-word;}` +
        'img{max-width:100%;height:auto;}table{max-width:100%;}'
    )
}
const baseStyle = paperStyle(PAPER_BACKGROUND, PAPER_INK)
const baseStyleDark = paperStyle(PAPER_BACKGROUND_DARK, PAPER_INK_DARK)

// darkModeStyle renders the email dark when the app theme is dark. Virtually all HTML email is authored for a
// light background and never anticipates dark mode, so the only technique that darkens an arbitrary message
// (rather than only the few that ship a prefers-color-scheme variant) is to invert the whole light-designed
// document: a white background becomes dark and dark body text becomes light. Real media is then re-inverted
// with the same filter, which double-inverts it back to its true colours, so photos and logos still look
// right. The 180deg hue-rotate keeps hues recognisable rather than turning them complementary.
//
// The re-invert targets only leaf media: the replaced elements (img, picture, video, svg, canvas) plus a
// background image on an otherwise-empty box. It must never match a container that holds content. A CSS
// filter on an element and one on its descendant compound, so re-inverting a content-bearing box (a layout
// table cell carrying a background attribute or a background-image, as Amazon-style transactional email
// uses) double-inverts that whole subtree back to light: it defeats dark mode for the block and leaves the
// descendant images (a product thumbnail, a logo) inverted the wrong way. Restricting the background match
// to :empty keeps a genuinely decorative background image looking right while never flipping a content
// wrapper. A plain background-colour is never matched, so a coloured box keeps its inverted dark fill.
const DARK_INVERT_FILTER = 'invert(1) hue-rotate(180deg)'
const DARK_MEDIA_SELECTOR = 'img,picture,video,svg,canvas,[background]:empty,[style*="background-image"]:empty'
// DARK_MEDIA_FRAME is a border drawn around re-inverted media in dark mode. Keeping media at its true colour
// is right for a photo. A genuinely dark image (a book cover, a dark logo) was designed to sit on the
// sender's light page, so once the surround inverts to dark it has no contrast and reads as a dark block on
// a dark cell; the frame gives it a visible edge. The colour rides the filtered media, so it renders as a
// mid-grey line against both the dark image and the dark surround. It is !important because HTML email almost
// universally resets image borders (an inline border:0 that kills the old linked-image border), which would
// otherwise beat this stylesheet rule and leave the frame off exactly the product images that need it.
// box-sizing keeps the border from resizing a fixed-dimension image. The frame also lands faintly on a spacer
// or tracking image, an accepted cost for readable covers.
const DARK_MEDIA_FRAME = '2px solid #808080'
const darkModeStyle =
    `html{filter:${DARK_INVERT_FILTER};}` +
    `${DARK_MEDIA_SELECTOR}{filter:${DARK_INVERT_FILTER};border:${DARK_MEDIA_FRAME} !important;box-sizing:border-box;}`

// The iframe is the security boundary. Its Content-Security-Policy grants no script-src (so no JavaScript runs
// even if some slipped past the sanitiser), blocks every default source and permits only inline styles, data:
// fonts plus data: images. It never allows a remote http/https image: a message's remote images are fetched
// server-side and inlined as data: URIs before they reach the frame (see the LoadRemoteImages proxy), so the
// frame makes no remote request at all and cannot leak that a message was opened, even for an image whose
// server-side fetch failed and stayed parked.
const CONTENT_SECURITY_POLICY = "default-src 'none'; style-src 'unsafe-inline'; img-src data:; font-src data:;"

// LINK_SCHEMES are the URL schemes a link inside the email may open externally; any other scheme is ignored.
const LINK_SCHEMES = ['http:', 'https:', 'mailto:']

interface EmailHtmlFrameProps {
    // html is the already sanitised, image-parked-or-inlined message body the parent computed. When the reader
    // has asked for images, the parent passes the proxy-resolved HTML whose remote images are inlined as data:
    // URIs; otherwise it passes the parked HTML, which shows no images.
    html: string
    // dark renders the email to match the app's dark theme. A light-designed message is inverted inside the
    // frame; a message that carries its own dark-mode styling is left to render natively on a dark paper. It
    // is false in the light theme, where the email keeps its faithful white surface.
    dark: boolean
    // onOpenLink receives an http/https/mailto href when a link inside the email is clicked; the parent
    // opens it in the external browser rather than letting it navigate the frame or the app.
    onOpenLink: (href: string) => void
}

// EMAIL_DARK_MODE_SIGNAL detects an email that ships its own dark-mode styling. A prefers-color-scheme:dark
// media query is the reliable, standards-based signal, used heavily by large senders such as Amazon. When it
// is present the frame lets the message darken itself (the frame reports the app's dark scheme, so the
// message's own dark rules apply) and skips the invert: inverting an email that has already darkened itself
// flips it back to light, its dark background becoming a light page with dark text and a forced-white product
// tile becoming black.
const EMAIL_DARK_MODE_SIGNAL = /prefers-color-scheme\s*:\s*dark/i

// htmlSupportsDarkMode reports whether the message carries its own dark-mode styling, so the frame can let it
// render natively rather than inverting it.
function htmlSupportsDarkMode(html: string): boolean {
    return EMAIL_DARK_MODE_SIGNAL.test(html)
}

// buildSrcDoc assembles the self-contained document the iframe renders through its srcdoc attribute. It is a
// full HTML document so the CSP meta tag governs the message; the sanitised body is dropped in verbatim. The
// theme decides the surface, not the body: a light message is inverted to darken it, while a message that
// carries its own dark mode renders natively on a dark paper.
function buildSrcDoc(html: string, dark: boolean): string {
    let surfaceStyle: string
    if (!dark) {
        surfaceStyle = baseStyle
    } else if (htmlSupportsDarkMode(html)) {
        surfaceStyle = baseStyleDark
    } else {
        surfaceStyle = baseStyle + darkModeStyle
    }
    return '<!doctype html><html><head><meta charset="utf-8">' +
        `<meta http-equiv="Content-Security-Policy" content="${CONTENT_SECURITY_POLICY}">` +
        `<style>${surfaceStyle}</style></head><body>${html}</body></html>`
}

// isOpenableHref reports whether a link's href is one we open externally (http, https or mailto). Other
// schemes (javascript:, data:, cid:, tel: and the rest) are ignored so a click cannot do anything unexpected.
function isOpenableHref(href: string): boolean {
    const scheme = href.trim().toLowerCase()
    return LINK_SCHEMES.some((s) => scheme.startsWith(s))
}

// resizeToContent grows the iframe to its content height so the email has no inner scrollbar of its own; the
// reader pane scrolls instead. contentDocument can be null transiently, so every access is guarded.
function resizeToContent(frame: HTMLIFrameElement) {
    const root = frame.contentDocument?.documentElement
    if (root) {
        frame.style.height = `${root.scrollHeight}px`
    }
}

// EmailHtmlFrame renders a message's sanitised HTML inside a sandboxed, CSP-locked iframe so the email keeps
// its own fonts, colours and layout while staying fully isolated from the app. The sandbox is
// allow-same-origin only (never allow-scripts), so no script in the frame can run or remove the sandbox.
// Because the frame is same-origin, the parent reads its height and intercepts its link clicks directly, so
// no script inside the frame is needed.
export function EmailHtmlFrame({html, dark, onOpenLink}: EmailHtmlFrameProps) {
    const frameRef = useRef<HTMLIFrameElement>(null)
    // The link callback is held in a ref so a new callback identity from the parent does not re-run the
    // binding effect (which would needlessly rebind on every parent render).
    const onOpenLinkRef = useRef(onOpenLink)
    onOpenLinkRef.current = onOpenLink

    const doc = buildSrcDoc(html, dark)

    useEffect(() => {
        const frame = frameRef.current
        if (!frame) {
            return
        }
        let observer: ResizeObserver | null = null
        let boundDocument: Document | null = null

        const onClick = (event: Event) => {
            const target = event.target as Element | null
            if (!target || typeof target.closest !== 'function') {
                return
            }
            const anchor = target.closest('a')
            if (!anchor) {
                return
            }
            // Never let a click navigate the frame (which could load a remote page and leak the open). Open
            // only an http, https or mailto link externally; ignore any other scheme.
            event.preventDefault()
            const href = anchor.getAttribute('href') ?? ''
            if (href && isOpenableHref(href)) {
                onOpenLinkRef.current(href)
            }
        }

        const detach = () => {
            observer?.disconnect()
            observer = null
            boundDocument?.removeEventListener('click', onClick)
            boundDocument = null
        }

        // attach binds the link handler and size tracking to the frame's current document. It runs on every
        // load (the srcdoc reparses whenever html or imagesShown change) and rebinds cleanly each time.
        const attach = () => {
            detach()
            const contentDocument = frame.contentDocument
            if (!contentDocument) {
                return
            }
            boundDocument = contentDocument
            contentDocument.addEventListener('click', onClick)
            resizeToContent(frame)
            // A ResizeObserver on the body keeps the height correct as late images load or the layout
            // reflows. It is absent in some environments (jsdom), so its use is guarded.
            const body = contentDocument.body
            if (body && typeof ResizeObserver !== 'undefined') {
                observer = new ResizeObserver(() => resizeToContent(frame))
                observer.observe(body)
            }
        }

        frame.addEventListener('load', attach)
        // The document is usually parsed by the time this effect runs, so bind once now as well as on load.
        attach()
        return () => {
            frame.removeEventListener('load', attach)
            detach()
        }
    }, [doc])

    return (
        <iframe
            ref={frameRef}
            className="reader-html-frame"
            title="Email content"
            sandbox="allow-same-origin"
            srcDoc={doc}
        />
    )
}
