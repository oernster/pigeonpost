import {useCallback, useMemo, useRef, useState} from 'react'
import {api} from '../api'
import {UndoEntry, groupBySource, pushEntry, rebindMoveItems, redoLabel, undoLabel} from '../undoStack'
import type {MessageStore} from './useMessageStore'

// TagExecutor re-applies or reverts one colour tag. It lives with useTags, which owns the tag
// palette the optimistic dot update needs, and is registered here so a tag entry can execute
// without this hook depending on the tag hook (or the other way round).
export type TagExecutor = (messageId: string, tagId: string, assigned: boolean) => Promise<void>

// UndoRecorder is the slice of this hook the action hooks receive: push records a completed
// action; registerTagExecutor hands over the tag executor once useTags exists.
export interface UndoRecorder {
    push: (entry: UndoEntry) => void
    registerTagExecutor: (fn: TagExecutor) => void
}

export interface UndoRedoDeps {
    store: MessageStore
    loadUnread: () => Promise<void>
    refreshFolders: () => Promise<void>
    setError: (message: string) => void
}

export interface UndoRedo {
    recorder: UndoRecorder
    undo: () => Promise<void>
    redo: () => Promise<void>
    // undoText / redoText name the top entry ("Undo delete") for the menu items, or null when the
    // stack is empty and the item is disabled.
    undoText: string | null
    redoText: string | null
}

