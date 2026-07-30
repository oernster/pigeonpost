import {useEffect, useRef} from 'react'

import {applyEmailColorTreatment} from './emailDarkMode'

// The email renders on a paper surface with readable defaults. The message is pinned to the light colour
// scheme (see FRAME_COLOR_SCHEME) so it always renders in the one design its author finished, and the
// document is then coloured for the app's theme element by element (see emailDarkMode). These are the design
// tokens for the paper, named rather than inlined so the base stylesheet carries no bare magic numbers.
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
    // dark colours the email to match the app's dark theme, region by region, so a message that mixes
    // light-designed and dark-designed blocks renders dark throughout. It is false in the light theme, where
    // the email keeps its faithful white surface.
    dark: boolean
    // onOpenLink receives an http/https/mailto href when a link inside the email is clicked; the parent
    // opens it in the external browser rather than letting it navigate the frame or the app.
    onOpenLink: (href: string) => void
}

// buildFrameDocument assembles the self-contained document the parent writes into the iframe. It is a
// full HTML document so the CSP meta tag governs the message; the sanitised body is dropped in verbatim. The
// document itself is theme-independent: the message always renders in its light design (pinned by the
// colour-scheme meta and FORCED_LIGHT_SCHEME) and the theme is applied afterwards by the colour treatment,
// which needs the laid-out document to read each region's authored background.
function buildFrameDocument(html: string): string {
    return '<!doctype html><html><head><meta charset="utf-8">' +
        `<meta name="color-scheme" content="${FRAME_COLOR_SCHEME}">` +
        `<meta http-equiv="Content-Security-Policy" content="${CONTENT_SECURITY_POLICY}">` +
        `<style>${baseStyle}</style></head><body>${html}</body></html>`
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

    const doc = buildFrameDocument(html)

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
            // Colour the message for the theme. This runs synchronously after the write, before the frame can
            // paint, so the reader never sees the untreated document flash. The treatment marks the document
            // it has walked, so the second attach an engine's load event triggers is a no-op rather than a
            // second pass that would invert everything back.
            applyEmailColorTreatment(contentDocument, dark)
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
    }, [doc, dark])

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
