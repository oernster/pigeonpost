import type {Dispatch, SetStateAction} from 'react'
import {DateField} from './DateField'
import {fromDatetimeLocal, isSchedulable, sendLaterChoices} from '../schedule'

interface SendLaterRowProps {
    sendAtValue: string
    setSendAtValue: Dispatch<SetStateAction<string>>
    setSendLaterOpen: Dispatch<SetStateAction<boolean>>
    attemptSend: (at: Date | null) => void
}

// SendLaterRow offers the preset moments and a chosen date and time for a scheduled send. Choosing one closes
// the row and sends through the same attempt as Send, so the separator fix and the attachment reminder apply.
export function SendLaterRow({sendAtValue, setSendAtValue, setSendLaterOpen, attemptSend}: SendLaterRowProps) {
    return (
        <div className="compose-schedule-row" role="menu" aria-label="Send later">
            {sendLaterChoices(new Date()).map((choice) => (
                <button
                    key={choice.label}
                    type="button"
                    role="menuitem"
                    className="btn"
                    onClick={() => {
                        setSendLaterOpen(false)
                        attemptSend(choice.at)
                    }}
                >
                    {choice.label}
                </button>
            ))}
            <DateField
                kind="datetime-local"
                className="compose-schedule-input"
                ariaLabel="Send at"
                pickerTitle="Send date"
                compact
                value={sendAtValue}
                onChange={setSendAtValue}
            />
            <button
                type="button"
                className="btn primary"
                disabled={!isSchedulable(fromDatetimeLocal(sendAtValue), new Date())}
                onClick={() => {
                    const at = fromDatetimeLocal(sendAtValue)
                    if (at) {
                        setSendLaterOpen(false)
                        attemptSend(at)
                    }
                }}
            >
                Schedule
            </button>
            <div className="compose-schedule-note">
                Sends at the chosen time while PigeonPost is running, else at the next launch after it.
                Cancel any time from the Outbox.
            </div>
        </div>
    )
}
