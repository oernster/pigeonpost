import {useRef, useState} from 'react'
import type {Dispatch, SetStateAction} from 'react'
import type {Editor} from '@tiptap/react'
import {api, ComposeInput} from '../api'
import {AUTO_COLLECT_KEY, collectableRecipients, shouldAutoCollect} from '../autoCollect'
import {splitAddresses} from '../composeAddresses'
import {bodyMentionsAttachment} from '../composeAttachment'
import type {ComposeInitial, MessageAttachment} from '../components/ComposeModal'
import type {useComposeIntake} from './useComposeIntake'
import type {useDraftAutosave} from './useDraftAutosave'
import type {useSeparatorCorrection} from './useSeparatorCorrection'

export interface ComposeSendDeps {
    accountId: string
    senders: {name: string; address: string}[]
    initial: ComposeInitial | undefined
    from: string
    to: string
    cc: string
    bcc: string
    subject: string
    editor: Editor | null
    attachments: string[]
    setAttachments: Dispatch<SetStateAction<string[]>>
    messageAttachments: MessageAttachment[]
    setMessageAttachments: Dispatch<SetStateAction<MessageAttachment[]>>
    intake: ReturnType<typeof useComposeIntake>
    autosave: ReturnType<typeof useDraftAutosave>
    correction: ReturnType<typeof useSeparatorCorrection>
    setError: (message: string) => void
    onMarkReplied: (messageId: string) => void
    onMarkForwarded: (messageId: string) => void
    onDraftSuperseded: (draftId: string) => void
    onClose: () => void
}

