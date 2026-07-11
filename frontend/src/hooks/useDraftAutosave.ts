import {useEffect, useRef, useState} from 'react'
import type {Editor} from '@tiptap/react'
import {api} from '../api'

// autosaveDelayMs debounces local draft-recovery autosaves, so a snapshot is written a short pause after
// the user stops typing rather than on every keystroke.
const autosaveDelayMs = 1500

interface DraftFields {
    accountId: string
    to: string
    cc: string
    bcc: string
    subject: string
    editor: Editor | null
}

// useDraftAutosave debounces a local draft-recovery snapshot of the in-progress compose, so an accidental
// close or a crash does not lose it. It runs only after a real edit (markDirty from the recipient fields, or
// noteEdit from the editor), clears the slot when the compose is emptied back out and stops once the message
// has been sent or saved to the server (stopAutosave), so a pending debounce cannot re-save a stale copy.
// This is local only and never touches the server. noteEdit is the editor's onUpdate: it marks the compose
// dirty and bumps a tick so the effect re-runs on a body edit.
export function useDraftAutosave({accountId, to, cc, bcc, subject, editor}: DraftFields) {
    // bodyTick bumps on each editor edit so the effect re-runs; dirtyRef gates it to real user edits so an
    // untouched reply or forward template is never snapshotted; stopRef halts it once the message has been
    // sent or saved to the server.
    const [bodyTick, setBodyTick] = useState(0)
    const dirtyRef = useRef(false)
    const stopRef = useRef(false)
    const markDirty = () => {
        dirtyRef.current = true
    }
    const noteEdit = () => {
        markDirty()
        setBodyTick((tick) => tick + 1)
    }
    const stopAutosave = () => {
        stopRef.current = true
    }
    const resumeAutosave = () => {
        stopRef.current = false
    }

    useEffect(() => {
        if (!dirtyRef.current || stopRef.current) return
        const bodyText = editor?.getText() ?? ''
        const hasContent = to.trim() !== '' || cc.trim() !== '' || bcc.trim() !== '' ||
            subject.trim() !== '' || bodyText.trim() !== ''
        const timer = window.setTimeout(() => {
            if (stopRef.current) return
            if (!hasContent) {
                void api.clearDraftRecovery()
                return
            }
            void api.saveDraftRecovery({
                accountId,
                to,
                cc,
                bcc,
                subject,
                bodyHtml: editor?.getHTML() ?? '',
            })
        }, autosaveDelayMs)
        return () => window.clearTimeout(timer)
    }, [accountId, to, cc, bcc, subject, bodyTick, editor])

    return {markDirty, noteEdit, stopAutosave, resumeAutosave}
}
