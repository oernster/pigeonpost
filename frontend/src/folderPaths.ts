import {Folder} from './api'

// detectSeparator infers the server's mailbox hierarchy delimiter from the folder paths. A character
// is the delimiter when some folder's path, split on it, yields a parent that is itself a folder (e.g.
// "Archived.Debt" alongside "Archived" means the delimiter is "."). It checks the two common IMAP
// delimiters and falls back to "/".
export function detectSeparator(paths: string[]): string {
    const set = new Set(paths)
    for (const sep of ['.', '/']) {
        for (const p of paths) {
            const idx = p.lastIndexOf(sep)
            if (idx > 0 && set.has(p.slice(0, idx))) {
                return sep
            }
        }
    }
    return '/'
}

// leafName returns the last segment of a path under the given separator.
export function leafName(path: string, sep: string): string {
    const idx = path.lastIndexOf(sep)
    return idx >= 0 ? path.slice(idx + 1) : path
}

// ancestorPaths returns every parent path of a folder path under the given separator.
export function ancestorPaths(path: string, sep: string): string[] {
    const parts = path.split(sep)
    const out: string[] = []
    for (let i = 1; i < parts.length; i++) {
        out.push(parts.slice(0, i).join(sep))
    }
    return out
}

// nearestParentPath returns the path of the folder's nearest ancestor that actually exists among the
// given set of folder paths, falling back to the empty string for a top-level folder. It skips over
// gaps where an intermediate path is not itself a folder, matching how orderFolders builds the tree.
export function nearestParentPath(path: string, existing: Set<string>, sep: string): string {
    const anc = ancestorPaths(path, sep)
    for (let i = anc.length - 1; i >= 0; i--) {
        if (existing.has(anc[i])) {
            return anc[i]
        }
    }
    return ''
}

// specialFolderOrder is the canonical top-to-bottom order for the well-known mailboxes, so Inbox,
// Sent and the rest sit at the top rather than wherever the server happens to list them. Any kind not
// named here (custom folders) ranks after all of these.
const specialFolderOrder = ['inbox', 'drafts', 'sent', 'archive', 'junk', 'trash']

// folderRank is a folder's primary sort key: the well-known mailboxes lead in specialFolderOrder.
// Every custom folder shares the one trailing rank, so customs only ever reorder amongst themselves.
export function folderRank(kind: string): number {
    const idx = specialFolderOrder.indexOf(kind)
    return idx === -1 ? specialFolderOrder.length : idx
}

// isJunkFolderMessage reports whether a message currently lives in a Junk folder, deciding whether
// the junk action reads "Mark as junk" or "Not junk" (the rescue back to the inbox).
export function isJunkFolderMessage(
    message: {folderId: string},
    folders: ReadonlyArray<{id: string; kind: string}>,
): boolean {
    return folders.some((f) => f.id === message.folderId && f.kind === 'junk')
}

// orderFolders reorders the folders for display so the well-known mailboxes lead (see folderRank) while
// every subtree stays contiguous under its parent. It walks the tree from the roots, sorting siblings at
// each level by rank first, then by the caller's persisted local order (orderPaths, a list of folder
// paths in the wanted order). A sibling absent from orderPaths ranks after every ordered one and keeps
// its server order, so a partial order still behaves. The path-index compare uses < and > rather than a
// subtraction so two unordered siblings (both at +Infinity) compare equal instead of yielding NaN.
export function orderFolders(folders: Folder[], sep: string, orderPaths: string[]): Folder[] {
    const existing = new Set(folders.map((f) => f.path))
    const orderIndex = new Map(orderPaths.map((p, i): [string, number] => [p, i]))
    const localOf = (f: Folder): number =>
        orderIndex.has(f.path) ? (orderIndex.get(f.path) as number) : Number.POSITIVE_INFINITY
    const childrenOf = new Map<string, Folder[]>()
    const roots: Folder[] = []
    folders.forEach((f) => {
        const parent = nearestParentPath(f.path, existing, sep)
        // A top-level folder has no ancestor folder, so nearestParentPath returns "". Only a genuine
        // ancestor (a non-empty path that exists) makes it a child.
        if (parent === '' && !existing.has('')) {
            roots.push(f)
            return
        }
        const siblings = childrenOf.get(parent) ?? []
        siblings.push(f)
        childrenOf.set(parent, siblings)
    })
    const sortSiblings = (arr: Folder[]): Folder[] =>
        [...arr].sort((a, b) => {
            const byRank = folderRank(a.kind) - folderRank(b.kind)
            if (byRank !== 0) {
                return byRank
            }
            const la = localOf(a)
            const lb = localOf(b)
            if (la < lb) {
                return -1
            }
            if (la > lb) {
                return 1
            }
            return 0
        })
    const ordered: Folder[] = []
    const walk = (f: Folder) => {
        ordered.push(f)
        sortSiblings(childrenOf.get(f.path) ?? []).forEach(walk)
    }
    sortSiblings(roots).forEach(walk)
    return ordered
}

// placeAdjacent returns a new persisted order list with movedPath positioned immediately before or after
// anchorPath within one sibling group. groupPaths is that group's current display order (it may already
// contain movedPath from a same-level reorder; it will not when a reparented folder joins a new group);
// the moved path is stripped then reinserted next to the anchor. The group's members are written to the
// front of the returned list, so their relative order is what sorting reads; every other recorded path
// follows with its order preserved (cross-group position never matters, since groups sort independently).
// When the anchor is not in the group the order is returned unchanged.
export function placeAdjacent(
    orderPaths: string[],
    groupPaths: string[],
    movedPath: string,
    anchorPath: string,
    after: boolean,
): string[] {
    const seq = groupPaths.filter((p) => p !== movedPath)
    const at = seq.indexOf(anchorPath)
    if (at < 0) {
        return orderPaths
    }
    seq.splice(after ? at + 1 : at, 0, movedPath)
    const inGroup = new Set(seq)
    return [...seq, ...orderPaths.filter((p) => !inGroup.has(p))]
}
