import {api} from '../api'
import {MenuItem} from '../components/Menu'
import {TAG_PALETTE, colourTagId} from '../tagColours'
import {snoozeChoices} from '../schedule'
import {isJunkFolderMessage} from '../folderPaths'
import type {MenusDeps} from './useMenus'

// buildMailMenu builds the Mail menu: composing and attaching, the account-level items, then every action
// on the open message. It is rebuilt on every render with the other menus (see useMenus), so an item's
// label and enabled state always follow the current selection. It lives apart from the other four
// because it is most of the menu bar: the actions on a message are one concern of their own.
export function buildMailMenu(deps: MenusDeps): MenuItem[] {
    const {
        activeMessage, activeOutbox, canMailAct, canReplyAll, canMoveCopy, selectedAccount, accountSyncing,
        isWindows, conversationView, folders, messageTags, displayMessages, signatureHtml, setComposeInitial,
        setComposing, setSettingUp, sync, openInNewTab, openThread, openReply, openReplyAll, openForward,
        attachToNewMessage, attachFiles, setAttachPickerOpen, setReadState, toggleFlag, toggleTag, moveMessage,
        copyMessage, markJunk, markNotJunk, snoozeTo, unsnooze, setSnoozePickerFor, setMessageToCancelSend,
    } = deps

    const mailMoveTargets = activeMessage ? folders.filter((f) => f.id !== activeMessage.folderId) : []
    const appliedTagIds = new Set(messageTags.map((t) => t.id))
    return [
        {
            // Hidden: composing has its own button in the title bar, so an entry here would be a second
            // way to say the same thing at the top of the menu. The item stays for Ctrl+N, which is
            // wired from these definitions and would otherwise go with it.
            label: 'Compose',
            hidden: true,
            shortcut: 'Ctrl+N',
            disabled: !selectedAccount,
            onClick: () => {
                const sig = signatureHtml()
                setComposeInitial(sig ? {bodyHtml: `<p></p>${sig}`} : undefined)
                setComposing(true)
            },
        },
        {
            label: 'Attach',
            icon: '\u{1F4CE}',
            submenu: [
                {
                    label: 'Attach email...',
                    icon: '\u{2709}\u{FE0F}',
                    disabled: !selectedAccount || displayMessages.length === 0,
                    onClick: () => setAttachPickerOpen(true),
                },
                {
                    label: 'Attach file(s)...',
                    icon: '\u{1F4C4}',
                    disabled: !selectedAccount,
                    onClick: () => void attachFiles(),
                },
            ],
        },
        {
            label: 'Add account',
            onClick: () => setSettingUp(true),
        },
        {
            label: accountSyncing ? 'Synchronising…' : 'Sync',
            shortcut: 'F9',
            disabled: !selectedAccount || accountSyncing,
            onClick: () => void sync(),
        },
        ...(isWindows
            ? [{
                label: 'Set as default for .eml...',
                icon: '\u{1F4CC}',
                onClick: () => void api.showDefaultAppSettings(),
            }, {
                label: 'Set as default email client...',
                icon: '\u{2709}\u{FE0F}',
                onClick: () => void api.showDefaultMailAppSettings(),
            }]
            : []),
        {label: '', separator: true},
        {
            label: 'Open in new tab',
            shortcut: 'Ctrl+T',
            disabled: !canMailAct,
            onClick: () => activeMessage && openInNewTab(activeMessage),
        },
        {
            // Offered only while the conversation view is on: the tick governs the whole feature, so a
            // keyboard route into a thread must not survive switching conversations off.
            label: 'Open conversation',
            shortcut: 'Ctrl+Shift+T',
            disabled: !canMailAct || !conversationView,
            onClick: () => activeMessage && openThread(activeMessage.id),
        },
        {label: '', separator: true},
        {
            label: 'Respond',
            icon: '\u{21A9}\u{FE0F}',
            disabled: !canMailAct,
            // The accelerators are Thunderbird's: Ctrl+R replies, Ctrl+Shift+R replies to all and
            // Ctrl+L forwards.
            submenu: [
                {label: 'Reply', icon: '\u{21A9}\u{FE0F}', shortcut: 'Ctrl+R', disabled: !canMailAct, onClick: () => activeMessage && openReply(activeMessage)},
                {label: 'Reply all', icon: '\u{1F465}', shortcut: 'Ctrl+Shift+R', disabled: !canReplyAll, onClick: () => activeMessage && openReplyAll(activeMessage)},
                {label: 'Forward', icon: '\u{21AA}\u{FE0F}', shortcut: 'Ctrl+L', disabled: !canMailAct, onClick: () => activeMessage && openForward(activeMessage)},
                {
                    label: 'Attach to new message',
                    icon: '\u{1F4CE}',
                    disabled: !canMailAct,
                    onClick: () => activeMessage && attachToNewMessage(activeMessage),
                },
            ],
        },
        {label: '', separator: true},
        {
            label: activeMessage?.read ? 'Mark as unread' : 'Mark as read',
            disabled: !canMailAct,
            onClick: () => activeMessage && void setReadState(activeMessage, !activeMessage.read),
        },
        {
            label: activeMessage?.flagged ? 'Remove star' : 'Add star',
            disabled: !canMailAct,
            onClick: () => activeMessage && void toggleFlag(activeMessage),
        },
        {
            label: 'Tag with colour',
            disabled: !canMailAct,
            submenu: TAG_PALETTE.map((c) => {
                const id = colourTagId(c.colour)
                const on = appliedTagIds.has(id)
                return {label: c.name, swatch: c.colour, checked: on, onClick: () => void toggleTag(id, !on)}
            }),
        },
        ...(activeMessage && activeMessage.snoozedUntilMs > 0
            ? [{
                label: 'Unsnooze',
                icon: '\u{23F0}',
                disabled: !canMailAct,
                onClick: () => activeMessage && void unsnooze(activeMessage),
            }]
            : [{
                label: 'Snooze',
                icon: '\u{23F0}',
                disabled: !canMailAct,
                submenu: [
                    ...snoozeChoices(new Date()).map((choice) => ({
                        label: choice.label,
                        onClick: () => activeMessage && void snoozeTo(activeMessage, choice.at),
                    })),
                    {label: 'Pick a time...', onClick: () => activeMessage && setSnoozePickerFor(activeMessage)},
                ],
            }]),
        {label: '', separator: true},
        {
            label: 'Move to',
            disabled: !canMailAct || !canMoveCopy || mailMoveTargets.length === 0,
            submenu: mailMoveTargets.map((f) => ({
                label: f.name,
                onClick: () => activeMessage && void moveMessage(activeMessage, f.id),
            })),
        },
        {
            label: 'Copy to',
            disabled: !canMailAct || !canMoveCopy || mailMoveTargets.length === 0,
            submenu: mailMoveTargets.map((f) => ({
                label: f.name,
                onClick: () => activeMessage && void copyMessage(activeMessage, f.id),
            })),
        },
        // A message already in Junk offers the rescue back to the inbox instead of re-junking.
        activeMessage && isJunkFolderMessage(activeMessage, folders) ? {
            label: 'Not junk',
            disabled: !canMailAct || !canMoveCopy,
            onClick: () => activeMessage && void markNotJunk(activeMessage),
        } : {
            label: 'Mark as junk',
            disabled: !canMailAct || !canMoveCopy,
            onClick: () => activeMessage && void markJunk(activeMessage),
        },
        {label: '', separator: true},
        {
            label: 'Cancel send',
            disabled: !activeOutbox,
            onClick: () => activeMessage && setMessageToCancelSend(activeMessage),
        },
    ]
}
