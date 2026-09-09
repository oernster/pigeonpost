import {describe, expect, it} from 'vitest'

// A structural test over the stylesheets themselves rather than over a rendered component. It exists
// because the app mark shipped with a rectangle drawn round it under the mouse: the mark borrowed
// .icon-btn for its geometry and asked for a transparent border, which an ungated .icon-btn:hover then
// repainted on specificity. The comment beside the mark already claimed the hover rule was gated on
// :enabled, so the intent was written down and the code quietly disagreed with it. A comment cannot
// fail; this can. The mark itself has since become the watermark behind the panes, whose own two
// declarations are held at the foot of this file for the same reason: neither is visible to jsdom.
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

    // The watermark's two load-bearing declarations. A negative z-index only stays inside the pane while
    // the pane is a stacking context; drop the isolation and the mark is painted behind the application's
    // own background instead, which is invisible rather than subtle. The pointer gate is the other half:
    // the pseudo-element covers the whole pane, so without it every click in the list would land on the
    // watermark rather than on the row under the cursor. Neither failure is one a rendered-component test
    // would catch, because jsdom computes no stacking and no hit testing.
    it('keep the pane watermark isolated and untouchable', async () => {
        const {readFileSync} = await nodeFs()
        const css = withoutComments(readFileSync(`${STYLESHEET_DIR}/base-and-panes.css`, 'utf8'))
        const panes = css.slice(css.indexOf('.pane.message-list,')).split('}')[0]
        expect(panes).toMatch(/isolation:\s*isolate;/)
        expect(panes).toMatch(/position:\s*relative;/)
        const mark = css.slice(css.indexOf('.pane.message-list::before,')).split('}')[0]
        expect(mark).toMatch(/z-index:\s*-1;/)
        expect(mark).toMatch(/pointer-events:\s*none;/)
        expect(mark).toMatch(/background-image:\s*url\('\.\.\/assets\/pigeonpost\.png'\);/)
    })

    // Both panes wear the same mark at the same size, so the size and the opacity are tokens rather than
    // numbers written out twice. Two literals here would drift the moment one pane was tuned alone.
    it('take the watermark size and opacity from the shared tokens', async () => {
        const {readFileSync} = await nodeFs()
        const css = withoutComments(readFileSync(`${STYLESHEET_DIR}/base-and-panes.css`, 'utf8'))
        const mark = css.slice(css.indexOf('.pane.message-list::before,')).split('}')[0]
        expect(mark).toMatch(/background-size:\s*var\(--watermark-size\);/)
        expect(mark).toMatch(/opacity:\s*var\(--watermark-opacity\);/)
        const root = withoutComments(readFileSync('src/style.css', 'utf8'))
        expect(root).toMatch(/--watermark-size:/)
        expect(root).toMatch(/--watermark-opacity:/)
    })

    // The bars and the sidebar's section labels start on one line, which --bar-ink-x names. Two things
    // decide it and neither is visible to a rendered test: both bars take their leading inset from the
    // token rather than a number of their own; the empty leading group is taken out of the flow. That
    // group holds the unread badge alone, so with nothing unread it was an empty box contributing a gap,
    // which put the first control eight pixels right of every label under it.
    it('start both bars and the section labels on the one leading line', async () => {
        const {readFileSync} = await nodeFs()
        const root = withoutComments(readFileSync('src/style.css', 'utf8'))
        expect(root).toMatch(/--bar-ink-x:/)
        const panes = withoutComments(readFileSync(`${STYLESHEET_DIR}/base-and-panes.css`, 'utf8'))
        const bar = panes.slice(panes.indexOf('.titlebar {')).split('}')[0]
        expect(bar).toMatch(/padding:\s*\d+px var\(--bar-ink-x\);/)
        const foot = panes.slice(panes.indexOf('.bottombar {')).split('}')[0]
        expect(foot).not.toMatch(/padding/)
        const label = panes.slice(panes.indexOf('.section-label {')).split('}')[0]
        expect(label).toMatch(/padding:\s*\d+px var\(--bar-ink-x\) \d+px;/)
        const bars = withoutComments(readFileSync(`${STYLESHEET_DIR}/titlebar-and-menus.css`, 'utf8'))
        const empty = bars.slice(bars.indexOf('.titlebar-left:empty {')).split('}')[0]
        expect(empty).toMatch(/display:\s*none;/)
    })

    // The left group must not shrink. It carried min-width: 0 so it would give way first in a narrow
    // window, which measured badly: the group shrank to 45px while the picture inside it stayed its own
    // 75px, so it slid under the button beside it and was painted over. The picture has since moved to the
    // panes; the declaration stays, because the badge that is left would give way the same way. Nothing
    // here computes layout, so what is held is the declaration that decides it.
    it('keep the leading title-bar group from shrinking', async () => {
        const {readFileSync} = await nodeFs()
        const css = withoutComments(readFileSync(`${STYLESHEET_DIR}/titlebar-and-menus.css`, 'utf8'))
        const block = css.slice(css.indexOf('.titlebar-left {')).split('}')[0]
        expect(block).toMatch(/flex-shrink:\s*0;/)
        expect(block).not.toMatch(/min-width:\s*0;/)
    })
})
