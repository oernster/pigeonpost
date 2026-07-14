import type {Account} from './api'
import {TAG_PALETTE} from './tagColours'

// UNIFIED_FOLDER_ID is the synthetic folder id for the unified mailbox: every account's inbox merged
// into one newest-first list. Like the Outbox it is not a real server folder; the api module routes the
// listing, paging and sync calls made against it to the unified backend endpoints, so the folder-driven
// hooks (pagination, reload, the background poll) work on it unchanged.
export const UNIFIED_FOLDER_ID = '__unified__'

// isUnifiedFolder reports whether a folder id is the synthetic unified mailbox.
export function isUnifiedFolder(folderId: string): boolean {
    return folderId === UNIFIED_FOLDER_ID
}

// AccountChip labels a unified-list row with its owning account: a stable colour and the account's
// email for the tooltip.
export interface AccountChip {
    colour: string
    label: string
}

// accountChips assigns each account a colour from the tag palette by its sidebar position, so the
// unified list's per-account dots stay stable while the account order does. Keyed by account id.
export function accountChips(accounts: Account[]): Map<string, AccountChip> {
    const chips = new Map<string, AccountChip>()
    accounts.forEach((account, index) => {
        chips.set(account.id, {
            colour: TAG_PALETTE[index % TAG_PALETTE.length].colour,
            label: account.email,
        })
    })
    return chips
}
