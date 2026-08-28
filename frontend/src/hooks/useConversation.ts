import {useEffect, useRef, useState} from 'react'
import {ConversationEntry, Message, api} from '../api'

// useConversation loads the thread the open message belongs to: every cached message that shares its
// subject once reply and forward prefixes are stripped, across all of the account's folders, oldest
// first. The reading list groups conversations within one folder, so the answer you sent (which lives in
// Sent) never appears beside the message it answers; this is what makes the thread walkable.
//
// The lookup is local and cheap but still asynchronous, so each load is claimed by the message it
// was started for: opening a second message while the first is in flight discards the late result rather
// than showing one message's thread under another. A closed reader holds no entries.
export function useConversation(message: Message | null): ConversationEntry[] {
    const [entries, setEntries] = useState<ConversationEntry[]>([])
    // wanted mirrors the message the entries belong to, so a resolved load can tell whether the reader
    // has moved on since it started.
    const wanted = useRef<string>('')

    useEffect(() => {
        const id = message?.id ?? ''
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
                // strip empty rather than putting an error over a message that opened perfectly well.
                if (wanted.current === id) {
                    setEntries([])
                }
            }
        })()
    }, [message?.id])

    return entries
}
