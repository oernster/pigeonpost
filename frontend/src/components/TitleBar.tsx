import {Dispatch, SetStateAction} from 'react'
import {UnreadCountsResult} from '../api'
import {Theme} from '../theme'
import {ComposeInitial} from './ComposeModal'
import {Menu, MenuItem} from './Menu'

// TitleBarProps is the header's slice of App: the unread badge, the five menu-bar arrays (built by useMenus),
// the derived gating flags the icon buttons read plus the handlers those buttons fire. It is prop-heavy
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
    theme: Theme
    signatureHtml: () => string
    setComposeInitial: Dispatch<SetStateAction<ComposeInitial | undefined>>
    setComposing: Dispatch<SetStateAction<boolean>>
    setSettingUp: Dispatch<SetStateAction<boolean>>
    sync: () => Promise<void>
    setManagingContacts: Dispatch<SetStateAction<boolean>>
    setManagingCalendar: Dispatch<SetStateAction<boolean>>
    setTheme: Dispatch<SetStateAction<Theme>>
}

// TitleBar is the header in three groups: the brand with the all-accounts unread badge plus the
// File/Edit/View/Mail menus on the left, the working controls centred on the window (compose, add
// account, sync, Contacts, Calendar) and the app-level pair on the right (the theme toggle and Help).
// It is presentational: every action is a prop.
//
// The tray carries only what is live whatever is selected. Reply, reply-all, forward and Attach each needed a
// selected message, so they stood greyed out most of the time while taking room the tray needs to lay itself
// out. They now live in the Mail menu (Respond and Attach); the reply trio is also on the reader's own
// toolbar and in the message right-click menu, each beside the rest of its group.
export function TitleBar(props: TitleBarProps) {
    const {
        unreadCounts, fileMenu, editMenu, viewMenu, mailMenu, helpMenu,
        selectedAccount, accountSyncing, theme,
        signatureHtml, setComposeInitial, setComposing, setSettingUp, sync,
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
                <div className="titlebar-centre">
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
                    <button className="sync-btn" onClick={() => setManagingContacts(true)}>
                        <span className="btn-icon">{'\u{1F4C7}'}</span> Contacts
                    </button>
                    <button className="sync-btn" onClick={() => setManagingCalendar(true)}>
                        <span className="btn-icon">{'\u{1F4C5}'}</span> Calendar
                    </button>
                </div>
                <div className="titlebar-right">
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
