/// <reference types="vite/client" />
import {describe, expect, it} from 'vitest'

// The pure logic modules are the frontend's domain layer. Each must stay free of the UI framework
// (React) and the Wails infrastructure seam (the api object, the generated bindings), so it is
// unit-testable in isolation and the 100% coverage gate on it is meaningful. This list grows as
// each god module's pure slice is extracted. It is the React analogue of the Go boundary_test.go:
// purity is enforced by a test, not by convention.
//
// Note on the api seam: Message, Folder, OutboxItem and the rest are TYPES, erased at build, so
// importing them (import {Message} from './api') carries no runtime coupling. The runtime exports
// of ./api are the `api` object and the `EventScope` enum; a pure module must import neither.
const PURE_MODULES = [
    'tz', 'folderPaths', 'threads', 'outbox', 'tagColours',
    'messageText', 'shortcuts', 'print',
    'readerFormat', 'composeAddresses', 'accountProviders', 'sidebarDnd',
    'calendarModel', 'replyDraft', 'categories',
]

// The raw source of every src/*.ts, read at build time by Vite (no node:fs, so the frontend
// typecheck stays browser-only). Keyed by path; each pure module is looked up by name.
const sources = import.meta.glob('../*.ts', {query: '?raw', eager: true, import: 'default'}) as Record<string, string>

interface ImportInfo {
    clause: string
    specifier: string
}

function importStatements(source: string): ImportInfo[] {
    const re = /import\s+([\s\S]*?)\s+from\s*['"]([^'"]+)['"]/g
    const out: ImportInfo[] = []
    for (let m = re.exec(source); m !== null; m = re.exec(source)) {
        out.push({clause: m[1], specifier: m[2]})
    }
    return out
}

function sourceOf(name: string): string {
    const key = Object.keys(sources).find((path) => path.endsWith(`/${name}.ts`))
    if (key === undefined) {
        throw new Error(`pure module ${name}.ts was not found`)
    }
    return sources[key]
}

describe('pure module boundaries', () => {
    for (const name of PURE_MODULES) {
        describe(name, () => {
            const imports = importStatements(sourceOf(name))

            it('does not import React', () => {
                expect(imports.filter((i) => i.specifier === 'react' || i.specifier === 'react-dom')).toEqual([])
            })

            it('does not import the Wails bindings', () => {
                expect(imports.filter((i) => i.specifier.includes('wailsjs'))).toEqual([])
            })

            it('imports only types from the api seam, never a runtime value', () => {
                const apiImports = imports.filter((i) => /(^|\/)api$/.test(i.specifier))
                const runtime = apiImports.filter((i) => /\bapi\b/.test(i.clause) || /\bEventScope\b/.test(i.clause))
                expect(runtime).toEqual([])
            })
        })
    }
})
