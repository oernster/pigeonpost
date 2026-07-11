import {useState} from 'react'
import {useEditor} from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Link from '@tiptap/extension-link'
import {Account, AccountSetupInput, Identity, api} from '../api'
import {
    DEFAULT_IN_PORT,
    DEFAULT_IN_PORT_POP3,
    DEFAULT_IN_SECURITY,
    DEFAULT_OUT_PORT,
    DEFAULT_OUT_SECURITY,
    domainOf,
    incomingHostPrefix,
    type Provider,
} from '../accountProviders'

// useAccountForm owns the whole account-setup form: the flat field state, the step and chosen-provider
// state, the derived OAuth mode and submit validity, and every handler (choosing a provider, guessing the
// hosts from the email, the Microsoft OAuth sign-in and the save). onSaved is called with the account's
// email once it is added, updated or signed in.
export function useAccountForm(account: Account | null | undefined, onSaved: (email: string) => void) {
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

    // goBack returns from the details step to the provider chooser, leaving the Microsoft-add mode.
    const goBack = () => {
        setMsAdd(false)
        setStep('provider')
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

    return {
        editing, step, provider, msAdd, oauthMode, oauthEditing,
        displayName, setDisplayName,
        email, onEmailChange,
        password, setPassword,
        protocol, onProtocolChange,
        inHost, setInHost, setInHostTouched,
        inPort, setInPort,
        inSecurity, setInSecurity,
        outHost, setOutHost, setOutHostTouched,
        outPort, setOutPort,
        outSecurity, setOutSecurity,
        identities, addIdentity, updateIdentity, removeIdentity,
        sigEditor,
        error, saving, msSigningIn, canSubmit,
        chooseProvider, chooseMicrosoft, chooseManual,
        signInMicrosoft, submit, goBack,
    }
}

// AccountForm is the shape useAccountForm returns, consumed by AccountDetailsForm.
export type AccountForm = ReturnType<typeof useAccountForm>
