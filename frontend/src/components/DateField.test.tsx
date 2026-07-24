// DateField at its outer interface: the native input stays typeable, the calendar button opens the
// themed picker dialog and picking a day merges only the date into the value (a datetime field
// keeps its typed time).
import {afterEach, describe, expect, it} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {useState} from 'react'
import {DateField} from './DateField'

function Harness({kind, initial = '', min}: {kind: 'date' | 'datetime-local'; initial?: string; min?: string}) {
    const [value, setValue] = useState(initial)
    return (
        <DateField kind={kind} ariaLabel="When" pickerTitle="When" min={min}
                   value={value} onChange={setValue}/>
    )
}

function input(): HTMLInputElement {
    return screen.getByLabelText('When') as HTMLInputElement
}

function openPicker() {
    fireEvent.click(screen.getByRole('button', {name: 'Open the when calendar'}))
}

afterEach(() => cleanup())

describe('DateField', () => {
    it('stays a typeable input', () => {
        render(<Harness kind="date"/>)
        fireEvent.change(input(), {target: {value: '2026-07-24'}})
        expect(input().value).toBe('2026-07-24')
        expect(screen.queryByRole('dialog')).toBeNull()
    })

    it('opens the picker on the field\'s month and picks a day into a date field', () => {
        render(<Harness kind="date" initial="2026-07-01"/>)
        openPicker()
        expect(screen.getByRole('dialog', {name: 'When'})).toBeInTheDocument()
        expect(screen.getByText('July 2026')).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', {name: '2026-07-24'}))
        expect(input().value).toBe('2026-07-24')
        expect(screen.queryByRole('dialog')).toBeNull()
    })

    it('keeps the typed time when picking into a datetime field', () => {
        render(<Harness kind="datetime-local" initial="2026-07-01T17:45"/>)
        openPicker()
        fireEvent.click(screen.getByRole('button', {name: '2026-07-24'}))
        expect(input().value).toBe('2026-07-24T17:45')
    })

    it('defaults the time when none was typed', () => {
        render(<Harness kind="datetime-local" initial="2026-07-01T"/>)
        openPicker()
        fireEvent.click(screen.getByRole('button', {name: '2026-07-24'}))
        expect(input().value).toBe('2026-07-24T09:00')
    })

    it('navigates months and disables days before the minimum', () => {
        render(<Harness kind="date" initial="2026-07-15" min="2026-07-10"/>)
        openPicker()
        expect(screen.getByRole('button', {name: '2026-07-09'})).toBeDisabled()
        expect(screen.getByRole('button', {name: '2026-07-10'})).toBeEnabled()
        fireEvent.click(screen.getByRole('button', {name: 'Next month'}))
        expect(screen.getByText('August 2026')).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', {name: 'Previous month'}))
        fireEvent.click(screen.getByRole('button', {name: 'Previous month'}))
        expect(screen.getByText('June 2026')).toBeInTheDocument()
    })

    it('cancel closes without changing the value', () => {
        render(<Harness kind="date" initial="2026-07-15"/>)
        openPicker()
        fireEvent.click(screen.getByRole('button', {name: 'Cancel'}))
        expect(screen.queryByRole('dialog')).toBeNull()
        expect(input().value).toBe('2026-07-15')
    })

    it('Today picks the current day', () => {
        render(<Harness kind="date"/>)
        openPicker()
        fireEvent.click(screen.getByRole('button', {name: 'Today'}))
        const now = new Date()
        const iso = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
        expect(input().value).toBe(iso)
    })
})
