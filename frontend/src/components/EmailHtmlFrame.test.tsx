// EmailHtmlFrame renders message HTML inside a sandboxed, CSP-locked iframe. The parent writes the frame's
// document itself (srcdoc left the click listener bound to a dead document under WebKit), so these tests pin
// the security-relevant contract on the written document and the iframe attributes, plus the delegated link
// handling exercised against the frame's same-origin document.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, render} from '@testing-library/react'
import {EmailHtmlFrame} from './EmailHtmlFrame'

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

describe('EmailHtmlFrame: dark mode', () => {
    it('keeps the faithful white surface with no inversion in light mode', () => {
        const {frame} = renderFrame({dark: false})
        const doc = frameHtml(frame)
        expect(doc).toContain('background:#ffffff')
        expect(doc).not.toContain('invert(1)')
    })

    it('inverts the whole document to render dark when the app theme is dark', () => {
        const {frame} = renderFrame({dark: true})
        // The light-designed document is inverted so a white background becomes dark and dark text light.
        expect(frameHtml(frame)).toContain('html{filter:invert(1) hue-rotate(180deg);}')
    })

    it('re-inverts images so photos and logos keep their true colours in dark mode', () => {
        const {frame} = renderFrame({dark: true})
        // The same filter on media double-inverts it back to its original colours; a plain background-colour
        // is deliberately not matched, so a coloured box keeps its inverted dark fill.
        expect(frameHtml(frame)).toContain(
            'img,picture,video,svg,canvas,[background]:empty,[style*="background-image"]:empty{filter:invert(1) hue-rotate(180deg);border:2px solid #808080 !important;box-sizing:border-box;}',
        )
    })

    it('frames re-inverted media with an !important border so it survives an email border reset', () => {
        const {frame} = renderFrame({dark: true})
        const doc = frameHtml(frame)
        // A mid-grey border rides the re-inverted media so a dark cover cannot read as a dark block on a dark
        // cell (the Amazon book-cover contrast bug). It is !important because HTML email almost universally
        // sets an inline border:0 on images, which would otherwise beat this rule and drop the frame.
        expect(doc).toContain('border:2px solid #808080 !important')
        expect(doc).toContain('box-sizing:border-box')
    })

    it('does not re-invert a content-bearing background container, which would flip its subtree back to light', () => {
        const {frame} = renderFrame({dark: true})
        const doc = frameHtml(frame)
        // A filter on a container and one on its descendants compound, so re-inverting a layout cell that
        // carries a background attribute or image would double-invert its whole subtree back to light (the
        // Amazon order-email bug) and flip its child images the wrong way. Only :empty background boxes, which
        // hold no content, are matched, so the bare blanket forms must be absent.
        expect(doc).toContain('[background]:empty')
        expect(doc).toContain('[style*="background-image"]:empty')
        expect(doc).not.toContain('[background],')
        expect(doc).not.toContain('[style*="background-image"]{')
    })

    it('renders a dark-mode-aware email natively, without inverting it', () => {
        const {frame} = renderFrame({
            dark: true,
            html: '<style>@media (prefers-color-scheme:dark){body{background:#181a1a;color:#fff}}</style><p>Hi</p>',
        })
        const doc = frameHtml(frame)
        // The message darkens itself, so the frame must not invert it (that would flip it back to light); it
        // renders on a dark paper and the message's own prefers-color-scheme:dark rules apply.
        expect(doc).not.toContain('invert(1)')
        expect(doc).toContain('background:#1a1a1a;color:#e6e6e6')
    })

    it('still inverts a light-only email that has no dark-mode styling', () => {
        const {frame} = renderFrame({dark: true, html: '<p>plain light email</p>'})
        expect(frameHtml(frame)).toContain('html{filter:invert(1) hue-rotate(180deg);}')
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
