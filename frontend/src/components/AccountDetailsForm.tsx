import {useBackdropDismiss} from './useBackdropDismiss'
import {ModalClose} from './ModalClose'
import {RichTextField} from './RichTextField'
import {PROTOCOL_OPTIONS, SECURITY_OPTIONS} from '../accountProviders'
import type {AccountForm} from '../hooks/useAccountForm'

interface AccountDetailsFormProps {
    form: AccountForm
    onClose: () => void
}

// AccountDetailsForm is the account-setup details step: the name, email and password fields, the server
// settings (hidden for an OAuth account), the send-as addresses and the signature. Its state and every
// handler live in useAccountForm; this renders the fields around them. The reduced OAuth form (a Microsoft
// add or an OAuth-account edit) shows only the name, addresses and signature.
export function AccountDetailsForm({form, onClose}: AccountDetailsFormProps) {
    const dismiss = useBackdropDismiss(onClose)
    const {
        editing, msAdd, provider, oauthMode, oauthEditing,
        displayName, setDisplayName, email, onEmailChange, password, setPassword,
        protocol, onProtocolChange, inHost, setInHost, setInHostTouched, inPort, setInPort,
        inSecurity, setInSecurity, outHost, setOutHost, setOutHostTouched, outPort, setOutPort,
        outSecurity, setOutSecurity, identities, addIdentity, updateIdentity, removeIdentity,
        sigEditor, error, saving, msSigningIn, canSubmit, signInMicrosoft, submit, goBack,
    } = form

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
                        <button className="btn" onClick={goBack} disabled={saving || msSigningIn}>Back</button>
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
