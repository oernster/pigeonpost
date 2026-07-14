import {useEffect, useState} from 'react'

export interface UndoSendToastProps {
    // expiresAt is the end of the undo window as a Unix-milliseconds timestamp.
    expiresAt: number
    onUndo: () => void
    // onExpired is called once when the window elapses, so the parent can drop the toast and apply the
    // deferred reply/forward marking. The message itself is sent by the backend dispatcher.
    onExpired: () => void
}

// undoToastTickMs is how often the countdown label refreshes.
const undoToastTickMs = 250

// millisPerSecond converts the remaining window to whole display seconds.
const millisPerSecond = 1000

// UndoSendToast is the countdown shown after a held send: the message leaves when the window elapses
// unless Undo is clicked first. It owns only the display timer; cancelling and expiry consequences live
// in the parent.
export function UndoSendToast({expiresAt, onUndo, onExpired}: UndoSendToastProps) {
    const [now, setNow] = useState<number>(() => Date.now())

    useEffect(() => {
        const interval = window.setInterval(() => setNow(Date.now()), undoToastTickMs)
        return () => window.clearInterval(interval)
    }, [])

    const remainingMs = expiresAt - now
    useEffect(() => {
        if (remainingMs <= 0) {
            onExpired()
        }
    }, [remainingMs <= 0]) // eslint-disable-line react-hooks/exhaustive-deps

    if (remainingMs <= 0) {
        return null
    }
    const seconds = Math.ceil(remainingMs / millisPerSecond)
    return (
        <div className="undo-send-toast" role="status">
            <span className="undo-send-text">Sending in {seconds}s</span>
            <button className="btn undo-send-btn" onClick={onUndo}>Undo</button>
        </div>
    )
}
