// shortcuts holds the pure accelerator matcher. No React, no api runtime, so it is unit-tested in
// isolation.

// matchesShortcut reports whether a keyboard event is the accelerator named by a shortcut string such as
// "Ctrl+N", "F9" or "Ctrl+Shift+K". Ctrl matches the Cmd key too, so the same strings work on macOS.
export function matchesShortcut(e: KeyboardEvent, shortcut: string): boolean {
    const parts = shortcut.toLowerCase().split('+').map((part) => part.trim())
    const key = parts[parts.length - 1]
    return parts.includes('ctrl') === (e.ctrlKey || e.metaKey) &&
        parts.includes('shift') === e.shiftKey &&
        parts.includes('alt') === e.altKey &&
        e.key.toLowerCase() === key
}
