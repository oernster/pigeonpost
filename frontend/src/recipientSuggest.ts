// recipientSuggest holds the pure logic behind the To / Cc / Bcc contact autocomplete: flattening
// the address book into suggestion entries, finding the address fragment being typed, matching and
// ranking suggestions against it and splicing an accepted suggestion back into the field. The field
// stays an ordinary text input throughout: a suggestion only ever inserts text, so the user can edit
// or replace it freely afterwards. No React, no api runtime, so it is unit-tested in isolation.

import {splitAddresses} from './composeAddresses'

// MAX_SUGGESTIONS caps the dropdown at a scan-friendly height.
export const MAX_SUGGESTIONS = 8

// Suggestion is one offerable address: the bare address that insertion uses, the contact's display
// name ('' when the contact has none) and the label the dropdown row shows.
export interface Suggestion {
    address: string
    name: string
    display: string
}

// SuggestibleContact is the slice of a contact the pool reads, so tests can hand in plain objects.
export interface SuggestibleContact {
    formattedName: string
    givenName: string
    familyName: string
    emails: ReadonlyArray<{address: string}>
}

// contactName resolves a contact's display name: the formatted name when present, otherwise the
// given and family names joined, otherwise empty.
function contactName(contact: SuggestibleContact): string {
    const formatted = contact.formattedName.trim()
    if (formatted !== '') {
        return formatted
    }
    return [contact.givenName.trim(), contact.familyName.trim()].filter(Boolean).join(' ')
}

// suggestionPool flattens the address book into suggestion entries, one per distinct email address
// (compared case-insensitively; the first holder of an address wins, so a duplicate in a second
// contact never doubles the row).
export function suggestionPool(contacts: readonly SuggestibleContact[]): Suggestion[] {
    const seen = new Set<string>()
    const pool: Suggestion[] = []
    for (const contact of contacts) {
        const name = contactName(contact)
        for (const email of contact.emails) {
            const address = email.address.trim()
            const key = address.toLowerCase()
            if (address === '' || seen.has(key)) {
                continue
            }
            seen.add(key)
            pool.push({address, name, display: name === '' ? address : `${name} <${address}>`})
        }
    }
    return pool
}

// activeFragment returns the address fragment being typed: the text after the last comma or
// semicolon with leading whitespace skipped, and the index it starts at (0 for a single-address
// field). applySuggestion splices at the same index, so the two always agree.
export function activeFragment(value: string): {start: number; text: string} {
    const lastSeparator = Math.max(value.lastIndexOf(','), value.lastIndexOf(';'))
    let start = lastSeparator + 1
    while (start < value.length && value[start] === ' ') {
        start += 1
    }
    return {start, text: value.slice(start)}
}

// suggestionsFor matches the pool against the fragment being typed: an empty fragment offers
// nothing (the dropdown appears only once typing starts) and an address already listed in the field
// is never re-offered. A match anywhere in the name or address qualifies; matches at the start of
// either rank first, alphabetically by display within each rank.
export function suggestionsFor(pool: readonly Suggestion[], value: string, limit: number = MAX_SUGGESTIONS): Suggestion[] {
    const fragment = activeFragment(value).text.trim().toLowerCase()
    if (fragment === '') {
        return []
    }
    const listed = new Set(splitAddresses(value).map((a) => a.toLowerCase()))
    const ranked: {rank: number; suggestion: Suggestion}[] = []
    for (const suggestion of pool) {
        const address = suggestion.address.toLowerCase()
        if (listed.has(address)) {
            continue
        }
        const name = suggestion.name.toLowerCase()
        const atStart = address.startsWith(fragment) || name.startsWith(fragment)
        const anywhere = atStart || address.includes(fragment) || name.includes(fragment)
        if (!anywhere) {
            continue
        }
        ranked.push({rank: atStart ? 0 : 1, suggestion})
    }
    ranked.sort((a, b) => a.rank - b.rank || a.suggestion.display.localeCompare(b.suggestion.display))
    return ranked.slice(0, limit).map((r) => r.suggestion)
}

// applySuggestion replaces the fragment being typed with the accepted address, keeping everything
// before it and adding the space a separator-tight prefix ("a@b.com,") is missing. Only text is
// inserted, so the result remains freely editable.
export function applySuggestion(value: string, suggestion: Suggestion): string {
    const {start} = activeFragment(value)
    let prefix = value.slice(0, start)
    if (prefix !== '' && !prefix.endsWith(' ')) {
        prefix += ' '
    }
    return prefix + suggestion.address
}
