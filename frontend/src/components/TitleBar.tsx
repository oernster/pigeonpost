import {Dispatch, SetStateAction} from 'react'
import {UnreadCountsResult} from '../api'
import appMark from '../assets/pigeonpost.png'
import {icons} from '../icons'
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

// TitleBar is the header in three groups: the app mark with the all-accounts unread badge, then the
// File/Edit/View/Mail menus running straight into the working controls (compose, add account, sync,
// Contacts and Calendar), all reading left to right from the mark, then the app-level pair held at the
// far end (the theme toggle and Help). The spare width between them is the separation; a rule there was
// tried and dropped, because with the controls back on the left it stood against the toggle rather than
// between the two things it was meant to divide.
//
// Three other arrangements were tried and none survived a maximised window. Centring only the working
// controls split one sequence into two with a gap in the middle. Centring the menus with them held the
// run dead centre, which measured exactly right and read as a row floating in empty bar. Pushing the
// whole run to the right edge put every control a screen's width from the mark, worse the wider the
// window got. A toolbar goes on the left.
//
// The mark borrows an icon button's box so it sizes and aligns with the controls beside it while
// painting none of it: no background and no border, because a frame says a thing can be pressed and
// this one cannot. It is a span rather than a button for the same reason, which is what keeps it out of the tab
// order and off the focus ring, with no markup needed to exclude it. It stands where the wordmark used to;
// the window title names the application in text.
// It is presentational: every action is a prop.
//
// Every control in the tray carries drawn artwork from ../icons rather than an emoji, so the whole bar is
// one set in one hand: an emoji is drawn by whichever font the platform happens to have and neither its
// weight nor its palette can be relied on across Windows, macOS and Linux.
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
                    <span className="icon-btn icon-btn-image titlebar-mark" aria-hidden="true">
                        <img src={appMark} alt="" draggable={false}/>
                    </span>
                    {unreadCounts.total > 0 && (
                        <span className="titlebar-unread" title={`${unreadCounts.total} unread across all accounts`}>
                            {unreadCounts.total}
                        </span>
                    )}
                </div>
                <div className="titlebar-actions">
                    <Menu title="File" icon={icons.file} items={fileMenu} align="left"/>
                    <Menu title="Edit" icon={icons.edit} items={editMenu} align="left"/>
                    <Menu title="View" icon={icons.view} items={viewMenu} align="left"/>
                    <Menu title="Mail" icon={icons.mail} items={mailMenu} align="left"/>
                    <span className="titlebar-sep" aria-hidden="true"/>
                    <button
                        className="icon-btn icon-btn-image"
                        data-tip="Compose"
                        aria-label="Compose"
                        disabled={!selectedAccount}
                        onClick={() => {
                            const sig = signatureHtml()
                            setComposeInitial(sig ? {bodyHtml: `<p></p>${sig}`} : undefined)
                            setComposing(true)
                        }}
                    >
                        <img src={icons.compose} alt="" draggable={false}/>
                    </button>
                    <button
                        className="icon-btn icon-btn-image"
                        data-tip="Add account"
                        aria-label="Add account"
                        onClick={() => setSettingUp(true)}
                    >
                        <img src={icons.addAccount} alt="" draggable={false}/>
                    </button>
                    <button
                        className="icon-btn icon-btn-image"
                        data-tip={accountSyncing ? 'Synchronising…' : 'Sync'}
                        aria-label="Sync"
                        disabled={!selectedAccount || accountSyncing}
                        onClick={() => void sync()}
                    >
                        <img src={icons.sync} alt="" draggable={false}/>
                    </button>
                    <span className="titlebar-sep" aria-hidden="true"/>
                    <button
                        className="icon-btn icon-btn-image"
                        data-tip="Contacts"
                        aria-label="Contacts"
                        onClick={() => setManagingContacts(true)}
                    >
                        <img src={icons.contacts} alt="" draggable={false}/>
                    </button>
                    <button
                        className="icon-btn icon-btn-image"
                        data-tip="Calendar"
                        aria-label="Calendar"
                        onClick={() => setManagingCalendar(true)}
                    >
                        <img src={icons.calendar} alt="" draggable={false}/>
                    </button>
                </div>
                <div className="titlebar-right">
                    <button
                        className="icon-btn icon-btn-image theme-toggle"
                        data-tip={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
                        aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
                        onClick={() => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))}
                    >
                        <img src={theme === 'dark' ? icons.lightMode : icons.darkMode} alt="" draggable={false}/>
                    </button>
                    <Menu title="Help" icon={icons.help} items={helpMenu}/>
                </div>
            </header>
    )
}
