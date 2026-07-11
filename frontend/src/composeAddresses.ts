// composeAddresses holds the pure text helpers for the compose window: URL normalising for the link
// editor, recipient-list splitting and validation, wrong-separator correction and attachment basenames.
// No React, no api, so each is unit-tested in isolation.

// normaliseUrl gives a bare host a scheme so the link is absolute rather than treated as relative.
export function normaliseUrl(url: string): string {
    const trimmed = url.trim()
    if (trimmed === '' || /^(https?:|mailto:)/i.test(trimmed)) {
        return trimmed
    }
    return `https://${trimmed}`
}

export function splitAddresses(value: string): string[] {
    // Accept both comma and semicolon between addresses, so a list pasted from a client that uses ";"
    // (such as Outlook) is split the same as a comma-separated one.
    return value.split(/[,;]/).map((part) => part.trim()).filter(Boolean)
}

// EMAIL_TOKEN finds an address-like run anywhere in a string; EMAIL_EXACT tests that a whole string is
// one. Both stop at the punctuation a user might wrongly place between addresses (a colon, a slash, a space
// and so on), so a mistyped separator leaves the addresses on either side intact to be found and re-joined.
// Neither is a full RFC validator; the backend stays the source of truth.
const EMAIL_TOKEN = /[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}/g
const EMAIL_EXACT = /^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$/

export function isValidAddress(value: string): boolean {
    return EMAIL_EXACT.test(value)
}

// detectSeparatorFix returns a corrected version of one recipient field when its only problem is a wrong
// separator between two or more otherwise-valid addresses, re-joining the addresses it finds with "; ". It
// returns null when the field is already correctly separated or holds a single address (valid or not), so a
// genuine typo is never silently rewritten and is left for the backend to reject.
export function detectSeparatorFix(value: string): string | null {
    const found = value.match(EMAIL_TOKEN) ?? []
    if (found.length < 2) return null
    const tokens = splitAddresses(value)
    if (tokens.length === found.length && tokens.every(isValidAddress)) return null
    return found.join('; ')
}

// basename returns the final path segment of a file path, handling both Windows and POSIX separators,
// so an attachment chip shows the filename rather than the full path.
export function basename(path: string): string {
    const parts = path.split(/[\\/]/)
    return parts[parts.length - 1] || path
}
