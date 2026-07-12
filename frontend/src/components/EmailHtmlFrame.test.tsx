// EmailHtmlFrame renders message HTML inside a sandboxed, CSP-locked iframe. jsdom does not lay out or
// parse an iframe srcdoc, so these tests pin the security-relevant contract on the srcdoc string and the
// iframe attributes, plus the delegated link handling exercised against the frame's same-origin document.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, render} from '@testing-library/react'
import {EmailHtmlFrame} from './EmailHtmlFrame'

afterEach(() => cleanup())

interface FrameOverrides {
    html?: string
    imagesShown?: boolean
    onOpenLink?: (href: string) => void
}

function renderFrame(overrides: FrameOverrides = {}) {
    const onOpenLink = overrides.onOpenLink ?? vi.fn()
    const {container} = render(
        <EmailHtmlFrame
            html={overrides.html ?? '<p>hello</p>'}
            imagesShown={overrides.imagesShown ?? false}
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

    it('blocks remote images in the CSP until images are shown', () => {
        const {frame} = renderFrame({imagesShown: false})
        const srcdoc = frame.getAttribute('srcdoc') ?? ''
        expect(srcdoc).toContain('img-src data:;')
        expect(srcdoc).not.toContain('img-src data: https:')
    })

    it('widens the CSP img-src to remote images once shown', () => {
        const {frame} = renderFrame({imagesShown: true})
        const srcdoc = frame.getAttribute('srcdoc') ?? ''
        expect(srcdoc).toContain('img-src data: https: http:')
    })

    it('carries the sanitised body verbatim in the srcdoc', () => {
        const {frame} = renderFrame({html: '<p>Message body</p>'})
        expect(frame.getAttribute('srcdoc')).toContain('<p>Message body</p>')
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
