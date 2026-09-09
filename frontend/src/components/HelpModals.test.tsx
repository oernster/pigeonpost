// The two help dialogs, About and Licence. Both hold content that outgrows the window, so both scroll only
// their body and pin their action row at the foot; both wear the self-reading cycle on that body. jsdom
// lays nothing out, so what is pinned cannot be measured here; what CAN be pinned by construction is that
// the action row is not inside the scroller and the scroller is the body, which is the invariant the
// layout rests on.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {AboutModal} from './AboutModal'
import {LicenceModal} from './LicenceModal'
import {GuideModal} from './GuideModal'
import {guideSections} from './guideContent'
import type {AboutInfo} from '../api'

const ABOUT: AboutInfo = {
    name: 'PigeonPost',
    tagline: 'A calm mail client',
    // Deliberately not a real version: the fixture pins the dialog's layout, not what VERSION happens to say.
    version: '0.0.0-test',
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

describe('GuideModal', () => {
    it('renders nothing while it is closed', () => {
        const {container} = render(<GuideModal open={false} onClose={vi.fn()}/>)
        expect(container.firstChild).toBeNull()
    })

    it('scrolls the body and keeps Close outside it', () => {
        const {container} = render(<GuideModal open={true} onClose={vi.fn()}/>)
        const close = container.querySelector('.modal-actions .btn')!
        expect(close.textContent).toBe('Close')
        expect(close.closest('.modal-body')).toBeNull()
        expect(container.querySelector('.modal.guide')!.classList.contains('pinned-actions')).toBe(true)
    })

    it('draws every section of the guide inside the scrolling body', () => {
        render(<GuideModal open={true} onClose={vi.fn()}/>)
        for (const section of guideSections) {
            expect(screen.getByText(section.heading).closest('.modal-body')).not.toBeNull()
        }
    })

    // The guide names the furniture by its REAL picture, so an entry without one would be the defect the
    // whole screen exists to avoid: every entry carries an image; it is the icon the entry declares.
    it('gives every named entry the icon it declares', () => {
        const {container} = render(<GuideModal open={true} onClose={vi.fn()}/>)
        const declared = guideSections.flatMap((section) => section.entries ?? [])
        const drawn = Array.from(container.querySelectorAll<HTMLImageElement>('.guide-entry .guide-icon'))
        expect(drawn.length).toBe(declared.length)
        drawn.forEach((img, i) => expect(img.getAttribute('src')).toBe(declared[i].icon))
    })

    it('closes from the footer button', () => {
        const onClose = vi.fn()
        const {container} = render(<GuideModal open={true} onClose={onClose}/>)
        fireEvent.click(container.querySelector('.modal-actions .btn')!)
        expect(onClose).toHaveBeenCalledTimes(1)
    })
})
