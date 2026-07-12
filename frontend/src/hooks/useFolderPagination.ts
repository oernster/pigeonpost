import {useCallback, useMemo, useRef} from 'react'
import {MESSAGE_PAGE_SIZE, Message, MessagePage, api} from '../api'

// useFolderPagination tracks the keyset cursor for the flat folder view so a huge folder (a real Trash of
// tens of thousands of messages) loads one page at a time instead of every row at once. It owns only the
// cursor, the sort direction it was loaded in and an in-flight guard; the caller owns the message list and
// applies the returned rows. The cursor is opaque: it is recorded from a page and passed straight back to
// fetch the next one, never constructed here.
export interface FolderPagination {
    // reset abandons the current cursor and any in-flight guard, so the next loadFirst starts a folder (or
    // a new sort direction) fresh and a stray loadNext before it does nothing.
    reset: () => void
    // loadFirst fetches page one of a folder in the given direction, records the cursor and whether more
    // pages remain, and returns the page's messages. hasCursor is false, so the backend ignores the cursor.
    loadFirst: (folderId: string, ascending: boolean) => Promise<Message[]>
    // loadNext fetches the page after the recorded cursor, in the direction loadFirst was called with, and
    // returns only the rows not already in loadedIds. It fetches nothing and returns an empty array when no
    // more pages remain or a load is already running, so overlapping scroll triggers cannot double-fetch or
    // append duplicates.
    loadNext: (folderId: string, loadedIds: Set<string>) => Promise<Message[]>
    // hasMore reports whether another page can be loaded after the last recorded cursor.
    hasMore: () => boolean
}

interface Cursor {
    dateMs: number
    id: string
    ascending: boolean
    hasMore: boolean
    loading: boolean
}

function freshCursor(): Cursor {
    return {dateMs: 0, id: '', ascending: false, hasMore: false, loading: false}
}

export function useFolderPagination(): FolderPagination {
    const cursorRef = useRef<Cursor>(freshCursor())

    const record = (page: MessagePage) => {
        const cursor = cursorRef.current
        cursor.dateMs = page.nextCursorDateMs
        cursor.id = page.nextCursorId
        cursor.hasMore = page.hasMore
    }

    const reset = useCallback(() => {
        cursorRef.current = freshCursor()
    }, [])

    const loadFirst = useCallback(async (folderId: string, ascending: boolean): Promise<Message[]> => {
        cursorRef.current = {...freshCursor(), ascending}
        const page = await api.listMessagesPage(folderId, false, 0, '', MESSAGE_PAGE_SIZE, ascending)
        record(page)
        return page.messages
    }, [])

    const loadNext = useCallback(async (folderId: string, loadedIds: Set<string>): Promise<Message[]> => {
        const cursor = cursorRef.current
        if (!cursor.hasMore || cursor.loading) {
            return []
        }
        cursor.loading = true
        try {
            const page = await api.listMessagesPage(
                folderId, true, cursor.dateMs, cursor.id, MESSAGE_PAGE_SIZE, cursor.ascending,
            )
            // A reset or loadFirst during the fetch (the user switched folder or flipped the sort) replaces
            // the cursor object. Discard this stale page rather than record its cursor over the new folder's,
            // which would skip or misorder that folder's pages.
            if (cursorRef.current !== cursor) {
                return []
            }
            record(page)
            return page.messages.filter((message) => !loadedIds.has(message.id))
        } finally {
            cursor.loading = false
        }
    }, [])

    const hasMore = useCallback(() => cursorRef.current.hasMore, [])

    // Memoise the facade so it is a stable dependency for the callers' useCallback/useEffect that reload
    // the folder; its methods are already stable, so this object never needs to change.
    return useMemo(() => ({reset, loadFirst, loadNext, hasMore}), [reset, loadFirst, loadNext, hasMore])
}
