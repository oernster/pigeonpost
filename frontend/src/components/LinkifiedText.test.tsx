import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {LinkifiedText} from './LinkifiedText'

afterEach(() => cleanup())

describe('LinkifiedText', () => {
    it('turns a bare https URL into a link', () => {
        render(<LinkifiedText text="see https://example.org/page now" onOpenLink={() => {}} />)
        const link = screen.getByRole('link', {name: 'https://example.org/page'})
        expect(link.getAttribute('href')).toBe('https://example.org/page')
    })

    it('keeps trailing sentence punctuation outside the link', () => {
        render(<LinkifiedText text="read https://example.org/a, ok?" onOpenLink={() => {}} />)
        const link = screen.getByRole('link')
        expect(link.textContent).toBe('https://example.org/a')
        expect(link.getAttribute('href')).toBe('https://example.org/a')
    })

    it('adds an http scheme for www hosts while keeping the bare text', () => {
        render(<LinkifiedText text="visit www.example.org today" onOpenLink={() => {}} />)
        const link = screen.getByRole('link', {name: 'www.example.org'})
        expect(link.getAttribute('href')).toBe('http://www.example.org')
    })

    it('links mailto addresses', () => {
        render(<LinkifiedText text="write mailto:jane@example.org soon" onOpenLink={() => {}} />)
        const link = screen.getByRole('link', {name: 'mailto:jane@example.org'})
        expect(link.getAttribute('href')).toBe('mailto:jane@example.org')
    })

    it('hands clicks to onOpenLink instead of navigating', () => {
        const onOpenLink = vi.fn()
        render(<LinkifiedText text="go https://example.org/x" onOpenLink={onOpenLink} />)
        fireEvent.click(screen.getByRole('link'))
        expect(onOpenLink).toHaveBeenCalledWith('https://example.org/x')
    })

    it('renders several links with the surrounding text intact', () => {
        const {container} = render(
            <LinkifiedText
                text="first https://a.example then https://b.example done"
                onOpenLink={() => {}}
            />,
        )
        expect(screen.getAllByRole('link')).toHaveLength(2)
        expect(container.textContent).toBe('first https://a.example then https://b.example done')
    })

    it('renders text without URLs unchanged', () => {
        const {container} = render(<LinkifiedText text="no links here" onOpenLink={() => {}} />)
        expect(container.querySelector('a')).toBeNull()
        expect(container.textContent).toBe('no links here')
    })
})
