// toolbarNav models the arrow-key movement of a roving-tabindex toolbar (the WAI-ARIA toolbar
// pattern): the whole strip is one tab stop, Left and Right move between tools wrapping at the
// ends and Home and End jump to the first and last tool. stepTool returns the next tool index
// for a key, or null when the key is not a toolbar navigation key so the caller leaves it alone.
export function stepTool(current: number, key: string, count: number): number | null {
    if (count <= 0) {
        return null
    }
    switch (key) {
        case 'ArrowRight':
            return (current + 1) % count
        case 'ArrowLeft':
            return (current - 1 + count) % count
        case 'Home':
            return 0
        case 'End':
            return count - 1
        default:
            return null
    }
}
