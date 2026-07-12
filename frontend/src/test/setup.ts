// Vitest setup: register the jest-dom matchers (toBeInTheDocument, toHaveTextContent and the
// rest) on Vitest's expect, so component tests added later can assert against the rendered DOM.
import '@testing-library/jest-dom/vitest'

// The message list is virtualized with @tanstack/react-virtual, which sizes the scroll viewport and each
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
