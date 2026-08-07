// optimisticList holds the pure list surgery behind an optimistic removal: lifting rows out of a message
// list the moment an action is issued, and putting them back at the positions they held when the server
// refuses. No React and no api seam, so it sits under the 100% coverage gate; the hooks that own the lists
// call it.
import type {Message} from './api'

// Lifted records a message taken out of a list, with enough to put it back in the same gap.
export interface Lifted {
    // afterId is the id of the nearest message before this one that STAYED in the list, so the gap is
    // named by a row that is still there. Empty means the message sat at the head, or that everything
    // before it was lifted too. An absolute index cannot serve here: a partial move leaves some lifted
    // rows gone for good, which shifts every index after them.
    afterId: string
    // index is the position the message held in the original list, used only to order several restores
    // and as the fallback when its anchor has itself since left the list.
    index: number
    message: Message
}

// takeOut removes every message whose id is in ids and reports what was taken. The input is not mutated.
export function takeOut(list: readonly Message[], ids: ReadonlySet<string>): {next: Message[]; lifted: Lifted[]} {
    const next: Message[] = []
    const lifted: Lifted[] = []
    let anchor = ''
    list.forEach((message, index) => {
        if (ids.has(message.id)) {
            lifted.push({afterId: anchor, index, message})
        } else {
            next.push(message)
            anchor = message.id
        }
    })
    return {next, lifted}
}

// putBack splices the lifted messages back into list, each into the gap after its anchor, skipping any id
// the list already carries (the server may have re-listed it meanwhile). Several rows sharing one anchor go
// back in their original order. The input is not mutated.
export function putBack(list: readonly Message[], lifted: readonly Lifted[]): Message[] {
    const present = new Set(list.map((m) => m.id))
    const next = [...list]
    // placed counts how many rows have already gone back after each anchor, so the second row sharing an
    // anchor lands after the first rather than displacing it.
    const placed = new Map<string, number>()
    for (const entry of [...lifted].sort((a, b) => a.index - b.index)) {
        if (present.has(entry.message.id)) {
            continue
        }
        present.add(entry.message.id)
        const offset = placed.get(entry.afterId) ?? 0
        placed.set(entry.afterId, offset + 1)
        next.splice(Math.min(insertIndex(next, entry, offset), next.length), 0, entry.message)
    }
    return next
}

// insertIndex resolves where a lifted message goes back: at the head when it had no anchor, just after its
// anchor when that row is still listed, and otherwise at its remembered index as a last resort.
function insertIndex(list: readonly Message[], entry: Lifted, offset: number): number {
    if (entry.afterId === '') {
        return offset
    }
    const anchorAt = list.findIndex((m) => m.id === entry.afterId)
    return anchorAt === -1 ? entry.index : anchorAt + 1 + offset
}
