import {useState} from 'react'
import {ModalClose} from './ModalClose'
import {useBackdropDismiss} from './useBackdropDismiss'
import {
    addMonths,
    beforeMin,
    isoDate,
    monthGrid,
    monthLabel,
    monthOf,
    WEEKDAY_LABELS,
} from '../datePicker'

interface DatePickerDialogProps {
    title: string
    // value is the field's current ISO date ('' when unset): it selects its day and opens its month.
    value: string
    // min disables the days before it ('' means none), mirroring the field's own minimum.
    min: string
    onPick: (iso: string) => void
    onClose: () => void
}

// DatePickerDialog is the app's calendar picker: a themed month grid stacked over whatever dialog
// opened it (the Escape stack closes one layer at a time). Picking a day reports its ISO date and
// closes; the month navigates with the header arrows and Today jumps back to the current month.
export function DatePickerDialog({title, value, min, onPick, onClose}: DatePickerDialogProps) {
    const dismiss = useBackdropDismiss(onClose)
    const now = new Date()
    const todayISO = isoDate(now.getFullYear(), now.getMonth() + 1, now.getDate())
    const [page, setPage] = useState(() => monthOf(value, monthOf(todayISO, {year: now.getFullYear(), month: now.getMonth() + 1})))

    const pick = (iso: string) => {
        onPick(iso)
        onClose()
    }

    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal datepick" role="dialog" aria-label={title} onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">{title}</h2>
                <div className="datepick-head">
                    <button type="button" className="btn" aria-label="Previous month"
                            onClick={() => setPage((p) => addMonths(p, -1))}>&#8249;</button>
                    <span className="datepick-label" aria-live="polite">{monthLabel(page)}</span>
                    <button type="button" className="btn" aria-label="Next month"
                            onClick={() => setPage((p) => addMonths(p, 1))}>&#8250;</button>
                </div>
                <div className="datepick-grid" role="grid" aria-label={monthLabel(page)}>
                    {WEEKDAY_LABELS.map((label) => (
                        <span key={label} className="datepick-weekday" aria-hidden="true">{label}</span>
                    ))}
                    {monthGrid(page).flat().map((cell) => (
                        <button
                            key={cell.iso}
                            type="button"
                            className={'datepick-day' +
                                (cell.inMonth ? '' : ' out') +
                                (cell.iso === value ? ' selected' : '') +
                                (cell.iso === todayISO ? ' today' : '')}
                            disabled={beforeMin(cell.iso, min)}
                            aria-label={cell.iso}
                            onClick={() => pick(cell.iso)}
                        >
                            {cell.day}
                        </button>
                    ))}
                </div>
                <div className="modal-actions spread">
                    <button type="button" className="btn" onClick={onClose}>Cancel</button>
                    <button type="button" className="btn" disabled={beforeMin(todayISO, min)}
                            onClick={() => pick(todayISO)}>
                        Today
                    </button>
                </div>
            </div>
        </div>
    )
}
