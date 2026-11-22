// Tests for the bottom tray at its outer interface: what it renders and what the donate button does.
// The URL is asserted literally on purpose, so a typo in the payment address fails here rather than
// sending a supporter to a page that is not Oliver's.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {act, cleanup, fireEvent, render, screen} from '@testing-library/react'
import {BottomBar} from './BottomBar'
import {HoverChannelContext} from '../hooks/useHoverStatus'
import {createHoverChannel, emojiStatus, linkStatus} from '../hoverStatus'
import {spiesNotInApi, unstubbedNames} from '../test/apiMock'

const apiSpies = vi.hoisted(() => ({openExternal: vi.fn()}))
const openExternal = apiSpies.openExternal

// The mock is built from the real api rather than hand-listed here, so a method reached with no spy
// fails the test by name instead of throwing a TypeError into the nearest catch and passing. The
// afterEach below reports any that were reached. See src/test/apiMock.ts.
const unstubbedCalls = vi.hoisted(() => new Set<string>())
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    const {buildApiStubs} = await import('../test/apiMock')
    return {...actual, api: buildApiStubs(actual, apiSpies as unknown as Record<string, unknown>, unstubbedCalls)}
})

afterEach(() => {
    cleanup()
    openExternal.mockClear()
    // Unmounting can reach the api too, so this is read after cleanup rather than before it.
    expect(unstubbedNames(unstubbedCalls), 'api methods reached with no stub: declare them in apiSpies')
        .toEqual([])
})

describe('BottomBar', () => {
    it('carries the donate button at the far left of the tray', () => {
        const {container} = render(<BottomBar/>)
        const tray = container.querySelector('footer.titlebar.bottombar')
        expect(tray).toBeTruthy()
        const buttons = tray?.querySelectorAll('button') ?? []
        expect(buttons.length).toBe(1)
        expect(buttons[0]).toBe(screen.getByLabelText('Donate to support PigeonPost'))
    })

    it('opens the payment page in the browser rather than the webview', () => {
        render(<BottomBar/>)
        fireEvent.click(screen.getByLabelText('Donate to support PigeonPost'))
        expect(openExternal).toHaveBeenCalledWith('https://www.paypal.com/ncp/payment/Q2U2RRG5WV7YJ')
    })
})

describe('BottomBar: hover readout', () => {
    function renderWithChannel() {
        const channel = createHoverChannel()
        const {container} = render(
            <HoverChannelContext.Provider value={channel}>
                <BottomBar/>
            </HoverChannelContext.Provider>,
        )
        const tray = container.querySelector('footer') as HTMLElement
        const readout = screen.getByRole('status')
        return {channel, tray, readout}
    }

    it('is empty and not raised while nothing is hovered', () => {
        const {tray, readout} = renderWithChannel()
        expect(readout).toBeEmptyDOMElement()
        expect(tray).not.toHaveClass('bottombar-raised')
    })

    it('shows a link destination with the host in bold and raises the tray over a dialog', () => {
        const {channel, tray, readout} = renderWithChannel()
        act(() => channel.report(linkStatus('https://example.com/path', 'Read more')))
        expect(readout).toHaveTextContent('https://example.com/path')
        expect(readout.querySelector('strong')).toHaveTextContent(/^example\.com$/)
        expect(tray).toHaveClass('bottombar-raised')
        act(() => channel.report(null))
        expect(readout).toBeEmptyDOMElement()
        expect(tray).not.toHaveClass('bottombar-raised')
    })

    it('warns when the link text names a different site', () => {
        const {channel, readout} = renderWithChannel()
        act(() => channel.report(linkStatus('https://evil.test/', 'yourbank.com')))
        expect(readout).toHaveTextContent('Link text shows yourbank.com but goes to https://evil.test/')
    })

    it('says when PigeonPost will not open a link', () => {
        const {channel, readout} = renderWithChannel()
        act(() => channel.report(linkStatus('javascript:alert(1)', 'Win')))
        expect(readout).toHaveTextContent('PigeonPost will not open this link: javascript:alert(1)')
    })

    it('names a hovered emoji', () => {
        const {channel, readout} = renderWithChannel()
        act(() => channel.report(emojiStatus('🎉')))
        expect(readout).toHaveTextContent('🎉 party popper')
    })
})

// The mock covers the api in both directions: the afterEach above catches a method reached with no
// spy; this catches the opposite, a spy declared under a name the api does not have, which binds to
// nothing, so every test configuring it would be configuring a stub the code can never call.
describe('the api mock', () => {
    it('declares no spy the real api does not have', async () => {
        const actual = await vi.importActual<typeof import('../api')>('../api')
        expect(spiesNotInApi(actual, apiSpies as unknown as Record<string, unknown>)).toEqual([])
    })
})
