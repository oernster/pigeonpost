import {useState} from 'react'
import {EditorContent, useEditor} from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Link from '@tiptap/extension-link'
import {Account, AccountSetupInput, api} from '../api'
import {useBackdropDismiss} from './useBackdropDismiss'
import {ModalClose} from './ModalClose'

// normaliseSigUrl gives a bare host a scheme so a signature link is absolute, matching the composer.
function normaliseSigUrl(url: string): string {
    const trimmed = url.trim()
    if (trimmed === '' || /^(https?:|mailto:)/i.test(trimmed)) {
        return trimmed
    }
    return `https://${trimmed}`
}

interface AccountSetupModalProps {
    account?: Account | null
    onClose: () => void
    onSaved: (email: string) => void
}

// Default endpoints for a generic IMAP + submission setup: implicit TLS on 993 for retrieval and
// STARTTLS on 587 for sending. The user can override every field.
const DEFAULT_IN_PORT = 993
const DEFAULT_OUT_PORT = 587
const DEFAULT_IN_SECURITY = 'tls'
const DEFAULT_OUT_SECURITY = 'starttls'
// POP3 retrieval uses implicit TLS on 995 rather than IMAP's 993.
const DEFAULT_IN_PORT_POP3 = 995

const PROTOCOL_OPTIONS: ReadonlyArray<{value: string; label: string}> = [
    {value: 'imap', label: 'IMAP (keeps mail on the server)'},
    {value: 'pop3', label: 'POP3 (downloads mail, single inbox)'},
]

// incomingHostPrefix is the conventional hostname prefix guessed for each retrieval protocol.
function incomingHostPrefix(protocol: string): string {
    return protocol === 'pop3' ? 'pop' : 'imap'
}

const SECURITY_OPTIONS: ReadonlyArray<{value: string; label: string}> = [
    {value: 'tls', label: 'TLS (implicit)'},
    {value: 'starttls', label: 'STARTTLS'},
    {value: 'none', label: 'None (plaintext)'},
]

// Provider is a known email service with its server settings pre-filled. Choosing one skips manual
// server entry; the account itself is still added through the same AddAccount path.
interface Provider {
    id: string
    name: string
    note: string
    inHost: string
    inPort: number
    inSecurity: string
    outHost: string
    outPort: number
    outSecurity: string
}

const PROVIDERS: readonly Provider[] = [
    {
        id: 'gmail', name: 'Gmail',
        inHost: 'imap.gmail.com', inPort: 993, inSecurity: 'tls',
        outHost: 'smtp.gmail.com', outPort: 587, outSecurity: 'starttls',
        note: 'Personal Gmail only. Turn on 2-Step Verification, then generate an app password and use it here.',
    },
    {
        id: 'icloud', name: 'iCloud Mail',
        inHost: 'imap.mail.me.com', inPort: 993, inSecurity: 'tls',
        outHost: 'smtp.mail.me.com', outPort: 587, outSecurity: 'starttls',
        note: 'Create an app-specific password in your Apple ID settings and use it here.',
    },
    {
        id: 'yahoo', name: 'Yahoo Mail',
        inHost: 'imap.mail.yahoo.com', inPort: 993, inSecurity: 'tls',
        outHost: 'smtp.mail.yahoo.com', outPort: 465, outSecurity: 'tls',
        note: 'Yahoo requires an app password generated in your account security settings.',
    },
    {
        id: 'fastmail', name: 'Fastmail',
        inHost: 'imap.fastmail.com', inPort: 993, inSecurity: 'tls',
        outHost: 'smtp.fastmail.com', outPort: 465, outSecurity: 'tls',
        note: 'Create an app password in Fastmail settings and use it here.',
    },
    {
        id: 'startmail', name: 'StartMail',
        inHost: 'imap.startmail.com', inPort: 993, inSecurity: 'tls',
        outHost: 'smtp.startmail.com', outPort: 587, outSecurity: 'starttls',
        note: 'Enable IMAP/SMTP under Settings in StartMail, then use your app-specific password.',
    },
]

function domainOf(email: string): string {
    const at = email.indexOf('@')
    return at >= 0 ? email.slice(at + 1).trim() : ''
}

