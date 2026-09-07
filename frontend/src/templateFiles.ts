import {basename} from './composeAddresses'
import type {TemplateAttachment} from './api'

// templateFiles is the pure half of editing the files a template carries. The editor holds ONE ordered
// list mixing two kinds of entry: files already stored on the template, which are named by position
// rather than resent, plus files just picked from disk, which travel as paths. Keeping both in one list
// is what lets a chip be removed without caring which kind it is.

// STORED_ELSEWHERE marks an entry that is not yet on the template: a file just picked, which has a path
// and no stored position.
export const STORED_ELSEWHERE = -1

// TemplateEditorFile is one chip in the editor. size is the stored byte count; a newly picked file has
// none, since its bytes are read by the backend at save time and nothing has looked at the file yet.
export interface TemplateEditorFile {
    position: number
    path: string
    name: string
    size: number
}

// storedFiles turns a template's descriptions into editor entries, each remembering the position it
// occupies on the stored template. That position is the whole contract with the backend: the save says
// which stored files to carry over by naming them.
export function storedFiles(attachments: TemplateAttachment[]): TemplateEditorFile[] {
    return attachments.map((a, position) => ({
        position,
        path: '',
        name: a.filename,
        size: a.size,
    }))
}

// pickedFiles turns chosen paths into editor entries, skipping any whose path is already in the list so
// the same file cannot be attached twice, exactly as the compose window's picker does.
export function pickedFiles(paths: string[], existing: TemplateEditorFile[]): TemplateEditorFile[] {
    const taken = new Set(existing.map((f) => f.path).filter((p) => p !== ''))
    return paths
        .filter((path) => !taken.has(path))
        .map((path) => ({position: STORED_ELSEWHERE, path, name: basename(path), size: 0}))
}

// SaveFiles is the pair of fields the save request carries: the stored positions to keep, in the order
// they should end up in, then the paths to read and add.
export interface SaveFiles {
    keepFilePositions: number[]
    addFilePaths: string[]
}

// saveFiles splits the editor's list into what the request states. The backend stores the kept files
// first and the added ones after, which is the order they are listed in here, so what the editor showed
// is what comes back.
export function saveFiles(files: TemplateEditorFile[]): SaveFiles {
    return {
        keepFilePositions: files.filter((f) => f.position !== STORED_ELSEWHERE).map((f) => f.position),
        addFilePaths: files.filter((f) => f.position === STORED_ELSEWHERE).map((f) => f.path),
    }
}

// removeFileAt drops one chip, addressed by its index in the displayed list rather than by name or
// path, since two files may share either.
export function removeFileAt(files: TemplateEditorFile[], index: number): TemplateEditorFile[] {
    return files.filter((_, i) => i !== index)
}
