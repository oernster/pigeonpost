import type {ComponentProps, Dispatch, SetStateAction} from 'react'
import type {Folder, Message, Tag} from '../api'
import {isOutboxMessage} from '../outbox'
import {isJunkFolderMessage} from '../folderPaths'
import type {MessageClipboard} from '../hooks/useMessageClipboard'
import type {FolderPrompt} from '../hooks/useFolders'
import {MessageContextMenu} from './MessageContextMenu'
import {FolderContextMenu} from './FolderContextMenu'

// The two menus' own prop types are the source for the handlers passed straight through, so this
// interface cannot drift from what the menus actually take.
type MessageMenuProps = ComponentProps<typeof MessageContextMenu>
type FolderMenuProps = ComponentProps<typeof FolderContextMenu>

export interface ContextMenusProps {
    // Each menu's open state is its target plus where it was opened. Null is closed, which is why both
    // menus are conditional rather than hidden.
    contextMenu: {message: Message; x: number; y: number} | null
    folderContextMenu: {folder: Folder; x: number; y: number} | null
    closeContextMenu: () => void
    setFolderContextMenu: Dispatch<SetStateAction<{folder: Folder; x: number; y: number} | null>>
    // What the message menu acts on: the whole selection when more than one row is selected, otherwise
    // the message under the cursor.
    menuSelection: Message[]
    folders: Folder[]
    tags: Tag[]
    canMoveCopy: boolean
    canManageFolders: boolean
    // pasteFolderId is empty when the open folder cannot take a paste, which is what gates the entry.
    pasteFolderId: string
    clipboard: MessageClipboard
    pasteMessages: () => void
    // The single-message actions. Each is the app's own handler; the adapters the menus want are built
    // here rather than by the caller.
    openReply: (message: Message) => void
    openReplyAll: (message: Message) => void
    openForward: (message: Message) => void
    attachToNewMessage: (message: Message) => void
    openInNewTab: MessageMenuProps['onOpenInNewTab']
    saveMessageAs: (message: Message) => Promise<void>
    printMessage: (message: Message) => Promise<void>
    setReadState: (message: Message, read: boolean) => Promise<void>
    toggleFlag: (message: Message) => Promise<void>
    moveMessage: (message: Message, destFolderId: string) => Promise<void>
    copyMessage: (message: Message, destFolderId: string) => Promise<void>
    markJunk: (message: Message) => Promise<void>
    markNotJunk: (message: Message) => Promise<void>
    setMessageTagById: (messageId: string, tagId: string, assigned: boolean) => Promise<void>
    snoozeTo: (message: Message, at: Date) => Promise<void>
    unsnooze: (message: Message) => Promise<void>
    setSnoozePickerFor: Dispatch<SetStateAction<Message | null>>
    // The destructive entries open a confirmation rather than acting, so each is a setter or a request.
    requestDelete: (message: Message) => void
    setMessageToPurge: Dispatch<SetStateAction<Message | null>>
    setMessageToCancelSend: Dispatch<SetStateAction<Message | null>>
    setBulkToDelete: Dispatch<SetStateAction<Message[] | null>>
    setBulkToPurge: Dispatch<SetStateAction<Message[] | null>>
    // The bulk actions, offered when the menu opens on a selection of more than one.
    bulkSetRead: (messages: Message[], read: boolean) => Promise<void>
    bulkSetFlag: (messages: Message[], flagged: boolean) => Promise<void>
    // bulkMove is not awaited: it records an undo step and moves in the background.
    bulkMove: (messages: Message[], destFolderId: string) => void
    // The folder menu's own actions, each opening the prompt or confirmation its row button opens.
    setFolderPrompt: Dispatch<SetStateAction<FolderPrompt | null>>
    setFolderToDelete: Dispatch<SetStateAction<Folder | null>>
}