// useUndoRedo owns the Edit-menu undo and redo stacks, Thunderbird-style but wider: the
// move-shaped actions (move, delete-to-Trash, junk, rescue, bulk) plus the read, star and tag
// toggles. Move-shaped entries carry the id each message now holds (from the server's COPYUID
// reply); executing one moves the messages back and rebinds the entry with where they landed for
// the opposite stack, so undo and redo can ping-pong indefinitely. An entry that fails to execute
// (moved on by another client, folder gone) reports through the error sink and is dropped rather
// than wedging the stack. Only actions the server located are recorded, so Undo never lies.
export function useUndoRedo(deps: UndoRedoDeps): UndoRedo {
    const {store, loadUnread, refreshFolders, setError} = deps
    const {applyToAllLists, removeFromAllLists} = store

    const [undoStack, setUndoStack] = useState<UndoEntry[]>([])
    const [redoStack, setRedoStack] = useState<UndoEntry[]>([])
    const tagExecutor = useRef<TagExecutor | null>(null)
    // running guards against a second undo firing while the first's server calls are in flight,
    // which would execute a stale copy of the stack.
    const running = useRef(false)

    const push = useCallback((entry: UndoEntry) => {
        setUndoStack((prev) => pushEntry(prev, entry))
        // A fresh action forks history: what was undone can no longer be redone.
        setRedoStack([])
    }, [])

    const registerTagExecutor = useCallback((fn: TagExecutor) => {
        tagExecutor.current = fn
    }, [])

    const refreshBadges = useCallback(async () => {
        await Promise.all([loadUnread(), refreshFolders()])
    }, [loadUnread, refreshFolders])

    // syncFolder pulls a destination folder's listing straight away so the returned messages
    // appear there immediately, the same best-effort contract as the action hooks.
    const syncFolder = useCallback(async (folderId: string) => {
        try {
            await api.syncFolder(folderId)
        } catch {
            // The next background sync reconciles.
        }
    }, [])

    // executeMove carries a move-shaped entry out. Undoing returns each message to its source
    // folder; redoing re-applies the original action through the api that created it (a delete
    // re-resolves Trash, a junking rewrites the spam keywords). It returns the entry rebound to
    // where the messages landed, or null when the server reported none of them (the entry is spent).
    const executeMove = useCallback(async (
        entry: Extract<UndoEntry, {kind: 'move'}>, direction: 'undo' | 'redo',
    ): Promise<UndoEntry | null> => {
        const landed: Record<string, string> = {}
        const executed = new Set<string>()
        if (direction === 'undo') {
            for (const group of groupBySource(entry.items)) {
                const result = await api.moveMessages(group.ids, group.folderId)
                result.ids.forEach((id) => executed.add(id))
                Object.assign(landed, result.newIds)
                await syncFolder(group.folderId)
            }
        } else if (entry.flavour === 'delete') {
            const ids = entry.items.map((i) => i.messageId)
            const result = await api.deleteMessages(ids)
            result.ids.forEach((id) => executed.add(id))
            Object.assign(landed, result.newIds)
        } else if (entry.flavour === 'junk' || entry.flavour === 'notJunk') {
            for (const item of entry.items) {
                const result = entry.flavour === 'junk'
                    ? await api.markJunk(item.messageId)
                    : await api.markNotJunk(item.messageId)
                executed.add(item.messageId)
                if (result.newId) {
                    landed[item.messageId] = result.newId
                }
            }
            await syncFolder(entry.destFolderId)
        } else {
            const ids = entry.items.map((i) => i.messageId)
            const result = await api.moveMessages(ids, entry.destFolderId)
            result.ids.forEach((id) => executed.add(id))
            Object.assign(landed, result.newIds)
            await syncFolder(entry.destFolderId)
        }
        removeFromAllLists(executed)
        await refreshBadges()
        return rebindMoveItems(entry, landed)
    }, [removeFromAllLists, syncFolder, refreshBadges])

    // executeToggle restores each message's recorded value (undo) or re-applies the action's
    // value (redo), optimistically in every list and then on the server. The entry itself is its
    // own inverse: the ids are stable and both directions' values are stored.
    const executeToggle = useCallback(async (
        entry: Extract<UndoEntry, {kind: 'read' | 'flag'}>, direction: 'undo' | 'redo',
    ): Promise<UndoEntry> => {
        const valueFor = new Map(entry.items.map((i) => [i.messageId, direction === 'undo' ? i.before : entry.after]))
        applyToAllLists((m) => {
            const value = valueFor.get(m.id)
            if (value === undefined) {
                return m
            }
            return entry.kind === 'read' ? {...m, read: value} : {...m, flagged: value}
        })
        for (const item of entry.items) {
            const value = direction === 'undo' ? item.before : entry.after
            if (entry.kind === 'read') {
                await api.markRead(item.messageId, value)
            } else {
                await api.markFlagged(item.messageId, value)
            }
        }
        if (entry.kind === 'read') {
            await refreshBadges()
        }
        return entry
    }, [applyToAllLists, refreshBadges])

    const execute = useCallback(async (entry: UndoEntry, direction: 'undo' | 'redo'): Promise<UndoEntry | null> => {
        if (entry.kind === 'move') {
            return executeMove(entry, direction)
        }
        if (entry.kind === 'tag') {
            const assigned = direction === 'undo' ? !entry.assigned : entry.assigned
            await tagExecutor.current?.(entry.messageId, entry.tagId, assigned)
            return entry
        }
        return executeToggle(entry, direction)
    }, [executeMove, executeToggle])

    // run pops the top entry from one stack, executes it and pushes the rebound inverse onto the
    // other. A failed or spent entry is dropped: retrying a stale id would fail the same way.
    const run = useCallback(async (direction: 'undo' | 'redo') => {
        if (running.current) {
            return
        }
        const source = direction === 'undo' ? undoStack : redoStack
        const entry = source[source.length - 1]
        if (!entry) {
            return
        }
        running.current = true
        const setSource = direction === 'undo' ? setUndoStack : setRedoStack
        const setOther = direction === 'undo' ? setRedoStack : setUndoStack
        setSource((prev) => prev.slice(0, -1))
        setError('')
        try {
            const inverse = await execute(entry, direction)
            if (inverse) {
                setOther((prev) => pushEntry(prev, inverse))
            }
        } catch (e) {
            setError(String(e))
        } finally {
            running.current = false
        }
    }, [undoStack, redoStack, execute, setError])

    const undo = useCallback(() => run('undo'), [run])
    const redo = useCallback(() => run('redo'), [run])

    // recorder is memoised so the action hooks that take it as a dependency see one stable object.
    const recorder = useMemo(() => ({push, registerTagExecutor}), [push, registerTagExecutor])

    const topUndo = undoStack[undoStack.length - 1]
    const topRedo = redoStack[redoStack.length - 1]
    return {
        recorder,
        undo, redo,
        undoText: topUndo ? undoLabel(topUndo) : null,
        redoText: topRedo ? redoLabel(topRedo) : null,
    }
}
