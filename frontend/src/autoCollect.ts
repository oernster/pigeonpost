// autoCollect holds the pure logic behind adding outgoing recipients to the address book
// automatically: the persisted on/off flag's encoding and the recipient list worth collecting from
// a send. Storage access stays at the call sites (the contacts toggle writes, the composer reads),
// so this module is unit-tested in isolation.

import {splitAddresses} from './composeAddresses'

// AUTO_COLLECT_KEY is the localStorage key holding the setting; see shouldAutoCollect for its
// encoding.
export const AUTO_COLLECT_KEY = 'pigeonpost.autoCollectContacts'

// autoCollectDisabled is the single stored value meaning off: the feature is on by default, so an
// absent key (every existing install) and any other value mean on.
const autoCollectDisabled = '0'

// shouldAutoCollect decodes the stored setting: only the explicit off marker disables collection.
export function shouldAutoCollect(stored: string | null): boolean {
    return stored !== autoCollectDisabled
}

// autoCollectStored encodes the setting for storage.
export function autoCollectStored(enabled: boolean): string {
    return enabled ? '1' : autoCollectDisabled
}

// collectableRecipients gathers the addresses a send should offer for collection: every To, Cc and
// Bcc entry, deduplicated case-insensitively and with the sender's own addresses dropped (replying
// to yourself or a reply-all that includes you must not add you to your own address book). The
// backend validates each address and skips what is already a contact.
export function collectableRecipients(to: string, cc: string, bcc: string, ownAddresses: readonly string[]): string[] {
    const own = new Set(ownAddresses.map((a) => a.toLowerCase()))
    const seen = new Set<string>()
    const out: string[] = []
    for (const address of [...splitAddresses(to), ...splitAddresses(cc), ...splitAddresses(bcc)]) {
        const key = address.toLowerCase()
        if (own.has(key) || seen.has(key)) {
            continue
        }
        seen.add(key)
        out.push(address)
    }
    return out
}
