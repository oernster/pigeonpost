// Vitest setup: register the jest-dom matchers (toBeInTheDocument, toHaveTextContent and the
// rest) on Vitest's expect, so component tests added later can assert against the rendered DOM.
import '@testing-library/jest-dom/vitest'

// Node 25 and later ship a Web Storage of their own: localStorage and sessionStorage are getters on the
// global object that answer undefined without --localstorage-file. jsdom's window is that same
// global object, so the getters shadow jsdom's working storage and every test touching it failed
// (measured on Node 26.13.0: 203 failures reading getItem/clear of undefined). Vitest keeps the jsdom
// instance on the jsdom global; pointing both names at its storage gives every test the browser's
// storage on any Node version, with no flag for whoever runs the suite to remember.
const jsdomWindow = (globalThis as {jsdom?: {window: Window}}).jsdom?.window
if (jsdomWindow) {
    for (const name of ['localStorage', 'sessionStorage'] as const) {
        Object.defineProperty(globalThis, name, {configurable: true, enumerable: true, value: jsdomWindow[name]})
    }
}

// The message list is virtualised with @tanstack/react-virtual, which sizes the scroll viewport and each
// row from offsetHeight (see virtual-core's getRect and measureElement) and scrolls the container with
// scrollTo. jsdom runs no layout, so offsetHeight is 0 (which would collapse the virtual window and render
// no rows) and scrollTo is absent. Give the scroll container a tall viewport and every measured row a fixed
// height so the whole test list renders (production uses the real WebView layout) and stub scrollTo so
// scrollToIndex is a no-op. Only the message list's own elements are affected: every other element keeps
// jsdom's 0, so nothing else (the folder-drop hit tests, for one) changes.
const VIRTUAL_VIEWPORT_HEIGHT = 100000
const VIRTUAL_ROW_HEIGHT = 40
Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
    configurable: true,
    get(this: HTMLElement): number {
        if (this.classList?.contains('message-list-scroll')) {
            return VIRTUAL_VIEWPORT_HEIGHT
        }
        if (this.hasAttribute?.('data-index')) {
            return VIRTUAL_ROW_HEIGHT
        }
        return 0
    },
})
if (typeof Element.prototype.scrollTo !== 'function') {
    Element.prototype.scrollTo = () => undefined
}

// ProseMirror scrolls its caret into view after a transaction, which measures a Range (see singleRect
// in prosemirror-view). jsdom runs no layout and implements neither Range.getClientRects nor
// Range.getBoundingClientRect, so that measurement throws. It happens in the editor's own scroll pass
// rather than under a test's await, so the throw escapes as an unhandled error and fails the whole run
// instead of one test, intermittently, depending on whether it lands before the run ends. Give a Range
// the empty measurements a document with no layout honestly has: singleRect then finds no rects, falls
// back to the bounding box and scrolls nowhere, which is what jsdom can support.
const EMPTY_RECT: DOMRect = {
    x: 0, y: 0, width: 0, height: 0, top: 0, right: 0, bottom: 0, left: 0, toJSON: () => ({}),
}
if (typeof Range.prototype.getClientRects !== 'function') {
    Range.prototype.getClientRects = function (): DOMRectList {
        const rects: DOMRect[] = []
        return Object.assign(rects, {item: () => null}) as unknown as DOMRectList
    }
}
if (typeof Range.prototype.getBoundingClientRect !== 'function') {
    Range.prototype.getBoundingClientRect = (): DOMRect => EMPTY_RECT
}
