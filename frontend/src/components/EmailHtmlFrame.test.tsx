// EmailHtmlFrame renders message HTML inside a sandboxed, CSP-locked iframe. The parent writes the frame's
// document itself (srcdoc left the click listener bound to a dead document under WebKit), so these tests pin
// the security-relevant contract on the written document and the iframe attributes, plus the delegated link
// handling exercised against the frame's same-origin document.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, render} from '@testing-library/react'
import {EmailHtmlFrame} from './EmailHtmlFrame'
import {applyEmailColorTreatment} from './emailDarkMode'

afterEach(() => cleanup())

interface FrameOverrides {
    html?: string
    dark?: boolean
    onOpenLink?: (href: string) => void
}

function renderFrame(overrides: FrameOverrides = {}) {
    const onOpenLink = overrides.onOpenLink ?? vi.fn()
    const {container, rerender} = render(
        <EmailHtmlFrame
            html={overrides.html ?? '<p>hello</p>'}
            dark={overrides.dark ?? false}
            onOpenLink={onOpenLink}
        />,
    )
    const frame = container.querySelector('iframe') as HTMLIFrameElement
    return {frame, onOpenLink, rerender}
}

// frameHtml serialises the document the component wrote into the frame, which is where the CSP meta, the
// surface styles and the message body all live now that no srcdoc attribute is involved.
function frameHtml(frame: HTMLIFrameElement): string {
    return frame.contentDocument?.documentElement.outerHTML ?? ''
}

describe('EmailHtmlFrame: sandbox and CSP', () => {
    it('sandboxes the frame to same-origin and scripts only, with scripts denied by the CSP instead', () => {
        const {frame} = renderFrame()
        // allow-scripts is deliberate: WebKit refuses to dispatch listeners inside a scripts-disabled
        // browsing context, including ones the parent registered on the frame's document, so without it a
        // link click did nothing on macOS and Linux. Actual script execution stays impossible: the CSP
        // grants no script-src (pinned below) and the sanitiser strips scripts server-side. Everything
        // else stays blocked: no popups, no top navigation, no forms, no downloads.
        expect(frame.getAttribute('sandbox')).toBe('allow-same-origin allow-scripts')
        expect(frame.getAttribute('sandbox')).not.toContain('allow-popups')
        expect(frame.getAttribute('sandbox')).not.toContain('allow-top-navigation')
        expect(frame.getAttribute('sandbox')).not.toContain('allow-forms')
        expect(frame.getAttribute('sandbox')).not.toContain('allow-downloads')
    })

    it('writes the document into the frame rather than using the srcdoc attribute', () => {
        const {frame} = renderFrame({html: '<p>written</p>'})
        // Under WebKit (WKWebView on macOS, WebKitGTK on Linux) srcdoc replaces the frame's document without
        // reliably firing load, leaving the click listener on the dead initial document, so email buttons
        // navigated the frame inline. The parent now writes the document it binds to; the attribute must
        // stay absent so that failure class cannot return.
        expect(frame.hasAttribute('srcdoc')).toBe(false)
        expect(frame.contentDocument?.body.innerHTML).toContain('<p>written</p>')
    })

    it('locks the written document down with a script-free CSP', () => {
        const {frame} = renderFrame()
        const doc = frameHtml(frame)
        expect(doc).toContain("default-src 'none'")
        expect(doc).toContain("style-src 'unsafe-inline'")
        expect(doc).not.toContain('script-src')
    })

    it('restricts images to data: URIs only, since remote images are proxied and inlined server-side', () => {
        const {frame} = renderFrame()
        const doc = frameHtml(frame)
        expect(doc).toContain('img-src data:;')
        // The frame never permits a remote image: there is no widening to http/https as there used to be, so
        // it makes no remote request at all.
        expect(doc).not.toContain('img-src data: https:')
        expect(doc).not.toContain('http:')
    })

    it('carries the sanitised body verbatim in the written document', () => {
        const {frame} = renderFrame({html: '<p>Message body</p>'})
        expect(frameHtml(frame)).toContain('<p>Message body</p>')
    })

    it('rewrites the frame document when the message changes', () => {
        const {frame, rerender, onOpenLink} = renderFrame({html: '<p>first</p>'})
        rerender(<EmailHtmlFrame html={'<p>second</p>'} dark={false} onOpenLink={onOpenLink} />)
        const doc = frameHtml(frame)
        expect(doc).toContain('<p>second</p>')
        expect(doc).not.toContain('<p>first</p>')
    })
})

