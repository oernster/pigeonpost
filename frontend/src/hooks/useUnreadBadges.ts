import {useCallback, useEffect, useState} from 'react'
import {UnreadCountsResult, api} from '../api'

export interface UnreadBadgesDeps {
    // refreshFolders reloads the selected account's folder list, whose rows carry the per-folder unread badge.
    refreshFolders: () => Promise<void>
    setError: (message: string) => void
}

export interface UnreadBadges {
    unreadCounts: UnreadCountsResult
    loadUnread: () => Promise<void>
    refreshBadges: () => Promise<void>
}

// useUnreadBadges owns the unread badges. They come from two separate reads: the counts badge the titlebar
// and the account picker, while the per-folder badge rides on the folder list. An action that changes what
// is unread where must refresh both, so refreshBadges is the one place that pairs them; refreshing only one
// is the stale-badge bug the pairing exists to prevent. loadUnread alone serves the paths that change only
// the counts (a sync, a snooze, the outbox, opening a folder).
export function useUnreadBadges(deps: UnreadBadgesDeps): UnreadBadges {
    const {refreshFolders, setError} = deps
    const [unreadCounts, setUnreadCounts] = useState<UnreadCountsResult>(
        {total: 0, byAccount: {}, newestByAccount: {}},
    )

    // loadUnread refreshes the per-account and cross-account unread counts from the local cache. It reports
    // its own failure through the error sink and never rejects.
    const loadUnread = useCallback(async () => {
        try {
            setUnreadCounts(await api.unreadCounts())
        } catch (e) {
            setError(String(e))
        }
    }, [setError])

    useEffect(() => {
        void loadUnread()
    }, [loadUnread])

    // refreshBadges refreshes both surfaces together. Only the folder list can reject, so a caller keeps its
    // own handling for that.
    const refreshBadges = useCallback(async () => {
        await Promise.all([loadUnread(), refreshFolders()])
    }, [loadUnread, refreshFolders])

    return {unreadCounts, loadUnread, refreshBadges}
}