// ContextMenus holds both right-click menus: the one on a message row and the one on a folder row.
// Neither menu acts on anything itself: each entry calls back into the app. A destructive entry
// opens the same confirmation its toolbar button opens. Keeping the two together puts the whole
// right-click surface in one place, including the clipboard both of them read.
export function ContextMenus(props: ContextMenusProps) {
    const {contextMenu, folderContextMenu, clipboard} = props
    // An outbox message cannot be cut or copied, so it is dropped from the clipboard gestures rather
    // than the entries being hidden: a mixed selection still moves the messages that can move.
    const sendable = (messages: Message[]) => messages.filter((m) => !isOutboxMessage(m))
    const onDeleteFolder: FolderMenuProps['onDeleteFolder'] = (folder) => props.setFolderToDelete(folder)
    return (
        <>
            {contextMenu && (
                <MessageContextMenu
                    message={contextMenu.message}
                    x={contextMenu.x}
                    y={contextMenu.y}
                    folders={props.folders}
                    tags={props.tags}
                    selection={props.menuSelection}
                    onClose={props.closeContextMenu}
                    onReply={props.openReply}
                    onReplyAll={props.openReplyAll}
                    onForward={props.openForward}
                    onSetRead={(m, read) => void props.setReadState(m, read)}
                    onToggleFlag={(m) => void props.toggleFlag(m)}
                    onMove={(m, dest) => void props.moveMessage(m, dest)}
                    onCopy={(m, dest) => void props.copyMessage(m, dest)}
                    canMoveCopy={props.canMoveCopy}
                    onSetTag={(id, tagId, assigned) => void props.setMessageTagById(id, tagId, assigned)}
                    onCutMessages={(msgs) => clipboard.cutMessages(sendable(msgs))}
                    onCopyMessages={(msgs) => clipboard.copyMessages(sendable(msgs))}
                    onPaste={props.pasteMessages}
                    canPaste={clipboard.hasClip && props.pasteFolderId !== ''}
                    onOpenInNewTab={props.openInNewTab}
                    onSaveAs={(m) => void props.saveMessageAs(m)}
                    onPrint={(m) => void props.printMessage(m)}
                    onAttachToNew={props.attachToNewMessage}
                    onMarkJunk={(m) => void props.markJunk(m)}
                    onMarkNotJunk={(m) => void props.markNotJunk(m)}
                    isJunk={(m) => isJunkFolderMessage(m, props.folders)}
                    onSnooze={(m, at) => void props.snoozeTo(m, at)}
                    onSnoozeCustom={(m) => props.setSnoozePickerFor(m)}
                    onUnsnooze={(m) => void props.unsnooze(m)}
                    onDelete={props.requestDelete}
                    onDeletePermanent={(m) => props.setMessageToPurge(m)}
                    onCancelSend={(m) => props.setMessageToCancelSend(m)}
                    onBulkSetRead={(msgs, read) => void props.bulkSetRead(msgs, read)}
                    onBulkSetFlag={(msgs, flagged) => void props.bulkSetFlag(msgs, flagged)}
                    onBulkMove={(msgs, dest) => props.bulkMove(msgs, dest)}
                    onBulkDelete={(msgs) => props.setBulkToDelete(msgs)}
                    onBulkDeletePermanent={(msgs) => props.setBulkToPurge(msgs)}
                />
            )}
            {folderContextMenu && (
                <FolderContextMenu
                    folder={folderContextMenu.folder}
                    x={folderContextMenu.x}
                    y={folderContextMenu.y}
                    canPaste={clipboard.hasClip}
                    onPaste={(folder) => void clipboard.pasteInto(folder.id)}
                    canManageFolders={props.canManageFolders}
                    onNewSubfolder={(folder) => props.setFolderPrompt({mode: 'create', parent: folder})}
                    onRenameFolder={(folder) => props.setFolderPrompt({mode: 'rename', folder})}
                    onDeleteFolder={onDeleteFolder}
                    onClose={() => props.setFolderContextMenu(null)}
                />
            )}
        </>
    )
}
