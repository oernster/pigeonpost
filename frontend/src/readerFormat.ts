// readerFormat holds the pure display helpers for the reader: correspondent formatting, attachment
// sizes and legible tag-chip ink. No React, no api, so each is unit-tested in isolation.

// formatAddress renders one correspondent as "Name <address>", or just the address when it has no name.
export function formatAddress(a: {name: string; address: string}): string {
    return a.name ? `${a.name} <${a.address}>` : a.address
}

// formatAddressList joins a recipient list for display, dropping any empty entries.
export function formatAddressList(list: {name: string; address: string}[]): string {
    return list.map(formatAddress).filter(Boolean).join(', ')
}

// formatBytes renders an attachment size in the largest unit that keeps the number readable.
export function formatBytes(bytes: number): string {
    const kib = 1024
    if (bytes < kib) {
        return `${bytes} B`
    }
    const units = ['KB', 'MB', 'GB']
    let value = bytes / kib
    let unit = 0
    while (value >= kib && unit < units.length - 1) {
        value /= kib
        unit += 1
    }
    return `${value.toFixed(1)} ${units[unit]}`
}

// readableInk picks black or white text for a #rrggbb background using its perceived luminance, so a
// tag chip's label stays legible on any colour.
export function readableInk(hex: string): string {
    const value = hex.replace('#', '')
    if (value.length !== 6) {
        return '#000000'
    }
    const r = parseInt(value.slice(0, 2), 16)
    const g = parseInt(value.slice(2, 4), 16)
    const b = parseInt(value.slice(4, 6), 16)
    const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255
    return luminance > 0.6 ? '#000000' : '#ffffff'
}