// useComposeSend is everything that takes a message out of the compose window: the attachments it carries,
// sending now or later (after the separator fix and the attachment reminder), saving it as a draft and
// what follows a success (the reply or forward mark, the superseded draft and contact collection).
export function useComposeSend(deps: ComposeSendDeps) {
    const {
        accountId, senders, initial, from, to, cc, bcc, subject, editor, attachments, setAttachments,
        messageAttachments, setMessageAttachments, intake, autosave, correction, setError,
        onMarkReplied, onMarkForwarded, onDraftSuperseded, onClose,
    } = deps
    const [sending, setSending] = useState(false)
    const [savingDraft, setSavingDraft] = useState(false)

    // buildRequest packs the compose state for the backend. at is the send-later instant, null for an
    // immediate send.
    const buildRequest = (at: Date | null = null): ComposeInput => {
        const text = editor?.getText() ?? ''
        const html = editor?.getHTML() ?? ''
        return {
            accountId,
            from,
            to: splitAddresses(to),
            cc: splitAddresses(cc),
            bcc: splitAddresses(bcc),
            subject,
            body: text,
            // Only carry an HTML alternative when the body is non-empty, so an empty message stays plain.
            htmlBody: text.trim() === '' ? '' : html,
            attachmentPaths: attachments,
            attachmentData: intake.dataAttachments,
            attachmentMessageIds: messageAttachments.map((m) => m.id),
            sendAtMs: at === null ? 0 : at.getTime(),
        }
    }

    const removeMessageAttachment = (id: string) => {
        setMessageAttachments((prev) => prev.filter((m) => m.id !== id))
    }

    // addAttachments opens the native file picker and appends the chosen files, skipping any already
    // attached so the same file is not added twice.
    const addAttachments = async () => {
        try {
            const picked = await api.pickAttachments()
            if (picked.length > 0) {
                setAttachments((prev) => [...prev, ...picked.filter((p) => !prev.includes(p))])
            }
        } catch (e) {
            setError(String(e))
        }
    }

    const removeAttachment = (path: string) => {
        setAttachments((prev) => prev.filter((p) => p !== path))
    }

    // markOriginalOnSend reports a sent reply or forward on its original message so the row shows the
    // replied/forwarded indicator. The handlers own the server flag, the local cache and the in-memory list
    // update; this only says which original was acted on. It is fire-and-forget: it never blocks or fails the
    // send, so composing offline just leaves the indicator for the next sync.
    const markOriginalOnSend = () => {
        if (!initial?.inReplyToId) {
            return
        }
        if (initial.replyKind === 'reply') {
            onMarkReplied(initial.inReplyToId)
        } else if (initial.replyKind === 'forward') {
            onMarkForwarded(initial.inReplyToId)
        }
    }

    // supersedeDraft reports the stored draft this compose was reopened from, once its replacement has been
    // sent or saved. It runs only after that replacement succeeded, so the draft is never dropped before
    // something has taken its place.
    const supersedeDraft = () => {
        if (initial?.draftId) {
            onDraftSuperseded(initial.draftId)
        }
    }

    // send delivers the message now (at null) or schedules it for the chosen instant. A scheduled send
    // waits in the Outbox with Cancel send; it does not mark a reply or forward's original (a schedule
    // cancelled days later must not have already flagged it; the glyph is an accepted gap there).
    // maybeCollectContacts adds the message's recipients to the address book after a successful
    // send, when the automatic setting (on by default, toggled on the Contacts page) allows. The
    // sender's own addresses are never collected. Fire-and-forget: collection must never disturb a
    // send that has already succeeded.
    const maybeCollectContacts = () => {
        if (!shouldAutoCollect(window.localStorage.getItem(AUTO_COLLECT_KEY))) {
            return
        }
        const recipients = collectableRecipients(to, cc, bcc, senders.map((s) => s.address))
        if (recipients.length > 0) {
            void api.collectContacts(recipients).catch(() => {})
        }
    }

    const send = async (at: Date | null) => {
        autosave.stopAutosave()
        setSending(true)
        setError('')
        try {
            await api.send(buildRequest(at))
            maybeCollectContacts()
            if (at === null) {
                markOriginalOnSend()
            }
            supersedeDraft()
            void api.clearDraftRecovery()
            onClose()
        } catch (e) {
            autosave.resumeAutosave()
            setError(String(e))
            setSending(false)
        }
    }

    const [attachWarn, setAttachWarn] = useState(false)
    // sendLaterOpen shows the schedule row; sendAtValue is its datetime-local field. scheduleAtRef
    // carries a chosen moment through the attachment-reminder dialog, so "Send anyway" schedules
    // rather than sending now.
    const [sendLaterOpen, setSendLaterOpen] = useState(false)
    const [sendAtValue, setSendAtValue] = useState('')
    const scheduleAtRef = useRef<Date | null>(null)

    // canSend mirrors the Send button's enabled state, so Ctrl+Enter behaves exactly like the button.
    const canSend = () => !sending && !savingDraft && to.trim() !== ''
    const hasAttachments = () =>
        attachments.length > 0 || intake.dataAttachments.length > 0 || messageAttachments.length > 0
    // mentionsAttachment reports whether the message the user actually wrote talks about attaching
    // something, so it can prompt a reminder before sending. It passes the editor HTML (not its plain
    // text) so bodyMentionsAttachment can strip the quoted reply or forward chain: an attachment mentioned
    // only earlier in the thread must not trigger the reminder.
    const mentionsAttachment = () => bodyMentionsAttachment(subject, editor?.getHTML() ?? '')

    // attemptSend is the single entry point for the Send button, Ctrl+Enter and the send-later choices
    // (which pass their instant). It offers to fix a wrong address separator, then warns once when the
    // message mentions an attachment but none is attached, otherwise it sends or schedules straight away.
    const attemptSend = (at: Date | null = null) => {
        if (!canSend()) return
        if (correction.offer()) return
        if (mentionsAttachment() && !hasAttachments()) {
            scheduleAtRef.current = at
            setAttachWarn(true)
            return
        }
        void send(at)
    }

    const saveDraft = async () => {
        autosave.stopAutosave()
        setSavingDraft(true)
        setError('')
        try {
            await api.saveDraft(buildRequest(null))
            supersedeDraft()
            void api.clearDraftRecovery()
            onClose()
        } catch (e) {
            autosave.resumeAutosave()
            setError(String(e))
            setSavingDraft(false)
        }
    }

    return {
        sending, savingDraft, attachWarn, setAttachWarn, sendLaterOpen, setSendLaterOpen, sendAtValue, setSendAtValue,
        scheduleAtRef, canSend, hasAttachments, attemptSend, send, saveDraft, addAttachments, removeAttachment,
        removeMessageAttachment,
    }
}
