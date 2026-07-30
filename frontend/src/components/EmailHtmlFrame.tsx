import {useEffect, useRef} from 'react'

// The email renders on a paper surface with readable defaults. Every message is treated as light-designed:
// the frame pins the document to the light colour scheme (see FORCED_LIGHT_SCHEME) and the app darkens it by
// inverting the whole document in the dark theme (see darkModeStyle). These are the design tokens for that
// surface, named rather than inlined so the base stylesheet carries no bare magic numbers.
const PAPER_BACKGROUND = '#ffffff'
const PAPER_INK = '#1a1a1a'
const FRAME_PADDING_PX = 12
// 15px with a 1.6 leading, matching the app's own reading surfaces: email is read at length, so the frame
// gets the full base size rather than a compacted UI size. Segoe UI Variable Text leads the stack for the
// same reason as the app stylesheet: it is the Windows 11 text face and renders clearer in WebView2 than
// classic Segoe UI, which stays behind it as the fallback.
const FRAME_FONT_SIZE_PX = 15
const FRAME_LINE_HEIGHT = 1.6
const FRAME_FONT_STACK = "'Segoe UI Variable Text','Segoe UI',-apple-system,BlinkMacSystemFont,system-ui,sans-serif"

// The frame is pinned to the light colour scheme whatever the app theme is, so the message always renders in
// its light design and the dark theme has one uniform document to invert. Without the pin the frame inherits
// the app's dark scheme and any prefers-color-scheme:dark rules the message carries switch themselves on.
// That is the blinding case: dark-mode support in HTML email is almost always partial, a media query that
// recolours a handful of elements while the bulk of the page stays white through bgcolor attributes and
// inline styles no media query can reach, so the message ends up mostly white with a few dark patches.
//
// The pin that actually governs prefers-color-scheme is FRAME_COLOR_SCHEME, set on the iframe ELEMENT in this
// document: an embedded document takes its colour-scheme preference from its embedder. Declaring
// color-scheme inside the frame's own document does NOT change it (measured in Chromium/WebView2: with the
// declaration in the frame, prefers-color-scheme:dark still matched and a partial-dark email still darkened
// itself; with it on the element, the same email rendered light). FORCED_LIGHT_SCHEME keeps the declaration
// in the document too, where it still sets the used scheme for UA-rendered controls inside the message. The
// print path pins its frame the same way, on the element (see print.ts).
const FRAME_COLOR_SCHEME = 'light'
const FORCED_LIGHT_SCHEME = `:root{color-scheme:${FRAME_COLOR_SCHEME};}`

// baseStyle gives the email a page with readable defaults and stops a wide image or table overflowing the
// reader. The message's own inline styles and <style> blocks layer on top of it.
const baseStyle =
    FORCED_LIGHT_SCHEME +
    `html,body{margin:0;padding:${FRAME_PADDING_PX}px;background:${PAPER_BACKGROUND};color:${PAPER_INK};` +
    `font:${FRAME_FONT_SIZE_PX}px/${FRAME_LINE_HEIGHT} ${FRAME_FONT_STACK};overflow-wrap:break-word;}` +
    'img{max-width:100%;height:auto;}table{max-width:100%;}' +
    // A link that stands alone on its own line (marked pp-solo-link by the body parser) reads as a
    // call to action, so it is presented as a button rather than a raw link.
    'a.pp-solo-link{display:inline-block;margin:4px 0;padding:9px 20px;border-radius:18px;' +
    'background:#2f6fed;color:#ffffff;text-decoration:none;font-weight:600;}'

// darkModeStyle renders the email dark when the app theme is dark. HTML email is authored for a light
// background and, where it anticipates dark mode at all, does so only partially, so the only technique that
// darkens an arbitrary message is to invert the whole light-designed document (which FORCED_LIGHT_SCHEME
// guarantees it is): a white background becomes dark and dark body text becomes light. Real media is then re-inverted
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
    // dark renders the email to match the app's dark theme by inverting the message inside the frame. It is
    // false in the light theme, where the email keeps its faithful white surface.
    dark: boolean
    // onOpenLink receives an http/https/mailto href when a link inside the email is clicked; the parent
    // opens it in the external browser rather than letting it navigate the frame or the app.
    onOpenLink: (href: string) => void
}

// buildFrameDocument assembles the self-contained document the parent writes into the iframe. It is a
// full HTML document so the CSP meta tag governs the message; the sanitised body is dropped in verbatim. The
// theme decides the surface, not the body: the message always renders in its light design (pinned by the
// colour-scheme meta and FORCED_LIGHT_SCHEME) and the dark theme inverts that whole design.
function buildFrameDocument(html: string, dark: boolean): string {
    const surfaceStyle = dark ? baseStyle + darkModeStyle : baseStyle
    return '<!doctype html><html><head><meta charset="utf-8">' +
        `<meta name="color-scheme" content="${FRAME_COLOR_SCHEME}">` +
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
// its own fonts, colours and layout while staying fully isolated from the app. The sandbox grants
// allow-same-origin plus allow-scripts and nothing else: popups, top navigation, forms and downloads all stay
// blocked. allow-same-origin is what lets the parent read the frame's height and intercept its link clicks
// directly. allow-scripts is not there so email scripts can run: the CSP grants no script-src, so no script
// inside the document can ever execute, and the sanitiser has already stripped scripts server-side. It is
// there because WebKit (WKWebView on macOS, WebKitGTK on Linux) refuses to dispatch event listeners inside a
// scripts-disabled browsing context, including listeners the parent registered on the frame's document, so
// with a scriptless sandbox the click handler never ran and a link click did nothing. Chromium keys the same
// check off the listener's own context instead, which is why Windows always worked. Script execution is
// denied by the CSP layer rather than the sandbox flag.
//
// The document is written from the effect via contentDocument.open()/write()/close() rather than through the
// srcdoc attribute. With srcdoc, WebKit (WKWebView on macOS, WebKitGTK on Linux) replaces the frame's
// document without reliably firing the load event, so the click listener stayed bound to the dead initial
// about:blank document and a button click navigated the sandboxed frame inline instead of opening the
// external browser. Writing the document means the parent creates and owns the very document object it binds
// to, on every engine, deterministically; the sandbox attribute and the CSP meta still apply to the written
// content.
export function EmailHtmlFrame({html, dark, onOpenLink}: EmailHtmlFrameProps) {
    const frameRef = useRef<HTMLIFrameElement>(null)
    // The link callback is held in a ref so a new callback identity from the parent does not re-run the
    // binding effect (which would needlessly rebind on every parent render).
    const onOpenLinkRef = useRef(onOpenLink)
    onOpenLinkRef.current = onOpenLink

    const doc = buildFrameDocument(html, dark)

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

        // attach binds the link handler and size tracking to the frame's current document. It runs right
        // after the document is written and again on any load event, rebinding cleanly each time, so even an
        // engine that swaps the document object behind the frame ends up with the listener on the live one.
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
        // The parent writes the frame's document itself (see the component comment: srcdoc leaves the click
        // listener on a dead document under WebKit). open() reuses the existing same-origin document object,
        // so the attach that follows binds to exactly the document the message was parsed into.
        const contentDocument = frame.contentDocument
        if (contentDocument) {
            contentDocument.open()
            contentDocument.write(doc)
            contentDocument.close()
        }
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
            // The pin that governs prefers-color-scheme inside the message: it has to sit on the element,
            // not in the frame's own document (see FRAME_COLOR_SCHEME).
            style={{colorScheme: FRAME_COLOR_SCHEME}}
            sandbox="allow-same-origin allow-scripts"
        />
    )
}
