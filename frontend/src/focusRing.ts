// focusRing holds the DOM focus-ring traversal used by the keyboard navigation: which container the ring
// is scoped to, the visible tabbable elements within it, stepping the ring and trapping Tab inside a
// dialog. It touches the DOM (document, getComputedStyle) but no React and no api, so it lives outside the
// components. It is verified by the running app rather than a headless test: jsdom has no layout engine, so
// the getClientRects-based visibility filter cannot be driven meaningfully, the same reason the Win32 paths
// on the Go side are documented coverage exclusions. It is therefore not on the pure-module coverage gate.

// focusRingRoot is the container the ring is scoped to: the topmost open modal when one is showing (so
// focus stays trapped within the dialog), otherwise the whole document.
export function focusRingRoot(): Document | HTMLElement {
    const modals = document.querySelectorAll<HTMLElement>('.modal')
    if (modals.length > 0) {
        return modals[modals.length - 1]
    }
    // The full-width reader (reading pane off, a message opened) is a self-contained screen reached by Back,
    // so the ring stays inside it and wraps there rather than escaping to the menus.
    const reader = document.querySelector<HTMLElement>('.reader-scoped')
    return reader ?? document
}

// focusRingElements returns the visible, tabbable elements in document order within root: the same set
// the browser steps with Tab. The roving-tabindex message list contributes a single stop (its
// non-current rows are tabindex -1); the folder and account lists add the selected row's own action
// buttons after the row, so stepping this ring jumps region to region.
export function focusRingElements(root: ParentNode): HTMLElement[] {
    const selector = [
        'a[href]', 'button:not([disabled])', 'input:not([disabled])',
        'select:not([disabled])', 'textarea:not([disabled])', '[tabindex]:not([tabindex="-1"])',
    ].join(',')
    return Array.from(root.querySelectorAll<HTMLElement>(selector)).filter((el) => {
        if (el.tabIndex < 0 || el.hasAttribute('disabled')) {
            return false
        }
        // Match the browser's own tabbability: skip hidden and unlaid-out elements.
        if (el.getClientRects().length === 0 || getComputedStyle(el).visibility === 'hidden') {
            return false
        }
        // Collapse the message list to a single stop: the row is the stop and its nested star button is
        // skipped, because Up/Down move within the list. The folder and account lists are left uncollapsed,
        // so the selected row is followed in the ring by its own action buttons (a folder's rename and
        // delete; an account's move up, move down, edit and remove) before the ring carries on. Non-selected
        // rows and their buttons are tabindex -1, so only the selected row contributes its buttons.
        const row = el.closest('.message-row')
        return !row || row === el
    })
}

// stepFocusRing moves focus forward (1) or back (-1) through the focus ring, wrapping at the ends, so
// Right/Left mirror Tab/Shift+Tab.
export function stepFocusRing(direction: 1 | -1) {
    const items = focusRingElements(focusRingRoot())
    if (items.length === 0) {
        return
    }
    const index = items.indexOf(document.activeElement as HTMLElement)
    const next = index === -1
        ? (direction === 1 ? 0 : items.length - 1)
        : (index + direction + items.length) % items.length
    items[next]?.focus()
}

// trapTab keeps Tab and Shift+Tab inside the open dialog: it wraps at the first and last elements and
// pulls focus back in if it has somehow landed outside, while letting native Tab move between elements
// in the middle (so a rich-text editor keeps its own Tab handling).
export function trapTab(e: KeyboardEvent) {
    const root = focusRingRoot()
    const items = focusRingElements(root)
    if (items.length === 0) {
        return
    }
    const first = items[0]
    const last = items[items.length - 1]
    const active = document.activeElement as HTMLElement | null
    if (e.shiftKey && active === first) {
        e.preventDefault()
        last.focus()
    } else if (!e.shiftKey && active === last) {
        e.preventDefault()
        first.focus()
    } else if (!active || !root.contains(active)) {
        e.preventDefault()
        first.focus()
    }
}
