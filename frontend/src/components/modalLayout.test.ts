import {describe, expect, it} from 'vitest'

// Every modal that carries an action row pins it: the row sits outside the scrolling body, so the
// buttons stay where the reader expects them however tall the dialog grows. Before this was
// enforced, eighteen dialogs scrolled as one block and took their buttons off the bottom of a short
// window. That is invisible on a large screen, which is exactly why a test holds it rather than
// review. The rule is checked against the source text, so a new dialog fails here on the day it is
// written instead of on someone's laptop.
//
// Raw source via Vite's glob rather than node:fs, matching test/boundary.test.ts: the frontend
// typecheck stays browser-only.
const sources = import.meta.glob('./*.tsx', {query: '?raw', eager: true, import: 'default'}) as Record<string, string>
const appSource = import.meta.glob('../App.tsx', {query: '?raw', eager: true, import: 'default'}) as Record<string, string>

const allSources: Record<string, string> = {...sources, ...appSource}

// A dialog with no action row has nothing to pin. Both of these are viewers whose own content is
// the scroller, and neither carries buttons at the foot.
const NO_ACTION_ROW = new Set(['modal message-popout', 'modal email-viewer'])

// The licence viewer pins its actions but scrolls `.licence-text` instead of a `.modal-body`,
// because the licence needs to be its own scroller (it carries a max-height of its own).
const SCROLLS_SOMETHING_ELSE = new Set(['modal licence pinned-actions'])

interface Panel {
    file: string
    classes: string
}

function modalPanels(): Panel[] {
    const found: Panel[] = []
    for (const [file, source] of Object.entries(allSources)) {
        // The dialog panel itself: a className whose first class is exactly `modal`.
        const pattern = /className="(modal(?:\s[^"]*)?)"/g
        for (const match of source.matchAll(pattern)) {
            found.push({file, classes: match[1]})
        }
    }
    return found
}

describe('modal layout', () => {
    it('finds the dialog panels it claims to check', () => {
        // Guards the regex itself: a rename that stopped matching would otherwise turn every
        // assertion below into a vacuous pass over an empty list.
        const panels = modalPanels()
        expect(panels.length).toBeGreaterThan(15)
        expect(panels.some((p) => p.classes.includes('pinned-actions'))).toBe(true)
    })

    it('pins the action row on every dialog that has one', () => {
        const unpinned = modalPanels()
            .filter((p) => !NO_ACTION_ROW.has(p.classes))
            .filter((p) => !p.classes.includes('pinned-actions'))
            .map((p) => `${p.file}: className="${p.classes}"`)

        expect(unpinned).toEqual([])
    })

    it('gives every pinned dialog something that actually scrolls', () => {
        // `pinned-actions` makes the panel `overflow: hidden`, so without an inner scroller a tall
        // dialog would clip its own content rather than scroll it. That is worse than the bug this
        // layout fixes, so the scroller is required rather than assumed.
        const missing = Object.entries(allSources)
            .filter(([, source]) => source.includes('pinned-actions'))
            .filter(([, source]) => !source.includes('modal-body') && !source.includes('licence-text'))
            .map(([file]) => file)

        expect(missing).toEqual([])
    })

    it('keeps the licence viewer as the only dialog scrolling something other than a body', () => {
        const oddities = modalPanels()
            .filter((p) => p.classes.includes('pinned-actions'))
            .filter((p) => !allSources[p.file].includes('modal-body'))
            .map((p) => p.classes)

        expect(oddities).toEqual([...SCROLLS_SOMETHING_ELSE])
    })
})
