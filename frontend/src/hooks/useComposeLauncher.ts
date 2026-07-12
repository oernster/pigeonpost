import {Dispatch, SetStateAction, useEffect, useRef, useState} from 'react'
import {Account, DraftRecoveryResult, Message, MessageBody, api} from '../api'
import {ComposeInitial} from '../components/ComposeModal'
import {useEscapeToClose} from '../components/useBackdropDismiss'
import {emlFilename} from '../messageText'
import {buildForward, buildReply, buildReplyAll, quoteFor, replyFromAddress, sendersFor, signatureHtmlFor} from '../replyDraft'

// ComposeLauncherDeps is what launching a composer needs from the rest of App: the account list and the
// selected account (to derive the From address and the signature), the selected-account setter (a recovered
// draft switches to the identity it was written under), the fetched body of the open message (quoted into a
// reply or forward) and the error sink.
export interface ComposeLauncherDeps {
    accounts: Account[]
    selectedAccount: string
    setSelectedAccount: Dispatch<SetStateAction<string>>
    messageBody: MessageBody | null
    setError: (message: string) => void
}

export interface ComposeLauncher {
    composing: boolean
    setComposing: Dispatch<SetStateAction<boolean>>
    composeInitial: ComposeInitial | undefined
    setComposeInitial: Dispatch<SetStateAction<ComposeInitial | undefined>>
    attachPickerOpen: boolean
    setAttachPickerOpen: Dispatch<SetStateAction<boolean>>
    recovery: DraftRecoveryResult | null
    setRecovery: Dispatch<SetStateAction<DraftRecoveryResult | null>>
    // signatureHtml is exposed because the Compose buttons (which open a blank composer) still live in App and
    // seed a new message with the account signature.
    signatureHtml: () => string
    openReply: (message: Message) => void
    openReplyAll: (message: Message) => void
    openForward: (message: Message) => void
    attachToNewMessage: (message: Message) => void
    attachFiles: () => Promise<void>
    attachEmails: (picked: Message[]) => void
    restoreDraft: () => void
    discardDraft: () => void
}