export function AccountSetupModal({account, onClose, onSaved}: AccountSetupModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    const editing = Boolean(account)
    // Editing an existing account jumps straight to the details form; adding starts on the chooser.
    const [step, setStep] = useState<'provider' | 'details'>(editing ? 'details' : 'provider')
    const [provider, setProvider] = useState<Provider | null>(null)
    const [displayName, setDisplayName] = useState(account?.displayName ?? '')
    const [email, setEmail] = useState(account?.email ?? '')
    const [password, setPassword] = useState('')
    const [protocol, setProtocol] = useState(account?.protocol ?? 'imap')
    const [inHost, setInHost] = useState(account?.inHost ?? '')
    const [inPort, setInPort] = useState(account?.inPort ?? DEFAULT_IN_PORT)
    const [inSecurity, setInSecurity] = useState(account?.inSecurity ?? DEFAULT_IN_SECURITY)
    const [outHost, setOutHost] = useState(account?.outHost ?? '')
    const [outPort, setOutPort] = useState(account?.outPort ?? DEFAULT_OUT_PORT)
    const [outSecurity, setOutSecurity] = useState(account?.outSecurity ?? DEFAULT_OUT_SECURITY)
    // In edit mode the servers are already set, so they are treated as user-provided (no auto-guess).
    const [inHostTouched, setInHostTouched] = useState(editing)
    const [outHostTouched, setOutHostTouched] = useState(editing)
    const [saving, setSaving] = useState(false)
    const [msSigningIn, setMsSigningIn] = useState(false)
    const [error, setError] = useState('')
    const [sigLinkOpen, setSigLinkOpen] = useState(false)
    const [sigLinkUrl, setSigLinkUrl] = useState('')

    // The signature is edited as rich text (HTML), matching the composer. An empty editor is stored as ''
    // so a new message gets no signature block; a non-empty one is inserted above the quote on reply.
    const sigEditor = useEditor({
        extensions: [StarterKit, Link.configure({openOnClick: false, autolink: true, linkOnPaste: true})],
        content: account?.signature ?? '',
    })

    const openSigLink = () => {
        setSigLinkUrl((sigEditor?.getAttributes('link').href as string) ?? '')
        setSigLinkOpen(true)
    }

    const applySigLink = () => {
        const href = normaliseSigUrl(sigLinkUrl)
        if (href === '') {
            sigEditor?.chain().focus().extendMarkRange('link').unsetLink().run()
        } else {
            sigEditor?.chain().focus().extendMarkRange('link').setLink({href}).run()
        }
        setSigLinkOpen(false)
        setSigLinkUrl('')
    }

    const sigBtn = (active: boolean, label: string, title: string, onClick: () => void) => (
        <button
            type="button"
            className={'compose-tool' + (active ? ' active' : '')}
            title={title}
            aria-label={title}
            aria-pressed={active}
            onMouseDown={(e) => e.preventDefault()}
            onClick={onClick}
        >
            {label}
        </button>
    )

    const chooseProvider = (p: Provider) => {
        setProvider(p)
        setInHost(p.inHost)
        setInPort(p.inPort)
        setInSecurity(p.inSecurity)
        setOutHost(p.outHost)
        setOutPort(p.outPort)
        setOutSecurity(p.outSecurity)
        setInHostTouched(true)
        setOutHostTouched(true)
        setStep('details')
    }

    // Microsoft accounts sign in through OAuth rather than an app password: this opens the system browser
    // for consent and waits for the loopback redirect, so the modal shows a waiting state until the Go
    // side returns the signed-in address (or an error). No server details or password are collected here.
    const signInMicrosoft = async () => {
        setMsSigningIn(true)
        setError('')
        try {
            const signedInEmail = await api.signInMicrosoft('')
            onSaved(signedInEmail)
        } catch (e) {
            setError(String(e))
            setMsSigningIn(false)
        }
    }

    const chooseManual = () => {
        setProvider(null)
        setInHost('')
        setInPort(DEFAULT_IN_PORT)
        setInSecurity(DEFAULT_IN_SECURITY)
        setOutHost('')
        setOutPort(DEFAULT_OUT_PORT)
        setOutSecurity(DEFAULT_OUT_SECURITY)
        setInHostTouched(false)
        setOutHostTouched(false)
        setStep('details')
    }

    // Guess the server hosts from the email domain until the user edits them (manual mode only). The
    // incoming prefix follows the retrieval protocol (imap. or pop.).
    const onEmailChange = (value: string) => {
        setEmail(value)
        const domain = domainOf(value)
        if (domain) {
            if (!inHostTouched) {
                setInHost(`${incomingHostPrefix(protocol)}.${domain}`)
            }
            if (!outHostTouched) {
                setOutHost(`smtp.${domain}`)
            }
        }
    }

    // Changing the retrieval protocol tracks the default incoming port (993 for IMAP, 995 for POP3)
    // unless the user set a custom one, and re-guesses the incoming host while it is still auto-managed.
    const onProtocolChange = (value: string) => {
        setProtocol(value)
        if (value === 'pop3' && inPort === DEFAULT_IN_PORT) {
            setInPort(DEFAULT_IN_PORT_POP3)
        } else if (value === 'imap' && inPort === DEFAULT_IN_PORT_POP3) {
            setInPort(DEFAULT_IN_PORT)
        }
        const domain = domainOf(email)
        if (domain && !inHostTouched) {
            setInHost(`${incomingHostPrefix(value)}.${domain}`)
        }
    }

    const canSubmit =
        displayName.trim() !== '' &&
        email.trim() !== '' &&
        inHost.trim() !== '' &&
        outHost.trim() !== '' &&
        (editing || password !== '')

    const submit = async () => {
        setSaving(true)
        setError('')
        const req: AccountSetupInput = {
            displayName: displayName.trim(),
            email: email.trim(),
            password,
            protocol,
            inHost: inHost.trim(),
            inPort,
            inSecurity,
            outHost: outHost.trim(),
            outPort,
            outSecurity,
            // An empty editor serialises to "<p></p>"; store it as blank so no signature is inserted.
            signature: sigEditor && !sigEditor.isEmpty ? sigEditor.getHTML() : '',
        }
        try {
            if (editing) {
                await api.updateAccount(req)
            } else {
                await api.addAccount(req)
            }
            onSaved(email.trim())
        } catch (e) {
            setError(String(e))
            setSaving(false)
        }
    }

    if (step === 'provider') {
        return (
            <div className="modal-backdrop" {...dismiss}>
                <div className="modal setup" role="dialog" aria-label="Add account" onClick={(e) => e.stopPropagation()}>
                    <ModalClose onClose={onClose}/>
                    <h2 className="modal-title">Add account</h2>
                    <p className="setup-hint">Choose your email provider, or set the servers up yourself.</p>
                    {error && <div className="compose-error">{error}</div>}
                    <div className="provider-grid">
                        <button className="provider-btn" onClick={() => void signInMicrosoft()} disabled={msSigningIn}>
                            {msSigningIn ? 'Waiting for your browser...' : 'Microsoft'}
                        </button>
                        {PROVIDERS.map((p) => (
                            <button key={p.id} className="provider-btn" onClick={() => chooseProvider(p)} disabled={msSigningIn}>
                                {p.name}
                            </button>
                        ))}
                    </div>
                    <button className="btn manual-btn" onClick={chooseManual} disabled={msSigningIn}>Set up manually (other provider)</button>
                    <div className="modal-actions">
                        <button className="btn" onClick={onClose} disabled={msSigningIn}>Cancel</button>
                    </div>
                </div>
            </div>
        )
    }

    const serverFields = (
        <>
            <fieldset className="setup-group">
                <legend>Incoming ({protocol.toUpperCase()})</legend>
                <label className="field">
                    <span>Protocol</span>
                    <select value={protocol} onChange={(e) => onProtocolChange(e.target.value)}>
                        {PROTOCOL_OPTIONS.map((o) => (
                            <option key={o.value} value={o.value}>{o.label}</option>
                        ))}
                    </select>
                </label>
                <label className="field">
                    <span>Server</span>
                    <input
                        value={inHost}
                        onChange={(e) => {
                            setInHostTouched(true)
                            setInHost(e.target.value)
                        }}
                        placeholder="imap.example.com"
                    />
                </label>
                <div className="setup-row">
                    <label className="field narrow">
                        <span>Port</span>
                        <input type="number" value={inPort} onChange={(e) => setInPort(Number(e.target.value))}/>
                    </label>
                    <label className="field">
                        <span>Security</span>
                        <select value={inSecurity} onChange={(e) => setInSecurity(e.target.value)}>
                            {SECURITY_OPTIONS.map((o) => (
                                <option key={o.value} value={o.value}>{o.label}</option>
                            ))}
                        </select>
                    </label>
                </div>
            </fieldset>

            <fieldset className="setup-group">
                <legend>Outgoing (SMTP)</legend>
                <label className="field">
                    <span>Server</span>
                    <input
                        value={outHost}
                        onChange={(e) => {
                            setOutHostTouched(true)
                            setOutHost(e.target.value)
                        }}
                        placeholder="smtp.example.com"
                    />
                </label>
                <div className="setup-row">
                    <label className="field narrow">
                        <span>Port</span>
                        <input type="number" value={outPort} onChange={(e) => setOutPort(Number(e.target.value))}/>
                    </label>
                    <label className="field">
                        <span>Security</span>
                        <select value={outSecurity} onChange={(e) => setOutSecurity(e.target.value)}>
                            {SECURITY_OPTIONS.map((o) => (
                                <option key={o.value} value={o.value}>{o.label}</option>
                            ))}
                        </select>
                    </label>
                </div>
            </fieldset>
        </>
    )

    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal setup" role="dialog" aria-label={editing ? 'Edit account' : 'Add account'} onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">
                    {editing ? 'Edit account' : provider ? `Add ${provider.name}` : 'Add account'}
                </h2>
                <p className="setup-hint">
                    PigeonPost signs in to your incoming server to check these details before saving. Your
                    password is stored in the operating system keychain, never in the app database.
                </p>
                {provider && <p className="provider-note">{provider.note}</p>}
                {error && <div className="compose-error">{error}</div>}

                <label className="field">
                    <span>Your name</span>
                    <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} autoFocus placeholder="Jane Doe"/>
                </label>
                <label className="field">
                    <span>Email</span>
                    <input
                        value={email}
                        readOnly={editing}
                        className={editing ? 'locked' : undefined}
                        onChange={(e) => onEmailChange(e.target.value)}
                        placeholder="jane@example.com"
                    />
                </label>
                <label className="field">
                    <span>Password</span>
                    <input type="password" value={password} onChange={(e) => setPassword(e.target.value)}/>
                </label>
                {editing && <p className="field-hint">Leave the password blank to keep your current one.</p>}

                {provider ? (
                    <details className="setup-advanced">
                        <summary>Server settings (pre-filled for {provider.name})</summary>
                        {serverFields}
                    </details>
                ) : (
                    serverFields
                )}

                <fieldset className="setup-group">
                    <legend>Signature</legend>
                    <p className="field-hint">Added to new messages, and above the quoted text on a reply.</p>
                    <div className="compose-toolbar">
                        {sigBtn(sigEditor?.isActive('bold') ?? false, 'B', 'Bold', () => sigEditor?.chain().focus().toggleBold().run())}
                        {sigBtn(sigEditor?.isActive('italic') ?? false, 'I', 'Italic', () => sigEditor?.chain().focus().toggleItalic().run())}
                        {sigBtn(sigEditor?.isActive('link') ?? false, '🔗', 'Link', openSigLink)}
                    </div>
                    {sigLinkOpen && (
                        <div className="compose-link-row">
                            <input
                                className="tag-name-input"
                                value={sigLinkUrl}
                                autoFocus
                                placeholder="https://example.com"
                                onChange={(e) => setSigLinkUrl(e.target.value)}
                                onKeyDown={(e) => {
                                    if (e.key === 'Enter') {
                                        e.preventDefault()
                                        applySigLink()
                                    }
                                }}
                            />
                            <button className="btn primary" onClick={applySigLink}>Apply</button>
                        </div>
                    )}
                    <EditorContent editor={sigEditor} className="compose-editor signature-editor"/>
                </fieldset>

                <div className="modal-actions spread">
                    {editing ? (
                        <button className="btn" onClick={onClose} disabled={saving}>Cancel</button>
                    ) : (
                        <button className="btn" onClick={() => setStep('provider')} disabled={saving}>Back</button>
                    )}
                    <button className="btn primary" onClick={() => void submit()} disabled={saving || !canSubmit}>
                        {saving ? 'Verifying...' : editing ? 'Save changes' : 'Add account'}
                    </button>
                </div>
            </div>
        </div>
    )
}
