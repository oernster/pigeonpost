// The sending half of the Wails seam: the compose and draft-recovery types and the calls that send, save
// drafts, keep the recovery snapshot, pick attachments and work the outbox. It lives beside api.ts rather
// than inside it for the reason the filter rules do (see apiRules): a cohesive group of calls with its own
// types, taken out of a module that was over the size limit. The api object spreads what is exported
// here, so callers still reach these through api.* and nothing else changes.
import {
    CancelOutboxItem,
    ClearDraftRecovery,
    DraftRecovery,
    ListOutbox,
    OutboxCount,
    PickAttachments,
    ReplayOutbox,
    SaveDraft,
    SaveDraftRecovery,
    SendMessage,
} from '../wailsjs/go/main/App'
import {main} from '../wailsjs/go/models'

export type OutboxItem = main.OutboxItemDTO

export interface ComposeInput {
    accountId: string
    // from is the chosen sender address: empty means the account's primary address, otherwise it must be
    // one of the account's identities. The backend validates it.
    from: string
    to: string[]
    cc: string[]
    bcc: string[]
    subject: string
    body: string
    htmlBody: string
    attachmentPaths: string[]
    // attachmentData carries files pasted or dropped into the compose window, where the webview holds
    // name and bytes but no filesystem path. content is base64 (AttachmentDataEntry in send.go).
    attachmentData: {name: string; contentType: string; content: string}[]
    attachmentMessageIds: string[]
    // sendAtMs is send-later: a Unix-millisecond instant queues the send held until then (cancellable
    // from the Outbox; send returns the queued id). Zero means no schedule; send returns ''.
    sendAtMs: number
}

// DraftRecoveryInput is a local snapshot of the compose window, autosaved for crash and
// accidental-close recovery. The recipient fields are the raw text as typed, not parsed lists.
export interface DraftRecoveryInput {
    accountId: string
    to: string
    cc: string
    bcc: string
    subject: string
    bodyHtml: string
}

// DraftRecoveryResult is the stored compose snapshot. present is false when none is held, in which case
// the other fields are empty and there is nothing to restore.
export interface DraftRecoveryResult {
    present: boolean
    accountId: string
    to: string
    cc: string
    bcc: string
    subject: string
    bodyHtml: string
    savedMs: number
}

export const composeApi = {
    send: (req: ComposeInput): Promise<string> => SendMessage(main.ComposeRequest.createFrom(req)),
    saveDraft: (req: ComposeInput): Promise<void> => SaveDraft(main.ComposeRequest.createFrom(req)),
    saveDraftRecovery: (req: DraftRecoveryInput): Promise<void> =>
        SaveDraftRecovery(main.DraftRecoveryRequest.createFrom(req)),
    draftRecovery: (): Promise<DraftRecoveryResult> => DraftRecovery(),
    clearDraftRecovery: (): Promise<void> => ClearDraftRecovery(),
    outboxCount: (): Promise<number> => OutboxCount(),
    listOutbox: (): Promise<OutboxItem[]> => ListOutbox(),
    // cancelOutboxItem resolves true when the item was still queued and is now stopped; false means the
    // message had already been sent, so an undo that lost the race can say so.
    cancelOutboxItem: (id: string): Promise<boolean> => CancelOutboxItem(id),
    // A cancelled file dialog returns a Go nil slice, which arrives as null; coalesce it to an empty array
    // so callers can always read .length and filter it.
    pickAttachments: async (): Promise<string[]> => (await PickAttachments()) ?? [],
    replayOutbox: (): Promise<number> => ReplayOutbox(),
}
