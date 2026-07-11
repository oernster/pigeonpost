import type {Dispatch, SetStateAction} from 'react'
import {Calendar} from '../api'
import {DEFAULT_EVENT_COLOUR} from '../calendarModel'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'
import type {CalForm} from '../hooks/useCalendars'

interface CalendarsManagerProps {
    calendars: Calendar[]
    calForm: CalForm | null
    setCalForm: Dispatch<SetStateAction<CalForm | null>>
    pendingCalDelete: Calendar | null
    setPendingCalDelete: Dispatch<SetStateAction<Calendar | null>>
    saveCal: () => void
    confirmCalDelete: () => void
    // onClose closes the manager and clears the edit form.
    onClose: () => void
    busy: boolean
}

// CalendarsManager is the calendars sub-feature's modal: a chip bar of the existing calendars plus a New
// calendar chip, an inline colour-and-name editor for the selected calendar and the delete confirmation. It
// is the presentational surface over useCalendars; all its state and actions are injected.
export function CalendarsManager({
    calendars, calForm, setCalForm, pendingCalDelete, setPendingCalDelete, saveCal, confirmCalDelete, onClose, busy,
}: CalendarsManagerProps) {
    return (
        <>
            <div className="modal-backdrop">
                <div className="modal event-form" role="dialog" aria-label="Calendars"
                     onClick={(e) => e.stopPropagation()}>
                    <ModalClose onClose={onClose}/>
                    <h2 className="modal-title">Calendars</h2>
                    <p className="setup-hint">Calendars group your events and colour them across every view.</p>
                    <div className="cg-bar">
                        {calendars.map((c) => (
                            <button key={c.id} className="cg-chip"
                                    onClick={() => setCalForm({id: c.id, name: c.name, colour: c.colour || DEFAULT_EVENT_COLOUR})}>
                                <span className="cal-swatch" style={{background: c.colour || DEFAULT_EVENT_COLOUR}}/>
                                {c.name}
                            </button>
                        ))}
                        <button className="cg-chip"
                                onClick={() => setCalForm({id: '', name: '', colour: DEFAULT_EVENT_COLOUR})}>
                            + New calendar
                        </button>
                    </div>
                    {calForm && (
                        <div className="rule-form">
                            <div className="rule-form-row">
                                <input type="color" className="cal-colour" aria-label="Calendar colour"
                                       value={calForm.colour}
                                       onChange={(e) => setCalForm((cf) => cf ? {...cf, colour: e.target.value} : cf)}/>
                                <input className="tag-name-input" placeholder="Calendar name" value={calForm.name}
                                       autoFocus
                                       onChange={(e) => setCalForm((cf) => cf ? {...cf, name: e.target.value} : cf)}/>
                            </div>
                            <div className="modal-actions spread">
                                <span>
                                    {calForm.id && (
                                        <button className="btn danger" onClick={() => {
                                            const c = calendars.find((x) => x.id === calForm.id)
                                            if (c) setPendingCalDelete(c)
                                        }}>Delete</button>
                                    )}
                                </span>
                                <span className="cal-form-actions">
                                    <button className="btn" onClick={() => setCalForm(null)}>Cancel</button>
                                    <button className="btn primary" onClick={() => void saveCal()}
                                            disabled={busy || calForm.name.trim() === ''}>
                                        {busy ? 'Saving…' : (calForm.id ? 'Save calendar' : 'Add calendar')}
                                    </button>
                                </span>
                            </div>
                        </div>
                    )}
                    {!calForm && (
                        <div className="modal-actions spread">
                            <button className="btn" onClick={onClose}>Done</button>
                        </div>
                    )}
                </div>
            </div>

            {pendingCalDelete && (
                <ConfirmDialog
                    title="Delete calendar"
                    message={`Delete the calendar "${pendingCalDelete.name}"? Its events are deleted too. This cannot be undone.`}
                    confirmLabel="Delete"
                    busy={busy}
                    onConfirm={() => void confirmCalDelete()}
                    onCancel={() => setPendingCalDelete(null)}
                />
            )}
        </>
    )
}
