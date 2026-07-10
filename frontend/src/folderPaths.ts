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

// MoveTarget is one destination a folder can be reparented to. An empty id means the top level.
export interface MoveTarget {
    id: string
    label: string
}

// moveTargets returns the destinations a folder may be reparented to: a Top level entry (unless the
// folder already sits at the top level) followed by every other folder that is not the folder itself,
// not one of its descendants and not its current parent (moving there would be a no-op). Folder targets
// are ordered by their full path so the list is stable. The caller passes the account's whole folder set.
export function moveTargets(folder: Folder, folders: Folder[]): MoveTarget[] {
    const sep = detectSeparator(folders.map((f) => f.path))
    const parents = ancestorPaths(folder.path, sep)
    const currentParentPath = parents.length > 0 ? parents[parents.length - 1] : ''
    const eligible = folders
        .filter((f) => f.id !== folder.id)
        .filter((f) => f.path !== folder.path && !f.path.startsWith(folder.path + sep))
        .filter((f) => f.path !== currentParentPath)
        .sort((a, b) => a.path.localeCompare(b.path))
        .map((f): MoveTarget => ({id: f.id, label: f.path}))
    const top: MoveTarget[] = currentParentPath === '' ? [] : [{id: '', label: 'Top level'}]
    return [...top, ...eligible]
}
