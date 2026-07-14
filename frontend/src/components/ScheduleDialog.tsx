import {useState} from 'react'
import {ModalClose} from './ModalClose'
import {useBackdropDismiss} from './useBackdropDismiss'
import {fromDatetimeLocal, isSchedulable} from '../schedule'

interface ScheduleDialogProps {
    title: string
    label: string
    confirmLabel: string
    onSubmit: (at: Date) => void
    onCancel: () => void
}

// ScheduleDialog is the shared pick-a-moment modal (the snooze custom time). It submits only a moment
// that is strictly in the future, on Enter or the confirm button.
export function ScheduleDialog({title, label, confirmLabel, onSubmit, onCancel}: ScheduleDialogProps) {
    const dismiss = useBackdropDismiss(onCancel)
    const [value, setValue] = useState('')
    const chosen = fromDatetimeLocal(value)
    const valid = isSchedulable(chosen, new Date())
    const submit = () => {
        if (chosen !== null && valid) {
            onSubmit(chosen)
        }
    }
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal" role="dialog" aria-label={title} onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onCancel}/>
                <h2 className="modal-title">{title}</h2>
                <label className="field">
                    <span>{label}</span>
                    <input
                        type="datetime-local"
                        value={value}
                        autoFocus
                        onChange={(e) => setValue(e.target.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                                e.preventDefault()
                                submit()
                            }
                        }}
                    />
                </label>
                <div className="modal-actions spread">
                    <button className="btn" onClick={onCancel}>Cancel</button>
                    <button className="btn primary" onClick={submit} disabled={!valid}>
                        {confirmLabel}
                    </button>
                </div>
            </div>
        </div>
    )
}
