import {Dispatch, SetStateAction, useCallback, useEffect, useState} from 'react'
import {Message, api} from '../api'
import type {MessageStore} from './useMessageStore'

// SnoozeDeps is what snoozing needs from the rest of App: the message store (a snoozed or unsnoozed
// row leaves the list it is on), the badge refresher (hidden unread mail stops counting) and the error
// sink.
export interface SnoozeDeps {
    store: MessageStore
    loadUnread: () => Promise<void>
    setError: (message: string) => void
}

export interface Snooze {
    // snoozedCount sizes the sidebar's Snoozed entry; the entry shows only while something is hidden.
    snoozedCount: number
    refreshSnoozedCount: () => Promise<void>
    // snoozeTo hides a message until the chosen moment; unsnooze brings it back at once. Both drop the
    // row from the on-screen lists (it now belongs elsewhere) and refresh the badges.
    snoozeTo: (message: Message, at: Date) => Promise<void>
    unsnooze: (message: Message) => Promise<void>
    // snoozePickerFor is the message whose custom pick-a-moment dialog is open, or null.
    snoozePickerFor: Message | null
    setSnoozePickerFor: Dispatch<SetStateAction<Message | null>>
}

// useSnooze owns the snooze actions and the Snoozed entry's count. Snooze is local-only state: the
// backend hides the message from the visible listings until it comes due, so the frontend's whole job
// is to drop the row, keep the count in step and offer the custom picker.
export function useSnooze(deps: SnoozeDeps): Snooze {
    const {store, loadUnread, setError} = deps
    const {removeFromAllLists} = store
    const [snoozedCount, setSnoozedCount] = useState(0)
    const [snoozePickerFor, setSnoozePickerFor] = useState<Message | null>(null)

    const refreshSnoozedCount = useCallback(async () => {
        try {
            setSnoozedCount(await api.snoozedCount())
        } catch {
            // A count refresh failure just leaves the badge stale; the next refresh corrects it.
        }
    }, [])

    useEffect(() => {
        void refreshSnoozedCount()
    }, [refreshSnoozedCount])

    const snoozeTo = useCallback(async (message: Message, at: Date) => {
        setError('')
        try {
            await api.snoozeMessage(message.id, at.getTime())
            removeFromAllLists(new Set([message.id]))
            await refreshSnoozedCount()
            await loadUnread()
        } catch (e) {
            setError(String(e))
        }
    }, [removeFromAllLists, refreshSnoozedCount, loadUnread])

    const unsnooze = useCallback(async (message: Message) => {
        setError('')
        try {
            await api.unsnoozeMessage(message.id)
            // The row leaves the Snoozed view it is shown in; it is back in its real folder on the
            // next visit there.
            removeFromAllLists(new Set([message.id]))
            await refreshSnoozedCount()
            await loadUnread()
        } catch (e) {
            setError(String(e))
        }
    }, [removeFromAllLists, refreshSnoozedCount, loadUnread])

    return {snoozedCount, refreshSnoozedCount, snoozeTo, unsnooze, snoozePickerFor, setSnoozePickerFor}
}
