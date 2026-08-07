// The two help dialogs, About and Licence. Both hold content that outgrows the window, so both scroll only
// their body and pin their action row at the foot, and both wear the self-reading cycle on that body. jsdom
// lays nothing out, so what is pinned cannot be measured here; what CAN be pinned by construction is that
// the action row is not inside the scroller and the scroller is the body, which is the invariant the
// layout rests on.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, render, screen} from '@testing-library/react'
import {AboutModal} from './AboutModal'
import {LicenceModal} from './LicenceModal'
import type {AboutInfo} from '../api'

const ABOUT: AboutInfo = {
    name: 'PigeonPost',
    tagline: 'A calm mail client',
    version: '1.10.0',
    author: 'Oliver Ernster',
    licence: 'GPL-3.0',
    copyright: '(c) Oliver Ernster',
    attribution: 'Attribution term',
    credits: [{name: 'Go', licence: 'BSD-3-Clause'}, {name: 'React', licence: 'MIT'}],
} as AboutInfo

afterEach(() => cleanup())

describe('AboutModal', () => {
    it('renders nothing until there is something to show', () => {
        const {container} = render(<AboutModal about={null} onClose={vi.fn()}/>)
        expect(container.firstChild).toBeNull()
    })

    it('scrolls the body and keeps Close outside it', () => {
        const {container} = render(<AboutModal about={ABOUT} onClose={vi.fn()}/>)
        const close = container.querySelector('.modal-actions .btn')!
        expect(close.textContent).toBe('Close')
        expect(close.closest('.modal-body')).toBeNull()
        // The dialog delegates its scrolling to the body, which is what pins the row below it.
        expect(container.querySelector('.modal.about')!.classList.contains('pinned-actions')).toBe(true)
    })

    it('puts the credits inside the scrolling body', () => {
        render(<AboutModal about={ABOUT} onClose={vi.fn()}/>)
        expect(screen.getByText('React').closest('.modal-body')).not.toBeNull()
    })
})

describe('LicenceModal', () => {
    it('renders nothing until there is a licence to show', () => {
        const {container} = render(<LicenceModal text={null} onClose={vi.fn()}/>)
        expect(container.firstChild).toBeNull()
    })

    it('scrolls the licence text and keeps Close outside it', () => {
        const {container} = render(<LicenceModal text={'GPL-3.0\nterms'} onClose={vi.fn()}/>)
        const close = container.querySelector('.modal-actions .btn')!
        expect(close.textContent).toBe('Close')
        expect(close.closest('.licence-text')).toBeNull()
        expect(container.querySelector('.modal.licence')!.classList.contains('pinned-actions')).toBe(true)
    })

    it('shows an empty licence rather than nothing, so a missing file is visible', () => {
        const {container} = render(<LicenceModal text={''} onClose={vi.fn()}/>)
        expect(container.querySelector('.licence-text')).not.toBeNull()
    })
})
