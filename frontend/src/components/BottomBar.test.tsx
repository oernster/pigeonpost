// Tests for the bottom tray at its outer interface: what it renders and what the donate button does.
// The URL is asserted literally on purpose, so a typo in the payment address fails here rather than
// sending a supporter to a page that is not Oliver's.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {BottomBar} from './BottomBar'

const openExternal = vi.fn()

vi.mock('../api', () => ({
    api: {
        openExternal: (url: string) => openExternal(url),
    },
}))

afterEach(() => {
    cleanup()
    openExternal.mockClear()
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
        expect(openExternal).toHaveBeenCalledWith('https://www.paypal.com/ncp/payment/6QEJKCEQ3ZFZ8')
    })
})
