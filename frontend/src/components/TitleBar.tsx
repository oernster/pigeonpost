import {Dispatch, SetStateAction} from 'react'
import {Message, UnreadCountsResult} from '../api'
import {Theme} from '../theme'
import {ComposeInitial} from './ComposeModal'
import {Menu, MenuItem} from './Menu'

// TitleBarProps is the header's slice of App: the unread badge, the five menu-bar arrays (built by useMenus),
// the derived gating flags the icon buttons read, and the handlers those buttons fire. It is prop-heavy
// because the header is the app's whole action bar; each field drives the control that names it.
export interface TitleBarProps {
    unreadCounts: UnreadCountsResult
    fileMenu: MenuItem[]
    editMenu: MenuItem[]
    viewMenu: MenuItem[]
    mailMenu: MenuItem[]
    helpMenu: MenuItem[]
    selectedAccount: string
    accountSyncing: boolean
    canMailAct: boolean
    canReplyAll: boolean
    activeMessage: Message | null
    displayMessages: Message[]
    theme: Theme
    signatureHtml: () => string
    setComposeInitial: Dispatch<SetStateAction<ComposeInitial | undefined>>
    setComposing: Dispatch<SetStateAction<boolean>>
    setSettingUp: Dispatch<SetStateAction<boolean>>
    sync: () => Promise<void>
    openReply: (message: Message) => void
    openReplyAll: (message: Message) => void
    openForward: (message: Message) => void
    setAttachPickerOpen: Dispatch<SetStateAction<boolean>>
    attachFiles: () => Promise<void>
    setManagingContacts: Dispatch<SetStateAction<boolean>>
    setManagingCalendar: Dispatch<SetStateAction<boolean>>
    setTheme: Dispatch<SetStateAction<Theme>>
}

// TitleBar is the header: the brand with the all-accounts unread badge, the File/Edit/View/Mail menus on the
// left, and the icon buttons on the right (compose, add account, sync, reply/reply-all/forward, the Attach
// menu, Contacts, Calendar, the theme toggle and Help). It is presentational: every action is a prop.
export function TitleBar(props: TitleBarProps) {
    const {
        unreadCounts, fileMenu, editMenu, viewMenu, mailMenu, helpMenu,
        selectedAccount, accountSyncing, canMailAct, canReplyAll, activeMessage, displayMessages, theme,
        signatureHtml, setComposeInitial, setComposing, setSettingUp, sync,
        openReply, openReplyAll, openForward, setAttachPickerOpen, attachFiles,
        setManagingContacts, setManagingCalendar, setTheme,
    } = props
    return (
            <header className="titlebar">
                <div className="titlebar-left">
                    <span className="brand">
                        PigeonPost
                        {unreadCounts.total > 0 && (
                            <span className="titlebar-unread" title={`${unreadCounts.total} unread across all accounts`}>
                                {unreadCounts.total}
                            </span>
                        )}
                    </span>
                    <Menu title="File" icon={'\u{1F4C1}'} items={fileMenu} align="left"/>
                    <Menu title="Edit" icon={'\u{270F}\u{FE0F}'} items={editMenu} align="left"/>
                    <Menu title="View" icon={'\u{1F441}\u{FE0F}'} items={viewMenu} align="left"/>
                    <Menu title="Mail" icon={'\u{1F4EC}'} items={mailMenu} align="left"/>
                </div>
                <div className="titlebar-right">
                    <button
                        className="icon-btn"
                        data-tip="Compose"
                        aria-label="Compose"
                        disabled={!selectedAccount}
                        onClick={() => {
                            const sig = signatureHtml()
                            setComposeInitial(sig ? {bodyHtml: `<p></p>${sig}`} : undefined)
                            setComposing(true)
                        }}
                    >
                        {'\u{1F58A}\u{FE0F}'}
                    </button>
                    <button
                        className="icon-btn"
                        data-tip="Add account"
                        aria-label="Add account"
                        onClick={() => setSettingUp(true)}
                    >
                        {'\u{2795}'}
                    </button>
                    <button
                        className="icon-btn"
                        data-tip={accountSyncing ? 'Synchronising…' : 'Sync'}
                        aria-label="Sync"
                        disabled={!selectedAccount || accountSyncing}
                        onClick={() => void sync()}
                    >
                        {'\u{267B}\u{FE0F}'}
                    </button>
                    <span className="titlebar-sep" aria-hidden="true"/>
                    <button
                        className="icon-btn"
                        data-tip="Reply"
                        aria-label="Reply"
                        disabled={!canMailAct}
                        onClick={() => activeMessage && openReply(activeMessage)}
                    >
                        {'\u{21A9}\u{FE0F}'}
                    </button>
                    <button
                        className="icon-btn"
                        data-tip="Reply all"
                        aria-label="Reply all"
                        disabled={!canReplyAll}
                        onClick={() => activeMessage && openReplyAll(activeMessage)}
                    >
                        {'\u{1F465}'}
                    </button>
                    <button
                        className="icon-btn"
                        data-tip="Forward"
                        aria-label="Forward"
                        disabled={!canMailAct}
                        onClick={() => activeMessage && openForward(activeMessage)}
                    >
                        {'\u{21AA}\u{FE0F}'}
                    </button>
                    <Menu
                        title="Attach"
                        icon={'\u{1F4CE}'}
                        align="left"
                        items={[
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
                        ]}
                    />
                    <span className="titlebar-sep" aria-hidden="true"/>
                    <button className="sync-btn" onClick={() => setManagingContacts(true)}>
                        {'\u{1F4C7}'} Contacts
                    </button>
                    <button className="sync-btn" onClick={() => setManagingCalendar(true)}>
                        {'\u{1F4C5}'} Calendar
                    </button>
                    <span className="titlebar-sep" aria-hidden="true"/>
                    <button
                        className="icon-btn theme-toggle"
                        data-tip={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
                        aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
                        onClick={() => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))}
                    >
                        {theme === 'dark' ? '☀️' : '\u{1F319}'}
                    </button>
                    <Menu title="Help" icon={'\u{2139}\u{FE0F}'} items={helpMenu}/>
                </div>
            </header>
    )
}
