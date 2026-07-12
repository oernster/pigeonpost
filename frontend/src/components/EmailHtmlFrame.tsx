import {useEffect, useRef} from 'react'

// The email renders on its own white paper surface with readable defaults, independent of the app's dark
// theme, because virtually all HTML email is designed for a light background. These are the design tokens
// for that surface, named rather than inlined so the base stylesheet carries no bare magic numbers.
const PAPER_BACKGROUND = '#ffffff'
const PAPER_INK = '#1a1a1a'
const FRAME_PADDING_PX = 12
const FRAME_FONT_SIZE_PX = 14
const FRAME_LINE_HEIGHT = 1.55
const FRAME_FONT_STACK = "-apple-system,BlinkMacSystemFont,'Segoe UI',system-ui,sans-serif"

// baseStyle gives the email a white page with readable defaults and stops a wide image or table overflowing
// the reader. The message's own inline styles and <style> blocks layer on top of it.
const baseStyle =
    `html,body{margin:0;padding:${FRAME_PADDING_PX}px;background:${PAPER_BACKGROUND};color:${PAPER_INK};` +
    `font:${FRAME_FONT_SIZE_PX}px/${FRAME_LINE_HEIGHT} ${FRAME_FONT_STACK};overflow-wrap:break-word;}` +
    'img{max-width:100%;height:auto;}table{max-width:100%;}'

// The iframe is the security boundary. Its Content-Security-Policy grants no script-src (so no JavaScript
// runs even if some slipped past the sanitiser), blocks every default source and only permits inline styles,
// data: fonts plus images. img-src additionally allows remote http/https images once the reader opts in; a
// message's remote images then load only on request.
const CSP_IMG_SRC_BLOCKED = 'img-src data:'
const CSP_IMG_SRC_SHOWN = 'img-src data: https: http:'

// LINK_SCHEMES are the URL schemes a link inside the email may open externally; any other scheme is ignored.
const LINK_SCHEMES = ['http:', 'https:', 'mailto:']

interface EmailHtmlFrameProps {
    // html is the already sanitised, image-parked-or-unparked message body the parent computed.
    html: string
    // imagesShown widens the CSP img-src to allow remote images once the reader has opted in.
    imagesShown: boolean
    // onOpenLink receives an http/https/mailto href when a link inside the email is clicked; the parent
    // opens it in the external browser rather than letting it navigate the frame or the app.
    onOpenLink: (href: string) => void
}

// contentSecurityPolicy builds the CSP for the srcdoc. Only img-src changes with the images toggle;
// everything else stays locked down.
function contentSecurityPolicy(imagesShown: boolean): string {
    const imgSrc = imagesShown ? CSP_IMG_SRC_SHOWN : CSP_IMG_SRC_BLOCKED
    return `default-src 'none'; style-src 'unsafe-inline'; ${imgSrc}; font-src data:;`
}

// buildSrcDoc assembles the self-contained document the iframe renders through its srcdoc attribute. It is a
// full HTML document so the CSP meta tag governs the message; the sanitised body is dropped in verbatim.
function buildSrcDoc(html: string, imagesShown: boolean): string {
    return '<!doctype html><html><head><meta charset="utf-8">' +
        `<meta http-equiv="Content-Security-Policy" content="${contentSecurityPolicy(imagesShown)}">` +
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
// its own fonts, colours and layout while staying fully isolated from the app. The sandbox is
// allow-same-origin only (never allow-scripts), so no script in the frame can run or remove the sandbox.
// Because the frame is same-origin, the parent reads its height and intercepts its link clicks directly, so
// no script inside the frame is needed.
export function EmailHtmlFrame({html, imagesShown, onOpenLink}: EmailHtmlFrameProps) {
    const frameRef = useRef<HTMLIFrameElement>(null)
    // The link callback is held in a ref so a new callback identity from the parent does not re-run the
    // binding effect (which would needlessly rebind on every parent render).
    const onOpenLinkRef = useRef(onOpenLink)
    onOpenLinkRef.current = onOpenLink

    const doc = buildSrcDoc(html, imagesShown)

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
