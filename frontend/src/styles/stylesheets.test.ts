import {describe, expect, it} from 'vitest'

// A structural test over the stylesheets themselves rather than over a rendered component. It exists
// because the app mark shipped with a rectangle drawn round it under the mouse: the mark borrows
// .icon-btn for its geometry and asks for a transparent border, which an ungated .icon-btn:hover then
// repainted on specificity. The comment beside the mark already claimed the hover rule was gated on
// :enabled, so the intent was written down and the code quietly disagreed with it. A comment cannot
// fail; this can.
//
// The invariant: a :hover rule on a class worn by a button must also require :enabled. That closes both
// halves of the same defect. A span borrowing a button class never matches :enabled, so it stays inert
// under the mouse; a disabled button does not light up as a control that cannot be pressed. The focus
// rules were already written this way throughout, which is what made the gap in the hover rules visible.

// Every class here is worn by a <button> somewhere in the app. It is stated rather than derived from the
// components, because a className is assembled at runtime from strings and conditions, so a derivation
// would be a guess about markup. The second test is what stops a rename emptying the list in silence:
// without it a list of names matching nothing would report a clean sweep of nothing.
const BUTTON_CLASSES = [
    'icon-btn',
    'btn',
    'menu-title',
    'provider-btn',
    'context-item',
    'compose-template-option',
]

const STYLESHEET_DIR = 'src/styles'
const ROOT_STYLESHEETS = ['src/style.css', 'src/App.css']

// loc.test.ts reads its sources through import.meta.glob to keep the front end free of node:fs, which is
// the better route where it works. It does not work here: measured under this vitest setup a glob of the
// stylesheets finds all sixteen files and hands back an empty string for every one, because vitest does
// not process CSS, so a raw import of a stylesheet has no text in it. Reading the files is therefore the
// only route that sees the rules at all; it is the one Sidebar.test.tsx already takes for the same
// reason. The indirection through a built name keeps the import out of the bundler's static analysis.
async function nodeFs() {
    const fsModule = 'node:' + 'fs'
    return (await import(fsModule)) as {
        readFileSync: (path: string, encoding: string) => string
        readdirSync: (path: string) => string[]
    }
}

async function stylesheets(): Promise<Array<{path: string; css: string}>> {
    const {readFileSync, readdirSync} = await nodeFs()
    const paths = readdirSync(STYLESHEET_DIR)
        .filter((name) => name.endsWith('.css'))
        .map((name) => `${STYLESHEET_DIR}/${name}`)
        .concat(ROOT_STYLESHEETS)
    return paths.map((path) => ({path, css: readFileSync(path, 'utf8')}))
}

// Comments are dropped before anything is read as a selector. Several of them quote a selector verbatim
// while explaining a decision; a comment quoting an ungated rule is prose rather than a rule.
function withoutComments(css: string): string {
    return css.replace(/\/\*[\s\S]*?\*\//g, '')
}

// A block's selector list is the run of text before its opening brace. Each entry in that list is judged
// on its own, since one ungated selector among several is still an ungated rule.
function selectors(css: string): string[] {
    return Array.from(withoutComments(css).matchAll(/([^{}]+)\{/g))
        .flatMap((match) => match[1].split(','))
        .map((one) => one.trim())
        .filter((one) => one.length > 0)
}

// The negative lookahead is load-bearing: .icon-btn-image begins with .icon-btn and is a different class.
// The pattern is built from an ordinary string rather than a template literal on purpose: a template
// literal consumes the backslashes itself, which left the dot matching any character and reported
// .list-sort-btn as an offender it never was.
function mentions(text: string, cls: string): boolean {
    return new RegExp('\\.' + cls + '(?![\\w-])').test(text)
}

describe('the stylesheets', () => {
    it('gate every hover rule on a button class with :enabled', async () => {
        const offenders: string[] = []
        for (const sheet of await stylesheets()) {
            for (const selector of selectors(sheet.css)) {
                if (!selector.includes(':hover') || selector.includes(':enabled')) continue
                if (BUTTON_CLASSES.some((cls) => mentions(selector, cls))) {
                    offenders.push(`${sheet.path}: ${selector}`)
                }
            }
        }
        expect(offenders).toEqual([])
    })

    it('still hold every button class the guard claims to cover', async () => {
        const all = (await stylesheets()).map((sheet) => sheet.css).join('\n')
        expect(BUTTON_CLASSES.filter((cls) => !mentions(all, cls))).toEqual([])
    })

    // The invariant a user actually sees. The gating test above would still pass with this declaration
    // deleted, because the mark would then simply inherit the border it is meant to refuse.
    it('keep the app mark asking for no border', async () => {
        const {readFileSync} = await nodeFs()
        const css = withoutComments(readFileSync(`${STYLESHEET_DIR}/titlebar-and-menus.css`, 'utf8'))
        const block = css.slice(css.indexOf('.titlebar-mark {')).split('}')[0]
        expect(block).toMatch(/border-color:\s*transparent;/)
        expect(block).toMatch(/background-color:\s*transparent;/)
    })
})