describe('EmailHtmlFrame: colour treatment', () => {
    // The treatment runs on the document the component wrote, so these read the resulting element styles
    // rather than a stylesheet string: the decision is per element and depends on each element's own
    // background, which only the written document carries.
    function find(frame: HTMLIFrameElement, id: string): HTMLElement {
        return frame.contentDocument?.getElementById(id) as HTMLElement
    }

    function root(frame: HTMLIFrameElement): HTMLElement {
        return frame.contentDocument?.documentElement as HTMLElement
    }

    it('keeps the faithful white surface with no inversion in light mode', () => {
        const {frame} = renderFrame({dark: false, html: '<div id="card" style="background-color:#ffffff">Hi</div>'})
        expect(frameHtml(frame)).toContain('background:#ffffff')
        expect(root(frame).style.filter).toBe('')
        expect(find(frame, 'card').style.filter).toBe('')
    })

    it('inverts the document at the root when the app theme is dark', () => {
        const {frame} = renderFrame({dark: true})
        // The root flip is what darkens the paper and every light-designed region sitting on it.
        expect(root(frame).style.filter).toContain('invert(1) hue-rotate(180deg)')
    })

    it('leaves a region the sender already designed dark rendering as authored', () => {
        const {frame} = renderFrame({
            dark: true,
            // Steam's wishlist mail in miniature: a dark panel with white text inside a light message.
            html: '<div id="panel" style="background-color:#212429;color:#ffffff">1 GAME ON SALE</div>',
        })
        // A second filter on the panel cancels the root one for that subtree, so the panel keeps the dark
        // background and white text its author chose instead of being flipped to a light panel with black
        // text, which is what a single document-wide invert produced.
        expect(find(frame, 'panel').style.filter).toContain('invert(1) hue-rotate(180deg)')
    })

    it('inverts a light region rather than flipping it twice', () => {
        const {frame} = renderFrame({
            dark: true,
            html: '<div id="card" style="background-color:#ffffff;color:#111111">Light card</div>',
        })
        // The root flip already renders this dark, so touching it again would undo the dark theme.
        expect(find(frame, 'card').style.filter).toBe('')
    })

    it('keeps a nested light region dark inside a region designed dark', () => {
        const {frame} = renderFrame({
            dark: true,
            html: '<div id="panel" style="background-color:#212429">' +
                '<div id="inner" style="background-color:#ffffff">white callout</div></div>',
        })
        // The panel restored the authored colours for its subtree, so the white callout inside it needs a
        // flip of its own to come back to dark. Parity, not depth, is what the walk tracks.
        expect(find(frame, 'panel').style.filter).toContain('invert(1)')
        expect(find(frame, 'inner').style.filter).toContain('invert(1)')
    })

    it('re-inverts media in an inverted region so photos and logos keep their true colours', () => {
        const {frame} = renderFrame({dark: true, html: '<img id="shot" src="data:image/gif;base64,R0lGOD">'})
        const shot = find(frame, 'shot')
        // The image sits on the inverted paper, so it needs its own flip to land back on true colour. The
        // mid-grey frame is !important because HTML email almost universally resets image borders inline.
        expect(shot.style.filter).toContain('invert(1) hue-rotate(180deg)')
        expect(shot.style.getPropertyValue('border')).toBe('2px solid rgb(128, 128, 128)')
        expect(shot.style.getPropertyPriority('border')).toBe('important')
    })

    it('leaves media alone where the region around it already renders as authored', () => {
        const {frame} = renderFrame({
            dark: true,
            html: '<div style="background-color:#212429"><img id="logo" src="data:image/gif;base64,R0lGOD"></div>',
        })
        const logo = find(frame, 'logo')
        // Two flips are already in force here, so the logo is showing its true colours; flipping it again
        // would turn it into its own negative. It needs no frame either, the author composed it against dark.
        expect(logo.style.filter).toBe('')
        expect(logo.style.getPropertyValue('border')).toBe('')
    })

    it('rescues text left invisible by a background image that did not render', () => {
        const {frame} = renderFrame({
            dark: true,
            // ClearScore's hero: white text over a remote photo, with a near-white colour as the fallback.
            // The photo never renders, so the heading is white on #EAF5F5, which the invert turned into
            // black on black.
            html: '<div style="background-color:#EAF5F5"><h1 id="hero" style="color:#ffffff">Your options</h1></div>',
        })
        expect(find(frame, 'hero').style.color).toBe('rgb(26, 26, 26)')
    })

    it('rescues that text in the light theme too, where it was equally invisible', () => {
        const {frame} = renderFrame({
            dark: false,
            html: '<div style="background-color:#EAF5F5"><h1 id="hero" style="color:#ffffff">Your options</h1></div>',
        })
        // White on #EAF5F5 is a contrast ratio of 1.1 whichever theme is showing; the invert only made the
        // failure conspicuous.
        expect(find(frame, 'hero').style.color).toBe('rgb(26, 26, 26)')
    })

    it('renders a region with a painting background image as its author composed it', () => {
        const {frame} = renderFrame({
            dark: true,
            html: '<div id="hero-box" style="background-color:#EAF5F5;background-image:url(data:image/gif;base64,R0lGOD)">' +
                '<h1>Your options</h1></div>',
        })
        // Once the photo loads, the sender's picture and the white text over it are one composition. Leaving
        // the region inverted would render the photograph as its negative, so it is flipped back to authored
        // exactly as leaf media is.
        expect(find(frame, 'hero-box').style.filter).toContain('invert(1) hue-rotate(180deg)')
    })

    it('leaves text alone where a background image is actually painting', () => {
        const {frame} = renderFrame({
            dark: true,
            html: '<div style="background-color:#EAF5F5;background-image:url(data:image/gif;base64,R0lGOD)">' +
                '<h1 id="hero" style="color:#ffffff">Your options</h1></div>',
        })
        // With the image loaded the colour underneath is not what the reader sees, so the sender's white is
        // the right choice and must not be overridden. The repair marks what it sets !important, so an
        // untouched element is one whose colour carries no priority.
        expect(find(frame, 'hero').style.getPropertyPriority('color')).toBe('')
        expect(find(frame, 'hero').style.color).toBe('rgb(255, 255, 255)')
    })

    it('leaves readable text alone rather than flattening the sender palette', () => {
        const {frame} = renderFrame({
            dark: true,
            html: '<div style="background-color:#ffffff"><p id="body" style="color:#3999ec">Special promotion</p></div>',
        })
        expect(find(frame, 'body').style.getPropertyPriority('color')).toBe('')
        expect(find(frame, 'body').style.color).toBe('rgb(57, 153, 236)')
    })

    it('pins the light colour scheme on the iframe element in both themes', () => {
        // This is the pin that actually governs prefers-color-scheme inside the message: an embedded document
        // takes its colour-scheme preference from its embedder, and declaring it inside the frame's own
        // document does not change it (measured in Chromium/WebView2). Without it the frame inherits the app's
        // dark scheme and a partial-dark email switches its own rules on, which is the blinding case.
        for (const dark of [false, true]) {
            expect(renderFrame({dark}).frame.style.colorScheme).toBe('light')
        }
    })

    it('also declares the light scheme inside the frame document', () => {
        // Not what drives prefers-color-scheme (the element pin above does that), but it sets the used scheme
        // for UA-rendered controls inside the message.
        for (const dark of [false, true]) {
            const doc = frameHtml(renderFrame({dark}).frame)
            expect(doc).toContain('name="color-scheme" content="light"')
            expect(doc).toContain(':root{color-scheme:light;}')
        }
    })

    it('treats an email that ships partial dark-mode styling like any other', () => {
        const {frame} = renderFrame({
            dark: true,
            html: '<style>@media (prefers-color-scheme:dark){body{background:#181a1a;color:#fff}}</style>' +
                '<div id="card" style="background-color:#ffffff">Hi</div>',
        })
        // Dark-mode support in HTML email is almost always partial: the media query recolours a few elements
        // while bgcolor attributes and inline styles keep the bulk of the page white. The frame pins the
        // message to light so those rules never fire, and colours it from its actual backgrounds instead.
        expect(root(frame).style.filter).toContain('invert(1)')
        expect(find(frame, 'card').style.filter).toBe('')
        expect(frameHtml(frame)).toContain('name="color-scheme" content="light"')
    })

    it('does not treat the same document twice', () => {
        const {frame} = renderFrame({dark: true, html: '<p>plain light email</p>'})
        const doc = frame.contentDocument as Document
        const before = root(frame).style.filter
        // The component attaches once after writing and again on any load event. A second pass would flip
        // every region back and undo the theme, so the treated document is marked.
        applyEmailColorTreatment(doc, true)
        expect(root(frame).style.filter).toBe(before)
    })
})

