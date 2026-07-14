// The Snoozed view's frontend seam: the synthetic folder id the api module routes on, and the label a
// hidden row shows for when it comes back.

// SNOOZED_FOLDER_ID is the synthetic folder id for the Snoozed view: every hidden message across all
// accounts with its due time. Like the Outbox and the unified mailbox it is not a real server folder;
// the api module routes its listing and sync calls, so the folder-driven hooks work on it unchanged.
export const SNOOZED_FOLDER_ID = '__snoozed__'

// isSnoozedFolder reports whether a folder id is the synthetic Snoozed view.
export function isSnoozedFolder(folderId: string): boolean {
    return folderId === SNOOZED_FOLDER_ID
}

// snoozedUntilLabel is the row's due-time label, e.g. "Until 15 Jul, 09:00". Empty when the message
// carries no snooze (every row outside the Snoozed view).
export function snoozedUntilLabel(snoozedUntilMs: number): string {
    if (snoozedUntilMs <= 0) {
        return ''
    }
    const due = new Date(snoozedUntilMs)
    if (isNaN(due.getTime())) {
        return ''
    }
    return 'Until ' + due.toLocaleString(undefined, {
        month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
    })
}
