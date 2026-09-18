// Tests for the recurrence editor's interval box: it is always on screen so an "every 2 weeks" rule is
// discoverable. It is disabled while the event does not repeat; its value is carried into the rule as INTERVAL.
import {afterEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {RecurrenceEditor} from './RecurrenceEditor'

const START = '2026-09-18T09:00'

afterEach(cleanup)

describe('RecurrenceEditor interval', () => {
    it('shows the interval box disabled when the event does not repeat', () => {
        render(<RecurrenceEditor value="" onChange={vi.fn()} startDate={START}/>)
        expect(screen.getByText('Repeat every')).toBeTruthy()
        expect((screen.getByLabelText('Interval') as HTMLInputElement).disabled).toBe(true)
    })

    it('enables the interval box once a unit is chosen', () => {
        render(<RecurrenceEditor value="FREQ=WEEKLY" onChange={vi.fn()} startDate={START}/>)
        expect((screen.getByLabelText('Interval') as HTMLInputElement).disabled).toBe(false)
    })

    it('builds a two-weekly rule from the unit and the interval', () => {
        const onChange = vi.fn()
        render(<RecurrenceEditor value="" onChange={onChange} startDate={START}/>)
        fireEvent.change(screen.getByLabelText('Repeat'), {target: {value: 'WEEKLY'}})
        fireEvent.change(screen.getByLabelText('Interval'), {target: {value: '2'}})
        expect(onChange).toHaveBeenLastCalledWith('FREQ=WEEKLY;INTERVAL=2')
        expect(screen.getByRole('option', {name: 'weeks'})).toBeTruthy()
    })
})
