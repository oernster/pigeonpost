// Unit tests for the pure half of editing a template's files. The contract that matters is the one
// with the backend: a stored file is carried over by NAMING its position, so a bug here does not
// look like a bug, it looks like a template quietly losing its attachments on the next save.
import {describe, expect, it} from 'vitest'
import {pickedFiles, removeFileAt, saveFiles, storedFiles, STORED_ELSEWHERE} from './templateFiles'
import type {TemplateAttachment} from './api'

const described = (filename: string, size: number) =>
    ({filename, contentType: 'application/pdf', size}) as TemplateAttachment

describe('storedFiles', () => {
    it('remembers each file by the position it occupies on the stored template', () => {
        const files = storedFiles([described('a.pdf', 10), described('b.pdf', 20)])
        expect(files.map((f) => [f.name, f.position, f.size])).toEqual([
            ['a.pdf', 0, 10],
            ['b.pdf', 1, 20],
        ])
        expect(files.every((f) => f.path === '')).toBe(true)
    })

    it('gives a template with no files an empty list', () => {
        expect(storedFiles([])).toEqual([])
    })
})

describe('pickedFiles', () => {
    it('names a chosen file by its base name and marks it as not yet stored', () => {
        const windowsPath = 'C:\\Users\\me\\terms.pdf'
        const picked = pickedFiles([windowsPath], [])
        expect(picked).toEqual([
            {position: STORED_ELSEWHERE, path: windowsPath, name: 'terms.pdf', size: 0},
        ])
    })

    it('skips a file already in the list, so the same file cannot be attached twice', () => {
        const existing = pickedFiles(['/home/me/terms.pdf'], [])
        expect(pickedFiles(['/home/me/terms.pdf', '/home/me/other.pdf'], existing).map((f) => f.name))
            .toEqual(['other.pdf'])
    })

    // A stored file has no path, so the dedupe must not treat two of them as the same file.
    it('does not confuse stored files with each other', () => {
        const stored = storedFiles([described('a.pdf', 1), described('b.pdf', 2)])
        expect(pickedFiles(['/home/me/c.pdf'], stored)).toHaveLength(1)
    })
})

describe('saveFiles', () => {
    it('states the kept positions in display order and the added paths after them', () => {
        const stored = storedFiles([described('a.pdf', 1), described('b.pdf', 2), described('c.pdf', 3)])
        // Drop b, reverse the two that are left, then add one.
        const edited = [stored[2], stored[0], ...pickedFiles(['/home/me/new.pdf'], [])]

        expect(saveFiles(edited)).toEqual({
            keepFilePositions: [2, 0],
            addFilePaths: ['/home/me/new.pdf'],
        })
    })

    it('says an empty template holds nothing rather than omitting the fields', () => {
        expect(saveFiles([])).toEqual({keepFilePositions: [], addFilePaths: []})
    })
})

describe('removeFileAt', () => {
    // Addressed by index rather than by name or path, since two files may share either.
    it('drops the entry at the index, leaving the rest in order', () => {
        const files = storedFiles([described('a.pdf', 1), described('a.pdf', 2), described('c.pdf', 3)])
        expect(removeFileAt(files, 0).map((f) => f.position)).toEqual([1, 2])
    })
})