// useComposeLauncher owns opening the composer: replying, replying-all and forwarding the selected message
// (pre-filling from the pure replyDraft builders), attaching a message or files to a fresh message, and the
// draft-recovery prompt offered once on launch. The composer state (composing, composeInitial) and the attach
// picker and recovery flags live here; the ComposeModal render, the Compose buttons and the recovery dialog
// stay in App and consume these, so signatureHtml and the raw setters are exposed.
export function useComposeLauncher(deps: ComposeLauncherDeps): ComposeLauncher {
    const {accounts, selectedAccount, setSelectedAccount, messageBody, setError} = deps

    const [composing, setComposing] = useState<boolean>(false)
    const [composeInitial, setComposeInitial] = useState<ComposeInitial | undefined>(undefined)
    // attachPickerOpen shows the message picker for the Attach button's "Attach email" action.
    const [attachPickerOpen, setAttachPickerOpen] = useState(false)
    // recovery is a locally autosaved compose snapshot from a previous session, offered for restore once
    // accounts have loaded; recoveryCheckedRef makes that offer happen once per launch.
    const [recovery, setRecovery] = useState<DraftRecoveryResult | null>(null)
    const recoveryCheckedRef = useRef<boolean>(false)

    // Close the draft-recovery prompt on Escape, matching the other dialogs. It is a plain inline modal, so
    // it does not use the shared backdrop hook; the active flag registers it only while it is showing.
    useEscapeToClose(() => setRecovery(null), Boolean(recovery) && !composing)

    // Once accounts have loaded, check for a compose snapshot autosaved in a previous session and offer to
    // restore it. This runs once per launch. A snapshot whose account has since been removed is stale, so
    // it is cleared silently rather than offered against an account that no longer exists.
    useEffect(() => {
        if (recoveryCheckedRef.current || accounts.length === 0) return
        recoveryCheckedRef.current = true
        void (async () => {
            try {
                const snapshot = await api.draftRecovery()
                if (!snapshot.present) return
                if (accounts.some((account) => account.id === snapshot.accountId)) {
                    setRecovery(snapshot)
                } else {
                    void api.clearDraftRecovery()
                }
            } catch {
                // A recovery check failure is non-fatal; the composer still works without it.
            }
        })()
    }, [accounts])

    // restoreDraft reopens the composer pre-filled from the autosaved snapshot, switching to its account
    // first so the message is sent from the identity it was written under. The composer's own autosave
    // then keeps the snapshot current, so it is not cleared here.
    const restoreDraft = () => {
        if (!recovery) return
        if (accounts.some((account) => account.id === recovery.accountId)) {
            setSelectedAccount(recovery.accountId)
        }
        setComposeInitial({
            to: recovery.to,
            cc: recovery.cc,
            bcc: recovery.bcc,
            subject: recovery.subject,
            bodyHtml: recovery.bodyHtml,
        })
        setComposing(true)
        setRecovery(null)
    }

    // discardDraft drops the autosaved snapshot when the user chooses not to restore it.
    const discardDraft = () => {
        void api.clearDraftRecovery()
        setRecovery(null)
    }

    // signatureHtml is the selected account's signature (see replyDraft.signatureHtmlFor), inserted into a new
    // message and above the quoted text on a reply or forward.
    const signatureHtml = (): string => signatureHtmlFor(accounts.find((a) => a.id === selectedAccount))

    // replyFrom picks which of the account's own addresses a reply is sent from (see replyDraft.replyFromAddress).
    const replyFrom = (message: Message): string =>
        replyFromAddress(message, sendersFor(accounts.find((a) => a.id === selectedAccount)))

    const openReply = (message: Message) => {
        setComposeInitial({
            ...buildReply(message, {
                from: replyFrom(message),
                signatureHtml: signatureHtml(),
                quotedHtml: quoteFor(message, messageBody),
            }),
            inReplyToId: message.id,
            replyKind: 'reply',
        })
        setComposing(true)
    }

    const openReplyAll = (message: Message) => {
        setComposeInitial({
            ...buildReplyAll(message, selectedAccount, {
                from: replyFrom(message),
                signatureHtml: signatureHtml(),
                quotedHtml: quoteFor(message, messageBody),
            }),
            inReplyToId: message.id,
            replyKind: 'reply',
        })
        setComposing(true)
    }

    const openForward = (message: Message) => {
        setComposeInitial({
            ...buildForward(message, {
                signatureHtml: signatureHtml(),
                quotedHtml: quoteFor(message, messageBody),
            }),
            inReplyToId: message.id,
            replyKind: 'forward',
        })
        setComposing(true)
    }

    // attachToNewMessage opens a fresh composer with the chosen message attached as a .eml; the backend
    // fetches its raw bytes and adds it as a message/rfc822 part on send.
    const attachToNewMessage = (message: Message) => {
        setComposeInitial({
            messageAttachments: [{id: message.id, name: emlFilename(message.subject || '')}],
            bodyHtml: signatureHtml() ? `<p></p>${signatureHtml()}` : undefined,
        })
        setComposing(true)
    }

    // attachFiles picks files from the OS then opens a fresh compose with them already attached, for the
    // Attach button's "Attach file(s)" action. A cancelled picker opens nothing.
    const attachFiles = async () => {
        try {
            const paths = await api.pickAttachments()
            if (paths.length === 0) {
                return
            }
            setComposeInitial({
                attachmentPaths: paths,
                bodyHtml: signatureHtml() ? `<p></p>${signatureHtml()}` : undefined,
            })
            setComposing(true)
        } catch (e) {
            setError(String(e))
        }
    }

    // attachEmails opens a fresh compose with the picked messages attached as .eml files, for the Attach
    // button's "Attach email" action; the picker closes as it hands them over.
    const attachEmails = (picked: Message[]) => {
        setAttachPickerOpen(false)
        setComposeInitial({
            messageAttachments: picked.map((m) => ({id: m.id, name: emlFilename(m.subject || '')})),
            bodyHtml: signatureHtml() ? `<p></p>${signatureHtml()}` : undefined,
        })
        setComposing(true)
    }

    return {
        composing, setComposing,
        composeInitial, setComposeInitial,
        attachPickerOpen, setAttachPickerOpen,
        recovery, setRecovery,
        signatureHtml,
        openReply, openReplyAll, openForward,
        attachToNewMessage, attachFiles, attachEmails,
        restoreDraft, discardDraft,
    }
}
