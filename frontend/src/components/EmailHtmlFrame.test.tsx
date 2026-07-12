// EmailHtmlFrame renders message HTML inside a sandboxed, CSP-locked iframe. jsdom does not lay out or
// parse an iframe srcdoc, so these tests pin the security-relevant contract on the srcdoc string and the
// iframe attributes, plus the delegated link handling exercised against the frame's same-origin document.
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
    const {container} = render(
        <EmailHtmlFrame
            html={overrides.html ?? '<p>hello</p>'}
            dark={overrides.dark ?? false}
            onOpenLink={onOpenLink}
        />,
    )
    const frame = container.querySelector('iframe') as HTMLIFrameElement
    return {frame, onOpenLink}
}

describe('EmailHtmlFrame: sandbox and CSP', () => {
    it('sandboxes the frame to allow-same-origin only, never allow-scripts', () => {
        const {frame} = renderFrame()
        expect(frame.getAttribute('sandbox')).toBe('allow-same-origin')
        expect(frame.getAttribute('sandbox')).not.toContain('allow-scripts')
    })

    it('locks the srcdoc down with a script-free CSP', () => {
        const {frame} = renderFrame()
        const srcdoc = frame.getAttribute('srcdoc') ?? ''
        expect(srcdoc).toContain("default-src 'none'")
        expect(srcdoc).toContain("style-src 'unsafe-inline'")
        expect(srcdoc).not.toContain('script-src')
    })

    it('restricts images to data: URIs only, since remote images are proxied and inlined server-side', () => {
        const {frame} = renderFrame()
        const srcdoc = frame.getAttribute('srcdoc') ?? ''
        expect(srcdoc).toContain('img-src data:;')
        // The frame never permits a remote image: there is no widening to http/https as there used to be, so
        // it makes no remote request at all.
        expect(srcdoc).not.toContain('img-src data: https:')
        expect(srcdoc).not.toContain('http:')
    })

    it('carries the sanitised body verbatim in the srcdoc', () => {
        const {frame} = renderFrame({html: '<p>Message body</p>'})
        expect(frame.getAttribute('srcdoc')).toContain('<p>Message body</p>')
    })
})

describe('EmailHtmlFrame: dark mode', () => {
    it('keeps the faithful white surface with no inversion in light mode', () => {
        const {frame} = renderFrame({dark: false})
        const srcdoc = frame.getAttribute('srcdoc') ?? ''
        expect(srcdoc).toContain('background:#ffffff')
        expect(srcdoc).not.toContain('invert(1)')
    })

    it('inverts the whole document to render dark when the app theme is dark', () => {
        const {frame} = renderFrame({dark: true})
        const srcdoc = frame.getAttribute('srcdoc') ?? ''
        // The light-designed document is inverted so a white background becomes dark and dark text light.
        expect(srcdoc).toContain('html{filter:invert(1) hue-rotate(180deg);}')
    })

    it('re-inverts images so photos and logos keep their true colours in dark mode', () => {
        const {frame} = renderFrame({dark: true})
        const srcdoc = frame.getAttribute('srcdoc') ?? ''
        // The same filter on media double-inverts it back to its original colours; a plain background-colour
        // is deliberately not matched, so a coloured box keeps its inverted dark fill.
        expect(srcdoc).toContain(
            'img,picture,video,svg,canvas,[background]:empty,[style*="background-image"]:empty{filter:invert(1) hue-rotate(180deg);border:2px solid #808080 !important;box-sizing:border-box;}',
        )
    })

    it('frames re-inverted media with an !important border so it survives an email border reset', () => {
        const {frame} = renderFrame({dark: true})
        const srcdoc = frame.getAttribute('srcdoc') ?? ''
        // A mid-grey border rides the re-inverted media so a dark cover cannot read as a dark block on a dark
        // cell (the Amazon book-cover contrast bug). It is !important because HTML email almost universally
        // sets an inline border:0 on images, which would otherwise beat this rule and drop the frame.
        expect(srcdoc).toContain('border:2px solid #808080 !important')
        expect(srcdoc).toContain('box-sizing:border-box')
    })

    it('does not re-invert a content-bearing background container, which would flip its subtree back to light', () => {
        const {frame} = renderFrame({dark: true})
        const srcdoc = frame.getAttribute('srcdoc') ?? ''
        // A filter on a container and one on its descendants compound, so re-inverting a layout cell that
        // carries a background attribute or image would double-invert its whole subtree back to light (the
        // Amazon order-email bug) and flip its child images the wrong way. Only :empty background boxes, which
        // hold no content, are matched, so the bare blanket forms must be absent.
        expect(srcdoc).toContain('[background]:empty')
        expect(srcdoc).toContain('[style*="background-image"]:empty')
        expect(srcdoc).not.toContain('[background],')
        expect(srcdoc).not.toContain('[style*="background-image"]{')
    })

    it('renders a dark-mode-aware email natively, without inverting it', () => {
        const {frame} = renderFrame({
            dark: true,
            html: '<style>@media (prefers-color-scheme:dark){body{background:#181a1a;color:#fff}}</style><p>Hi</p>',
        })
        const srcdoc = frame.getAttribute('srcdoc') ?? ''
        // The message darkens itself, so the frame must not invert it (that would flip it back to light); it
        // renders on a dark paper and the message's own prefers-color-scheme:dark rules apply.
        expect(srcdoc).not.toContain('invert(1)')
        expect(srcdoc).toContain('background:#1a1a1a;color:#e6e6e6')
    })

    it('still inverts a light-only email that has no dark-mode styling', () => {
        const {frame} = renderFrame({dark: true, html: '<p>plain light email</p>'})
        const srcdoc = frame.getAttribute('srcdoc') ?? ''
        expect(srcdoc).toContain('html{filter:invert(1) hue-rotate(180deg);}')
    })
})

describe('EmailHtmlFrame: link interception', () => {
    // jsdom does not parse the srcdoc, but it exposes a real (empty) same-origin contentDocument, so the
    // delegated click listener the component binds there can be exercised by injecting a link and
    // dispatching a bubbling click, exactly as a real click inside the rendered email would.
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
})
