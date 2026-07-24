import {useState} from 'react'
import type {Editor} from '@tiptap/react'
import {
    attachmentBytes,
    DataAttachment,
    fileToDataAttachment,
    fileToDataURI,
    FileIntakePlan,
    inlineImageBytes,
    intakeSize,
    MAX_ATTACHMENT_MEBIBYTES,
    MAX_TOTAL_ATTACHMENT_BYTES,
    planFileIntake,
    transferFilePaths,
    transferFiles,
} from '../composeIntake'

interface IntakeDeps {
    editor: Editor | null
    setError: (value: string) => void
    markDirty: () => void
    // addPaths appends path-based attachments: a paste that carries file:// URIs instead of File
    // objects (WebKit never hands Finder-copied files to the page as Files) attaches by path through
    // the same flow as the Attach files picker.
    addPaths: (paths: string[]) => void
    // initial restores the in-memory attachments an undone send carried, so the reopened compose holds
    // exactly what it held.
    initial?: DataAttachment[]
}

// useComposeIntake owns the files that arrive in the compose window by paste or drop. The rule is one
// sentence: images embed in the body at the cursor (keeping their original bytes), every other file
// becomes an in-memory attachment (the webview has its name and bytes but no filesystem path, so it is
// carried as base64 attachmentData rather than a path). A batch that would break the attachment size
// cap is refused whole, with the backend enforcing the same cap authoritatively at send time.
export function useComposeIntake({editor, setError, markDirty, addPaths, initial}: IntakeDeps) {
    const [dataAttachments, setDataAttachments] = useState<DataAttachment[]>(initial ?? [])

    // apply reads the planned files in: embedded images become data: URI image nodes at the cursor and
    // the rest join the in-memory attachments, then the draft is marked dirty.
    const apply = async (plan: FileIntakePlan) => {
        try {
            for (const file of plan.embed) {
                const src = await fileToDataURI(file)
                editor?.chain().focus().setImage({src}).run()
            }
            if (plan.attach.length > 0) {
                const read = await Promise.all(plan.attach.map(fileToDataAttachment))
                setDataAttachments((prev) => [...prev, ...read])
            }
            markDirty()
        } catch (e) {
            setError(String(e))
        }
    }

    // take is the single entry for files arriving by paste or drop: the editor's handlePaste and
    // handleDrop plus the modal's own drop zone all land here. It reports whether it took the event, so
    // a file-free paste or drop falls through to the editor's default handling. Files arrive as File
    // objects where the engine provides them (files, or WebKit's items-only pasted images); a paste
    // carrying only file:// URIs attaches by path instead.
    const take = (dt: DataTransfer | null): boolean => {
        const files = transferFiles(dt)
        if (files.length === 0) {
            const paths = transferFilePaths(dt)
            if (paths.length === 0) {
                return false
            }
            addPaths(paths)
            markDirty()
            return true
        }
        const held = attachmentBytes(dataAttachments) + inlineImageBytes(editor?.getHTML() ?? '')
        if (held + intakeSize(files) > MAX_TOTAL_ATTACHMENT_BYTES) {
            setError(`Adding these files would exceed the ${MAX_ATTACHMENT_MEBIBYTES} MB attachment limit.`)
            return true
        }
        void apply(planFileIntake(files))
        return true
    }

    const remove = (index: number) => {
        setDataAttachments((prev) => prev.filter((_, i) => i !== index))
    }

    return {dataAttachments, take, remove}
}
