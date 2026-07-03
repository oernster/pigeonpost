import {useCallback, useEffect, useState} from 'react'
import './App.css'
import {AboutInfo, Account, api, Folder, Message} from './api'
import {applyTheme, loadTheme, Theme} from './theme'
import {Sidebar} from './components/Sidebar'
import {MessageList} from './components/MessageList'
import {Reader} from './components/Reader'
import {MenuBar} from './components/MenuBar'
import {AboutModal} from './components/AboutModal'
import {LicenceModal} from './components/LicenceModal'
import {ComposeModal} from './components/ComposeModal'
import {Splash} from './components/Splash'

function App() {
    const [accounts, setAccounts] = useState<Account[]>([])
    const [selectedAccount, setSelectedAccount] = useState<string>('')
    const [folders, setFolders] = useState<Folder[]>([])
    const [selectedFolder, setSelectedFolder] = useState<string>('')
    const [messages, setMessages] = useState<Message[]>([])
    const [selectedMessage, setSelectedMessage] = useState<Message | null>(null)
    const [error, setError] = useState<string>('')
    const [syncing, setSyncing] = useState<boolean>(false)
    const [theme, setTheme] = useState<Theme>(loadTheme())
    const [about, setAbout] = useState<AboutInfo | null>(null)
    const [licence, setLicence] = useState<string | null>(null)
    const [composing, setComposing] = useState<boolean>(false)
    const [appVersion, setAppVersion] = useState<string>('')
    const [appAuthor, setAppAuthor] = useState<string>('')
    const [splashVisible, setSplashVisible] = useState<boolean>(true)
    const [splashFading, setSplashFading] = useState<boolean>(false)

    useEffect(() => {
        applyTheme(theme)
    }, [theme])

    useEffect(() => {
        void api.version().then(setAppVersion).catch(() => undefined)
        void api.author().then(setAppAuthor).catch(() => undefined)
        const fade = window.setTimeout(() => setSplashFading(true), 1600)
        const hide = window.setTimeout(() => setSplashVisible(false), 2000)
        return () => {
            window.clearTimeout(fade)
            window.clearTimeout(hide)
        }
    }, [])

    const loadAccounts = useCallback(async () => {
        try {
            setAccounts(await api.listAccounts())
        } catch (e) {
            setError(String(e))
        }
    }, [])

    useEffect(() => {
        void loadAccounts()
    }, [loadAccounts])

    const selectAccount = useCallback(async (id: string) => {
        setSelectedAccount(id)
        setSelectedFolder('')
        setMessages([])
        setSelectedMessage(null)
        try {
            setFolders(await api.listFolders(id))
        } catch (e) {
            setError(String(e))
        }
    }, [])

    const selectFolder = useCallback(async (id: string) => {
        setSelectedFolder(id)
        setSelectedMessage(null)
        try {
            setMessages(await api.listMessages(id))
        } catch (e) {
            setError(String(e))
        }
    }, [])

    const sync = useCallback(async () => {
        if (!selectedAccount) {
            return
        }
        setSyncing(true)
        setError('')
        try {
            await api.syncAccount(selectedAccount)
            setFolders(await api.listFolders(selectedAccount))
            if (selectedFolder) {
                setMessages(await api.listMessages(selectedFolder))
            }
        } catch (e) {
            setError(String(e))
        } finally {
            setSyncing(false)
        }
    }, [selectedAccount, selectedFolder])

    const showAbout = useCallback(async () => {
        try {
            setAbout(await api.about())
        } catch (e) {
            setError(String(e))
        }
    }, [])

    const showLicence = useCallback(async () => {
        try {
            setLicence(await api.licence())
        } catch (e) {
            setError(String(e))
        }
    }, [])

    const checkUpdates = useCallback(() => {
        void api.openReleases()
    }, [])

    const toggleRead = useCallback(async (message: Message) => {
        try {
            await api.markRead(message.id, !message.read)
            if (selectedFolder) {
                const refreshed = await api.listMessages(selectedFolder)
                setMessages(refreshed)
                setSelectedMessage(refreshed.find((m) => m.id === message.id) ?? null)
            }
        } catch (e) {
            setError(String(e))
        }
    }, [selectedFolder])

    return (
        <div className="app">
            {splashVisible && <Splash version={appVersion} author={appAuthor} fading={splashFading}/>}
            <header className="titlebar">
                <span className="brand">PigeonPost</span>
                <div className="titlebar-right">
                    <button className="sync-btn" disabled={!selectedAccount} onClick={() => setComposing(true)}>
                        Compose
                    </button>
                    <button className="sync-btn" disabled={!selectedAccount || syncing} onClick={() => void sync()}>
                        {syncing ? 'Syncing...' : 'Sync'}
                    </button>
                    <MenuBar
                        theme={theme}
                        onToggleTheme={() => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))}
                        onShowAbout={() => void showAbout()}
                        onShowLicence={() => void showLicence()}
                        onCheckUpdates={checkUpdates}
                    />
                </div>
            </header>
            {error && <div className="error-bar" role="alert">{error}</div>}
            <div className="panes">
                <Sidebar
                    accounts={accounts}
                    selectedAccount={selectedAccount}
                    folders={folders}
                    selectedFolder={selectedFolder}
                    onSelectAccount={(id) => void selectAccount(id)}
                    onSelectFolder={(id) => void selectFolder(id)}
                />
                <MessageList
                    messages={messages}
                    selectedMessage={selectedMessage}
                    folderSelected={Boolean(selectedFolder)}
                    onSelectMessage={setSelectedMessage}
                />
                <Reader message={selectedMessage} onToggleRead={(m) => void toggleRead(m)}/>
            </div>
            <AboutModal about={about} onClose={() => setAbout(null)}/>
            <LicenceModal text={licence} onClose={() => setLicence(null)}/>
            {composing && selectedAccount && (
                <ComposeModal accountId={selectedAccount} onClose={() => setComposing(false)}/>
            )}
        </div>
    )
}

export default App
