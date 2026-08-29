/// <reference types="vite/client" />
import {describe, expect, it} from 'vitest'

// The module-size limit, enforced for the front end the way boundary_test.go enforces it for Go. The
// front end had no such guard, so while nearly every module stayed small a handful grew well past the
// limit with nothing to say so. A rule no test holds is a preference.
//
// EXEMPT names the files already over the limit when the guard was added. The list may only shrink:
// a file is removed from it when it is split; a file that is exempt while no longer over the limit
// fails too, so an entry cannot outlive the debt it records. Nothing new may join it. The open item in
// TECH_DEBT.md tracks the split.

// MAX_LINES is the module-size limit. DANGER_MIN derives the band beneath it rather than restating a
// second number, so the two cannot drift: a file that creeps into the band is one edit from breaking
// the limit, so it is reduced properly instead of shaved back under.
const MAX_LINES = 400
const DANGER_BAND_FRACTION = 20
const DANGER_MIN = MAX_LINES - MAX_LINES / DANGER_BAND_FRACTION
const DANGER_TARGET = 350

// Files over the limit when this guard was written, with the length each held at that point.
const EXEMPT: Record<string, number> = {
    'src/App.tsx': 1395,
    'src/components/ComposeModal.tsx': 731,
    'src/api.ts': 631,
    'src/components/EventFormModal.tsx': 559,
    'src/components/ContactsModal.tsx': 491,
    'src/components/FolderTree.tsx': 453,
    'src/hooks/useMenus.ts': 450,
    'src/components/CalendarModal.tsx': 436,
    'src/components/MessageContextMenu.tsx': 407,
}

// Every source module, read at build time by Vite (no node:fs, so the frontend stays browser-only).
// Test files are outside the limit by design: a table-driven suite is one coherent set of cases and
// splitting it to satisfy a cap it is not held to would scatter them for nothing.
const sources = import.meta.glob('../**/*.{ts,tsx}', {query: '?raw', eager: true, import: 'default'}) as Record<string, string>

// lineCount counts lines the way wc and every editor do: a trailing newline ends the last line rather
// than starting an empty one, so a 631-line file does not read as 632.
function lineCount(source: string): number {
    const body = source.replace(/\r?\n$/, '')
    return body === '' ? 0 : body.split('\n').length
}

// modulePath turns a glob key (../components/Foo.tsx, relative to src/test) into the src-relative path
// the exemption list is written in.
function modulePath(key: string): string {
    return `src/${key.replace(/^\.\.\//, '')}`
}

const modules = Object.entries(sources)
    .map(([key, source]) => ({path: modulePath(key), lines: lineCount(source)}))
    .filter((m) => !m.path.includes('.test.'))
    .sort((a, b) => a.path.localeCompare(b.path))

describe('module size', () => {
    it('found the source tree', () => {
        // Guards against a rename or a moved glob turning every assertion below into a vacuous pass.
        expect(modules.length).toBeGreaterThan(100)
    })

    for (const {path, lines} of modules) {
        const exempt = path in EXEMPT

        it(`${path} is within the limit`, () => {
            if (exempt) {
                expect(lines).toBeGreaterThan(MAX_LINES)
                return
            }
            expect(lines).toBeLessThanOrEqual(MAX_LINES)
        })

        it(`${path} is clear of the danger band`, () => {
            if (exempt || lines <= DANGER_MIN) {
                return
            }
            // In the band and not exempt: reduce it properly rather than shaving it under the limit,
            // or the next edit breaks it again and the same file is refactored twice.
            expect(lines).toBeLessThanOrEqual(DANGER_TARGET)
        })
    }

    it('exempts nothing that is not in the source tree', () => {
        const present = new Set(modules.map((m) => m.path))
        for (const path of Object.keys(EXEMPT)) {
            expect(present.has(path)).toBe(true)
        }
    })

    it('records each exempt file as no longer than it was when the guard was written', () => {
        // The list may only shrink. A file that grew past its recorded length is drifting further from
        // the limit rather than towards it.
        for (const {path, lines} of modules) {
            if (path in EXEMPT) {
                expect(lines).toBeLessThanOrEqual(EXEMPT[path])
            }
        }
    })
})
