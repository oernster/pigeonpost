import {useEffect, useRef, useState} from 'react'
import {ConversationEntry, api} from '../api'

// useConversation loads the thread a message belongs to: every cached message that shares its subject
// once reply and forward prefixes are stripped, across all of the account's folders, oldest first. The
// reading list groups conversations within one folder, so the answer you sent (which lives in Sent) never
// appears beside the message it answers; this is what makes the thread walkable.
//
// It is keyed on a message id rather than a message so the two surfaces that need a thread share one
// loader: the reader's strip, which passes the open message, plus the thread view, which passes the
// conversation the list was asked to open.
//
// The lookup is local and cheap but still asynchronous, so each load is claimed by the id it was started
// for: moving to a second message while the first is in flight discards the late result rather than
// showing one message's thread under another. No message means no entries.
export function useConversation(messageId: string | null): ConversationEntry[] {
    const [entries, setEntries] = useState<ConversationEntry[]>([])
    // wanted mirrors the id the entries belong to, so a resolved load can tell whether the surface has
    // moved on since it started.
    const wanted = useRef<string>('')

    useEffect(() => {
        const id = messageId ?? ''
        wanted.current = id
        if (id === '') {
            setEntries([])
            return
        }
        void (async () => {
            try {
                const loaded = await api.conversation(id)
                if (wanted.current === id) {
                    setEntries(loaded)
                }
            } catch {
                // The thread is an aid to navigation, not the message itself: a failed lookup leaves the
                // surface empty rather than putting an error over a message that opened perfectly well.
                if (wanted.current === id) {
                    setEntries([])
                }
            }
        })()
    }, [messageId])

    return entries
}
