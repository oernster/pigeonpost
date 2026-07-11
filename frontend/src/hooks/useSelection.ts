import {Dispatch, SetStateAction, useCallback, useState} from 'react'
import type {Message} from '../api'

// rangeIds returns the ids of the contiguous run of messages between fromId and toId in list (inclusive),
// the set a Shift click or a Shift arrow selects. When either end is not in the list it falls back to just
// toId, so a range taken from a stale anchor still selects the clicked row.
export function rangeIds(list: Message[], fromId: string | null, toId: string): Set<string> {
    const from = list.findIndex((m) => m.id === fromId)
    const to = list.findIndex((m) => m.id === toId)
    if (from === -1 || to === -1) {
        return new Set([toId])
    }
    const [lo, hi] = from <= to ? [from, to] : [to, from]
    return new Set(list.slice(lo, hi + 1).map((m) => m.id))
}

// toggleId adds or removes id from the current marked set, the gesture behind a Ctrl click or Ctrl+Space.
// When nothing is marked yet it seeds the set from fallbackId (the active message) first, so the first Ctrl
// gesture keeps what was already selected before toggling the new row in or out.
export function toggleId(prev: Set<string>, id: string, fallbackId: string | null): Set<string> {
    const base = prev.size ? new Set(prev) : new Set<string>(fallbackId ? [fallbackId] : [])
    if (base.has(id)) {
        base.delete(id)
    } else {
        base.add(id)
    }
    return base
}

// Selection is the multi-selection built by Ctrl and Shift gestures over the message list: the set of
// marked ids and the anchor a Shift range pivots from. Empty marks mean single-select mode, where the
// active message alone is the selection.
export interface Selection {
    markedIds: Set<string>
    setMarkedIds: Dispatch<SetStateAction<Set<string>>>
    anchorId: string | null
    setAnchorId: Dispatch<SetStateAction<string | null>>
    // clear drops the multi-selection back to single-select mode. It leaves the active message alone, so
    // clearing a selection does not close the message on screen.
    clear: () => void
}

export function useSelection(): Selection {
    const [markedIds, setMarkedIds] = useState<Set<string>>(new Set())
    const [anchorId, setAnchorId] = useState<string | null>(null)
    const clear = useCallback(() => {
        setMarkedIds(new Set())
        setAnchorId(null)
    }, [])
    return {markedIds, setMarkedIds, anchorId, setAnchorId, clear}
}
