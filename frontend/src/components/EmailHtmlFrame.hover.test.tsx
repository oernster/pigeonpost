// EmailHtmlFrame's part in the tray's hover readout and in showing emoticons as emoji: what the frame
// translates in the written document; what it reports as the pointer moves over a link and leaves.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render} from '@testing-library/react'
import {EmailHtmlFrame} from './EmailHtmlFrame'
import {HoverChannelContext} from '../hooks/useHoverStatus'
import {createHoverChannel} from '../hoverStatus'

afterEach(() => cleanup())

function renderFrame(html: string) {
    const channel = createHoverChannel()
    const {container, unmount} = render(
        <HoverChannelContext.Provider value={channel}>
            <EmailHtmlFrame html={html} dark={false} onOpenLink={vi.fn()}/>
        </HoverChannelContext.Provider>,
    )
    const frame = container.querySelector('iframe') as HTMLIFrameElement
    return {frame, channel, unmount, doc: frame.contentDocument as Document}
}

describe('EmailHtmlFrame: emoji', () => {
    it('shows the message text with its emoticons and shortcodes as emoji', () => {
        const {doc} = renderFrame('<p>Thanks :) <a href="https://x.test/:)">see :tada:</a></p>')
        expect(doc.body.innerHTML).toBe('<p>Thanks 🙂 <a href="https://x.test/:)">see 🎉</a></p>')
    })
})

describe('EmailHtmlFrame: hover readout', () => {
    it('reports where a hovered link really goes', () => {
        const {doc, channel} = renderFrame('<p><a href="https://evil.test/login"><b>www.yourbank.com</b></a></p>')
        fireEvent.mouseMove(doc.querySelector('b') as Element)
        expect(channel.current()).toMatchObject({kind: 'link', host: 'evil.test', shown: 'www.yourbank.com'})
    })

    it('clears the reading when the pointer leaves the frame or the frame goes away', () => {
        const {frame, doc, channel, unmount} = renderFrame('<p><a href="https://example.com/">go</a></p>')
        fireEvent.mouseMove(doc.querySelector('a') as Element)
        expect(channel.current()).not.toBeNull()
        fireEvent.mouseLeave(frame)
        expect(channel.current()).toBeNull()
        fireEvent.mouseMove(doc.querySelector('a') as Element)
        unmount()
        expect(channel.current()).toBeNull()
    })

    it('reports nothing over plain text with no emoji under the pointer', () => {
        const {doc, channel} = renderFrame('<p id="t">plain</p>')
        fireEvent.mouseMove(doc.getElementById('t') as Element)
        expect(channel.current()).toBeNull()
    })
})
