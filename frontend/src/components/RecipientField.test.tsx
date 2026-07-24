// RecipientField at its outer interface: an ordinary text input whose contact suggestions appear
// while an address fragment is typed, accept by keyboard or click as plain text insertion and never
// stop the user editing the field afterwards.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {useState} from 'react'
import {RecipientField} from './RecipientField'
import {Suggestion, suggestionPool} from '../recipientSuggest'

const pool: Suggestion[] = suggestionPool([
    {formattedName: 'Jane Doe', givenName: '', familyName: '', emails: [{address: 'jane@example.com'}]},
    {formattedName: 'Janet Frame', givenName: '', familyName: '', emails: [{address: 'janet@books.example'}]},
])

// Harness holds the value in state the way ComposeModal does, so accepted suggestions round-trip
// through onChange and the input re-renders with the new value.
function Harness({ensurePool = () => {}}: {ensurePool?: () => void}) {
    const [value, setValue] = useState('')
    return (
        <RecipientField label="To" value={value} placeholder="to..." pool={pool}
                        ensurePool={ensurePool} onChange={setValue}/>
    )
}

function input(): HTMLInputElement {
    return screen.getByPlaceholderText('to...') as HTMLInputElement
}

afterEach(() => cleanup())

describe('RecipientField', () => {
    it('shows matching suggestions once typing starts and none before', () => {
        render(<Harness/>)
        expect(screen.queryByRole('listbox')).toBeNull()
        fireEvent.change(input(), {target: {value: 'jan'}})
        const options = screen.getAllByRole('option')
        expect(options.map((o) => o.textContent)).toEqual([
            'Jane Doejane@example.com',
            'Janet Framejanet@books.example',
        ])
    })

    it('requests the pool on first focus and on typing', () => {
        const ensurePool = vi.fn()
        render(<Harness ensurePool={ensurePool}/>)
        fireEvent.focus(input())
        fireEvent.change(input(), {target: {value: 'j'}})
        expect(ensurePool).toHaveBeenCalled()
    })

    it('accepts the highlighted suggestion with Enter, inserting plain text', () => {
        render(<Harness/>)
        fireEvent.change(input(), {target: {value: 'jan'}})
        fireEvent.keyDown(input(), {key: 'ArrowDown'})
        fireEvent.keyDown(input(), {key: 'Enter'})
        expect(input().value).toBe('janet@books.example')
        expect(screen.queryByRole('listbox')).toBeNull()
    })

    it('accepts with Tab too', () => {
        render(<Harness/>)
        fireEvent.change(input(), {target: {value: 'jan'}})
        fireEvent.keyDown(input(), {key: 'Tab'})
        expect(input().value).toBe('jane@example.com')
    })

    it('accepts by click, keeping earlier addresses', () => {
        render(<Harness/>)
        fireEvent.change(input(), {target: {value: 'bob@quarry.example, jan'}})
        fireEvent.click(screen.getAllByRole('option')[0])
        expect(input().value).toBe('bob@quarry.example, jane@example.com')
    })

    it('wraps the highlight with the arrow keys', () => {
        render(<Harness/>)
        fireEvent.change(input(), {target: {value: 'jan'}})
        fireEvent.keyDown(input(), {key: 'ArrowUp'})
        fireEvent.keyDown(input(), {key: 'Enter'})
        expect(input().value).toBe('janet@books.example')
    })

    it('closes on Escape without touching the field and stops the event there', () => {
        render(<Harness/>)
        fireEvent.change(input(), {target: {value: 'jan'}})
        const documentEscape = vi.fn()
        document.addEventListener('keydown', documentEscape)
        fireEvent.keyDown(input(), {key: 'Escape'})
        document.removeEventListener('keydown', documentEscape)
        expect(screen.queryByRole('listbox')).toBeNull()
        expect(input().value).toBe('jan')
        expect(documentEscape).not.toHaveBeenCalled()
    })

    it('closes on blur and stays editable after an acceptance', () => {
        render(<Harness/>)
        fireEvent.change(input(), {target: {value: 'jan'}})
        fireEvent.keyDown(input(), {key: 'Enter'})
        fireEvent.change(input(), {target: {value: 'jane@other.example'}})
        expect(input().value).toBe('jane@other.example')
        fireEvent.change(input(), {target: {value: 'jan'}})
        expect(screen.getByRole('listbox')).toBeInTheDocument()
        fireEvent.blur(input())
        expect(screen.queryByRole('listbox')).toBeNull()
    })

    it('leaves modifier chords alone while open', () => {
        render(<Harness/>)
        fireEvent.change(input(), {target: {value: 'jan'}})
        fireEvent.keyDown(input(), {key: 'Enter', ctrlKey: true})
        expect(input().value).toBe('jan')
        expect(screen.getByRole('listbox')).toBeInTheDocument()
    })
})