describe('EmailHtmlFrame: link interception', () => {
    // The component binds its delegated click listener to the same document it wrote, so a link injected into
    // that document and clicked with a bubbling event exercises exactly the path a real click inside the
    // rendered email takes.
    function clickInjectedLink(frame: HTMLIFrameElement, href: string): Event {
        const cdoc = frame.contentDocument as Document
        const anchor = cdoc.createElement('a')
        anchor.setAttribute('href', href)
        anchor.textContent = 'link'
        cdoc.body.appendChild(anchor)
        const event = new Event('click', {bubbles: true, cancelable: true})
        anchor.dispatchEvent(event)
        return event
    }

    it('opens an http(s) link through onOpenLink', () => {
        const {frame, onOpenLink} = renderFrame()
        clickInjectedLink(frame, 'https://example.com/x')
        expect(onOpenLink).toHaveBeenCalledWith('https://example.com/x')
    })

    it('opens a mailto link through onOpenLink', () => {
        const {frame, onOpenLink} = renderFrame()
        clickInjectedLink(frame, 'mailto:person@example.com')
        expect(onOpenLink).toHaveBeenCalledWith('mailto:person@example.com')
    })

    it('ignores a link whose scheme is not http, https or mailto but still blocks frame navigation', () => {
        const {frame, onOpenLink} = renderFrame()
        const event = clickInjectedLink(frame, 'tel:+1234567890')
        expect(onOpenLink).not.toHaveBeenCalled()
        // The click is still cancelled so the sandboxed frame never navigates to the link.
        expect(event.defaultPrevented).toBe(true)
    })

    it('keeps intercepting clicks in the document written for a changed message', () => {
        const {frame, rerender, onOpenLink} = renderFrame({html: '<p>first</p>'})
        rerender(<EmailHtmlFrame html={'<p>second</p>'} dark={false} onOpenLink={onOpenLink} />)
        // The rewrite resets the document, so the listener must have been rebound to the live document or a
        // click in the new message would fall through (the WebKit dead-listener bug, engine-independent pin).
        clickInjectedLink(frame, 'https://example.com/rebound')
        expect(onOpenLink).toHaveBeenCalledWith('https://example.com/rebound')
    })
})
