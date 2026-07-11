import {useState} from 'react'
import {useEditor} from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Link from '@tiptap/extension-link'
import {Account, AccountSetupInput, Identity, api} from '../api'
import {useBackdropDismiss} from './useBackdropDismiss'
import {ModalClose} from './ModalClose'
import {RichTextField} from './RichTextField'
import {ProviderChooser} from './ProviderChooser'
import {
    DEFAULT_IN_PORT,
    DEFAULT_IN_PORT_POP3,
    DEFAULT_IN_SECURITY,
    DEFAULT_OUT_PORT,
    DEFAULT_OUT_SECURITY,
    PROTOCOL_OPTIONS,
    SECURITY_OPTIONS,
    domainOf,
    incomingHostPrefix,
    type Provider,
} from '../accountProviders'

interface AccountSetupModalProps {
    account?: Account | null
    onClose: () => void
    onSaved: (email: string) => void
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
    // msAdd is true while adding a Microsoft (OAuth) account: the details step then collects only the
    // sender name before launching the browser sign-in, with no password or server fields.
    const [msAdd, setMsAdd] = useState(false)
    const [error, setError] = useState('')
    // Alternate sender addresses (aliases this account may send as, such as a domain alias sharing the
    // mailbox). Prefilled when editing; the compose window offers them as From options.
    const [identities, setIdentities] = useState<Identity[]>(
        account?.identities?.map((i) => ({name: i.name, address: i.address})) ?? [],
    )
    const addIdentity = () => setIdentities([...identities, {name: '', address: ''}])
    const updateIdentity = (index: number, field: 'name' | 'address', value: string) =>
        setIdentities(identities.map((id, i) => (i === index ? {...id, [field]: value} : id)))
    const removeIdentity = (index: number) => setIdentities(identities.filter((_, i) => i !== index))

    // The signature is edited as rich text (HTML), matching the composer. An empty editor is stored as ''
    // so a new message gets no signature block; a non-empty one is inserted above the quote on reply.
    const sigEditor = useEditor({
        extensions: [StarterKit, Link.configure({openOnClick: false, autolink: true, linkOnPaste: true})],
        content: account?.signature ?? '',
    })

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

    // Choosing Microsoft moves to the details step to collect the sender name first: the OAuth flow
    // returns the signed-in address but not a display name, so it is gathered here before sign-in.
    const chooseMicrosoft = () => {
        setMsAdd(true)
        setStep('details')
    }

    // Microsoft accounts sign in through OAuth rather than an app password: this opens the system browser
    // for consent and waits for the loopback redirect, so the modal shows a waiting state until the Go
    // side returns the signed-in address (or an error). The chosen sender name is passed through; no
    // server details or password are collected.
    const signInMicrosoft = async () => {
        setMsSigningIn(true)
        setError('')
        try {
            const signedInEmail = await api.signInMicrosoft(displayName.trim())
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

    // An OAuth account edits only its profile: the modal hides the password and server fields; the save
    // skips credential verification. Adding Microsoft (msAdd) is the same reduced form ahead of the
    // browser sign-in. Both are oauthMode.
    const oauthEditing = editing && account?.auth === 'oauth2'
    const oauthMode = msAdd || oauthEditing

    const canSubmit = oauthMode
        ? displayName.trim() !== ''
        : displayName.trim() !== '' &&
          email.trim() !== '' &&
          inHost.trim() !== '' &&
          outHost.trim() !== '' &&
          (editing || password !== '')

    const submit = async () => {
        setSaving(true)
        setError('')
        if (oauthEditing) {
            try {
                await api.updateAccountProfile({
                    email: email.trim(),
                    displayName: displayName.trim(),
                    signature: sigEditor && !sigEditor.isEmpty ? sigEditor.getHTML() : '',
                    identities: identities
                        .filter((i) => i.address.trim() !== '')
                        .map((i) => ({name: i.name.trim(), address: i.address.trim()})),
                })
                onSaved(email.trim())
            } catch (e) {
                setError(String(e))
                setSaving(false)
            }
            return
        }
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
            // Drop blank rows so a half-filled identity is not saved; the backend validates the rest.
            identities: identities
                .filter((i) => i.address.trim() !== '')
                .map((i) => ({name: i.name.trim(), address: i.address.trim()})),
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
            <ProviderChooser
                error={error}
                busy={msSigningIn}
                onClose={onClose}
                onChooseMicrosoft={chooseMicrosoft}
                onChooseProvider={chooseProvider}
                onChooseManual={chooseManual}
            />
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
                    {editing ? 'Edit account' : msAdd ? 'Add Microsoft' : provider ? `Add ${provider.name}` : 'Add account'}
                </h2>
                {oauthMode ? (
                    <p className="setup-hint">
                        {msAdd
                            ? 'Enter the name to show on messages you send, then continue to sign in through your browser.'
                            : 'Update the name shown on your messages, your signature or your send-as addresses.'}
                    </p>
                ) : (
                    <p className="setup-hint">
                        PigeonPost signs in to your incoming server to check these details before saving. Your
                        password is stored in the operating system keychain, never in the app database.
                    </p>
                )}
                {provider && !oauthMode && <p className="provider-note">{provider.note}</p>}
                {error && <div className="compose-error">{error}</div>}

                <label className="field">
                    <span>Your name</span>
                    <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} autoFocus placeholder="Jane Doe"/>
                </label>
                {!msAdd && (
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
                )}
                {!oauthMode && (
                    <>
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
                    </>
                )}

                {!msAdd && (
                    <>
                        <fieldset className="setup-group">
                            <legend>Send-as addresses</legend>
                            <p className="field-hint">Extra addresses this account may send from (for example a domain alias that shares this mailbox). They appear as From options when you compose.</p>
                            {identities.map((identity, i) => (
                                <div className="identity-row" key={i}>
                                    <input
                                        className="identity-input"
                                        value={identity.name}
                                        placeholder="Name (optional)"
                                        onChange={(e) => updateIdentity(i, 'name', e.target.value)}
                                    />
                                    <input
                                        className="identity-input"
                                        value={identity.address}
                                        placeholder="alias@example.com"
                                        onChange={(e) => updateIdentity(i, 'address', e.target.value)}
                                    />
                                    <button type="button" className="btn identity-remove" onClick={() => removeIdentity(i)}>Remove</button>
                                </div>
                            ))}
                            <button type="button" className="btn" onClick={addIdentity}>Add address</button>
                        </fieldset>

                        <fieldset className="setup-group">
                            <legend>Signature</legend>
                            <p className="field-hint">Added to new messages and above the quoted text on a reply.</p>
                            <RichTextField editor={sigEditor}/>
                        </fieldset>
                    </>
                )}

                <div className="modal-actions spread">
                    {editing ? (
                        <button className="btn" onClick={onClose} disabled={saving}>Cancel</button>
                    ) : (
                        <button className="btn" onClick={() => { setMsAdd(false); setStep('provider') }} disabled={saving || msSigningIn}>Back</button>
                    )}
                    {msAdd ? (
                        <button className="btn primary" onClick={() => void signInMicrosoft()} disabled={msSigningIn || !canSubmit}>
                            {msSigningIn ? 'Waiting for your browser...' : 'Continue with Microsoft'}
                        </button>
                    ) : (
                        <button className="btn primary" onClick={() => void submit()} disabled={saving || !canSubmit}>
                            {saving ? (oauthEditing ? 'Saving...' : 'Verifying...') : editing ? 'Save changes' : 'Add account'}
                        </button>
                    )}
                </div>
            </div>
        </div>
    )
}
