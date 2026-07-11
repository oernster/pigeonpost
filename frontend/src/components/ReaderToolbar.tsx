import type {RefObject} from 'react'
import type {Folder, Message, Tag} from '../api'
import {TagColourMenu} from './TagColourMenu'

interface ReaderToolbarProps {
    message: Message
    // outbox switches the toolbar to a single Cancel send action for a queued message.
    outbox: boolean
    onBack?: () => void
    // backButtonRef is owned by the reader (shared with its focus sink) and attached to the Back button here.
    backButtonRef: RefObject<HTMLButtonElement>
    onReply: (message: Message) => void
    onReplyAll: (message: Message) => void
    onForward: (message: Message) => void
    onToggleRead: (message: Message) => void
    onDelete: (message: Message) => void
    onCancelSend: (message: Message) => void
    folders: Folder[]
    canMoveCopy: boolean
    onMove: (message: Message, destFolderId: string) => void
    onCopy: (message: Message, destFolderId: string) => void
    messageTags: Tag[]
    onToggleTag: (tagId: string, assigned: boolean) => void
}

// ReaderToolbar is the row of actions above a message: reply, reply all, forward, the read toggle, delete
// and, when the account supports it, the move and copy destinations plus the colour-tag control. A queued
// outbox message shows only Cancel send. The optional Back button (full-width reader) carries the reader's
// shared focus ref.
export function ReaderToolbar({
    message, outbox, onBack, backButtonRef, onReply, onReplyAll, onForward, onToggleRead, onDelete,
    onCancelSend, folders, canMoveCopy, onMove, onCopy, messageTags, onToggleTag,
}: ReaderToolbarProps) {
    return (
        <div className="reader-toolbar">
            {onBack && <button ref={backButtonRef} className="btn" onClick={onBack}>&#8592; Back</button>}
            {outbox ? (
                <button className="btn danger-outline" onClick={() => onCancelSend(message)}>
                    Cancel send
                </button>
            ) : (
                <>
                    <button className="btn" onClick={() => onReply(message)}>Reply</button>
                    {((message.to?.length || 0) + (message.cc?.length || 0)) > 0 && (
                        <button className="btn" onClick={() => onReplyAll(message)}>Reply all</button>
                    )}
                    <button className="btn" onClick={() => onForward(message)}>Forward</button>
                    <button className="btn" onClick={() => onToggleRead(message)}>
                        {message.read ? 'Mark as unread' : 'Mark as read'}
                    </button>
                    <button className="btn danger-outline" onClick={() => onDelete(message)}>Delete</button>
                    {canMoveCopy && folders.filter((f) => f.id !== message.folderId).length > 0 && (
                        <>
                            <select
                                className="move-select"
                                value=""
                                aria-label="Move to folder"
                                onChange={(e) => {
                                    if (e.target.value) {
                                        onMove(message, e.target.value)
                                    }
                                }}
                            >
                                <option value="">Move to…</option>
                                {folders.filter((f) => f.id !== message.folderId).map((f) => (
                                    <option key={f.id} value={f.id}>{f.name}</option>
                                ))}
                            </select>
                            <select
                                className="move-select"
                                value=""
                                aria-label="Copy to folder"
                                onChange={(e) => {
                                    if (e.target.value) {
                                        onCopy(message, e.target.value)
                                    }
                                }}
                            >
                                <option value="">Copy to…</option>
                                {folders.filter((f) => f.id !== message.folderId).map((f) => (
                                    <option key={f.id} value={f.id}>{f.name}</option>
                                ))}
                            </select>
                        </>
                    )}
                    <TagColourMenu
                        messageId={message.id}
                        messageTags={messageTags}
                        onToggleTag={onToggleTag}
                    />
                </>
            )}
        </div>
    )
}
