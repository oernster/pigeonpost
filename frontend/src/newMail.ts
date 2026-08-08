// New-mail attention cue for accounts other than the selected one. The semantic is "new since you
// last looked", not "unread": an account with a standing unread backlog must not light a permanent
// cue, so each account carries a watermark (the last instant it was the selected account) and the
// cue compares the account's newest unread message date against it. The watermarks live here, in
// localStorage; the newest-unread dates come from the backend beside the unread counts.

export type Watermarks = {[accountId: string]: number}

// WATERMARKS_KEY is the localStorage slot holding the per-account watermarks as a JSON object of
// account id to Unix milliseconds.
export const WATERMARKS_KEY = 'pigeonpost.accountSeen'

// loadWatermarks reads the stored watermarks, treating a missing or unparseable slot as empty (the
// ensure step then initialises every account to now, so nothing lights on first run).
export function loadWatermarks(storage: Pick<Storage, 'getItem'>): Watermarks {
    try {
        const raw = storage.getItem(WATERMARKS_KEY)
        if (!raw) {
            return {}
        }
        const parsed: unknown = JSON.parse(raw)
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
            return {}
        }
        const marks: Watermarks = {}
        for (const [id, value] of Object.entries(parsed)) {
            if (typeof value === 'number' && Number.isFinite(value)) {
                marks[id] = value
            }
        }
        return marks
    } catch {
        return {}
    }
}

export function saveWatermarks(storage: Pick<Storage, 'setItem'>, marks: Watermarks): void {
    try {
        storage.setItem(WATERMARKS_KEY, JSON.stringify(marks))
    } catch {
        // Storage full or unavailable: the cue degrades to session-only state.
    }
}

// touchWatermarks stamps the given accounts as looked-at at now. Empty ids (no previous selection
// yet) are skipped.
export function touchWatermarks(marks: Watermarks, accountIds: string[], now: number): Watermarks {
    const next = {...marks}
    for (const id of accountIds) {
        if (id) {
            next[id] = now
        }
    }
    return next
}

// ensureWatermarks reconciles the watermarks with the account list: an account seen for the first
// time starts at now (so a pre-existing backlog never lights the cue) and an account no longer in
// the list is dropped. Returns the same object when nothing changed, so an effect keyed on the
// result does not loop.
export function ensureWatermarks(marks: Watermarks, accountIds: string[], now: number): Watermarks {
    const ids = new Set(accountIds)
    const missing = accountIds.filter((id) => !(id in marks))
    const stale = Object.keys(marks).filter((id) => !ids.has(id))
    if (missing.length === 0 && stale.length === 0) {
        return marks
    }
    const next: Watermarks = {}
    for (const [id, value] of Object.entries(marks)) {
        if (ids.has(id)) {
            next[id] = value
        }
    }
    for (const id of missing) {
        next[id] = now
    }
    return next
}

// ElsewhereCue is the closed picker's summary of mail that arrived on accounts other than the
// selected one: how many unread messages those accounts hold and how many accounts they are.
export interface ElsewhereCue {
    unread: number
    accounts: number
}

// elsewhereCue flags each non-selected account whose newest unread message postdates its watermark
// and sums those accounts' unread counts. An account without a watermark yet (the ensure step has
// not seen it) stays dark rather than lighting on backlog.
export function elsewhereCue(
    activeAccountId: string,
    unreadByAccount: {[accountId: string]: number},
    newestByAccount: {[accountId: string]: number},
    marks: Watermarks,
): ElsewhereCue {
    let unread = 0
    let accounts = 0
    for (const [id, count] of Object.entries(unreadByAccount)) {
        if (id === activeAccountId || count <= 0) {
            continue
        }
        const newest = newestByAccount[id]
        const mark = marks[id]
        if (newest !== undefined && mark !== undefined && newest > mark) {
            unread += count
            accounts += 1
        }
    }
    return {unread, accounts}
}

// elsewhereCueLabel spells the cue out for the tooltip and the screen reader, e.g.
// "3 unread on 1 other account".
export function elsewhereCueLabel(cue: ElsewhereCue): string {
    const accountsNoun = cue.accounts === 1 ? 'account' : 'accounts'
    return `${cue.unread} unread on ${cue.accounts} other ${accountsNoun}`
}
