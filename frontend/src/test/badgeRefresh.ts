import {vi} from 'vitest'

// badgeRefresh stands in for the unread badge refresh a hook is handed (useUnreadBadges.refreshBadges),
// recording whether a path refreshed the badges. Every badge-changing path is pinned through `refreshed`,
// so a path that stops refreshing fails its test by name.
export function badgeRefresh() {
    const refreshBadges = vi.fn(async () => {})
    return {
        deps: {refreshBadges},
        refreshed: () => refreshBadges.mock.calls.length > 0,
        // untouched is true when the badges were not refreshed, for the paths that must leave them be.
        untouched: () => refreshBadges.mock.calls.length === 0,
        reset: () => refreshBadges.mockClear(),
    }
}
