import {useCallback, useState} from 'react'
import {api} from '../api'
import type {ComposeInitial} from '../components/ComposeModal'

// useUndoSend owns the window between clicking Send and the message actually leaving. The backend
// queues a held send and returns its outbox id; this holds that id with the compose state to restore,
// and decides what happens at each of the two ways the window can end.
//
// Undoing cancels the queued item and reopens the composer exactly as it was. Letting it elapse applies
// the reply or forward marking that was deliberately deferred: an original must not be flagged answered
// while its reply can still be pulled back, so the flag is written at the moment the message becomes
// unrecallable, which is where an immediate send would have written it.
//
// One send at a time: a new held send replaces the toast. The previous message's window keeps running in
// the backend, it just loses its button; the Outbox folder still offers Cancel send.
const MILLISECONDS_PER_SECOND = 1000

// HeldSend is the live undo window: the queued item to cancel, the instant the window ends and the
// compose state to restore if it is undone.
interface HeldSend {
    outboxId: string
    expiresAt: number
    reopen: ComposeInitial
}

export interface UndoSendOptions {
    // holdSeconds is the user's chosen window, which sets when the toast expires.
    holdSeconds: number
    refreshOutbox: () => Promise<void>
    setError: (message: string) => void
    // reopenCompose puts the composer back with the undone message's own state.
    reopenCompose: (initial: ComposeInitial) => void
    markReplied: (messageId: string) => Promise<void>
    markForwarded: (messageId: string) => Promise<void>
}

export function useUndoSend(options: UndoSendOptions) {
    const {holdSeconds, refreshOutbox, setError, reopenCompose, markReplied, markForwarded} = options
    const [held, setHeld] = useState<HeldSend | null>(null)

    // onHeldSend opens the window for a send the backend has queued rather than delivered.
    const onHeldSend = useCallback((outboxId: string, reopen: ComposeInitial) => {
        setHeld({outboxId, expiresAt: Date.now() + holdSeconds * MILLISECONDS_PER_SECOND, reopen})
    }, [holdSeconds])

    // undoHeldSend stops the queued send and reopens the composer exactly as it was. When the cancel
    // loses the race (the message left in the same instant), it says so instead of pretending.
    const undoHeldSend = useCallback(async () => {
        if (!held) {
            return
        }
        const toast = held
        setHeld(null)
        try {
            const stopped = await api.cancelOutboxItem(toast.outboxId)
            await refreshOutbox()
            if (!stopped) {
                setError('That message had already been sent.')
                return
            }
            reopenCompose(toast.reopen)
        } catch (e) {
            setError(String(e))
        }
    }, [held, refreshOutbox, setError, reopenCompose])

    // heldSendElapsed drops the window once it ends and applies the deferred reply or forward marking:
    // the message is now leaving, so the original's glyph becomes true the way an immediate send's would
    // have been.
    const heldSendElapsed = useCallback(() => {
        if (!held) {
            return
        }
        const {reopen} = held
        setHeld(null)
        if (reopen.inReplyToId) {
            if (reopen.replyKind === 'reply') {
                void markReplied(reopen.inReplyToId)
            } else if (reopen.replyKind === 'forward') {
                void markForwarded(reopen.inReplyToId)
            }
        }
    }, [held, markReplied, markForwarded])

    return {held, onHeldSend, undoHeldSend, heldSendElapsed}
}
