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
            'img,picture,video,svg,canvas,[background],[style*="background-image"]{filter:invert(1) hue-rotate(180deg);}',
        )
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
