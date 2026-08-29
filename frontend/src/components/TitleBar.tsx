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

// TitleBar is the header in three groups: the app mark with the all-accounts unread badge held at the
// left, then every control gathered against the right edge as one run (the File/Edit/View/Mail menus
// running straight into compose, add account, sync, Contacts and Calendar), then the app-level pair
// closing the bar behind a rule of its own (the theme toggle and Help).
//
// The mark is alone on the left deliberately: it is the one thing in the bar that is not a control, so it
// keeps the corner a window's icon belongs in while everything that acts on mail sits together at the
// other end. Read right to left the bar is Help, the theme toggle, a rule, then Calendar back to File.
//
// Centring the controls was tried twice and is not what this is. The first attempt left the menus on the
// left and moved only the working controls, which split one sequence into two with a gap between them.
// The second moved the menus too and held the whole run dead centre; it was correct and still read as a
// row floating in an empty bar. Against the edge it reads as a toolbar.
//
// The mark wears an icon button's box so it sits in the row of controls as one of them; it is a span
// rather than a button, because there is nothing to press. That is what keeps it out of the tab order
// and off the focus ring, with no markup needed to exclude it. It stands where the wordmark used to;
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
                    {/* The rule between Calendar and the theme toggle, marking the app-level pair off
                        from the working controls. It leads this group rather than trailing the one
                        before it so that it belongs to the pair it introduces; either position renders
                        identically, since the two groups sit flush against each other. */}
                    <span className="titlebar-sep" aria-hidden="true"/>
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
