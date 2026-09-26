// The selection-wide half of the Wails seam: the calls that act on many messages at once. It lives
// beside api.ts rather than inside it because api.ts is one of the modules over the size limit that the
// guard in src/test/loc.test.ts is ratcheting down. The api object spreads what is exported here, so
// callers still reach these through api.* and nothing else changes.
import {
    DeleteMessages,
    DeleteMessagesPermanent,
    MarkReadMessages,
    MoveMessages,
    PushReadMessages,
} from '../wailsjs/go/main/App'
import {main} from '../wailsjs/go/models'

// BulkResult reports which ids a selection-wide call acted on, how many it could not and any error text.
export type BulkResult = main.BulkResultDTO

export const bulkApi = {
    // deleteMessages / deleteMessagesPermanent / moveMessages act on the whole selection in one batched
    // backend call (grouped by folder, one server connection per folder) rather than a round trip per
    // message, which is what keeps a large Gmail selection under its simultaneous-connection cap.
    deleteMessages: (ids: string[]): Promise<BulkResult> => DeleteMessages(ids),
    deleteMessagesPermanent: (ids: string[]): Promise<BulkResult> => DeleteMessagesPermanent(ids),
    moveMessages: (ids: string[], destFolderId: string): Promise<BulkResult> => MoveMessages(ids, destFolderId),
    // markReadMessages writes a selection's read state to the local cache in one step and returns at
    // once, so the unread badges can refresh from it straight away; pushReadMessages then lands the
    // change on the server, one connection per folder, best effort (a failure is replayed by the sync).
    markReadMessages: (ids: string[], read: boolean): Promise<BulkResult> => MarkReadMessages(ids, read),
    pushReadMessages: (ids: string[], read: boolean): Promise<void> => PushReadMessages(ids, read),
}
