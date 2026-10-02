import {useRef, useState} from 'react'
import {useBackdropDismiss} from './useBackdropDismiss'
import {EditorContent, useEditor} from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import {api, Template} from '../api'
import {EDITOR_LINK_OPTIONS, EDITOR_PASTE_PROPS} from '../richText'
import {DataAttachment} from '../composeIntake'
import {useComposeIntake} from '../hooks/useComposeIntake'
import {useContactPool} from '../hooks/useContactPool'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'
import {normaliseUrl} from '../composeAddresses'
import {useLinkEditor} from '../hooks/useLinkEditor'
import {useDraftAutosave} from '../hooks/useDraftAutosave'
import {useSeparatorCorrection} from '../hooks/useSeparatorCorrection'
import {EditorTool, formattingTools} from '../editorTools'
import {useToolbarNav} from '../hooks/useToolbarNav'
import {useModalDrag} from '../hooks/useModalDrag'
import {useComposeSend} from '../hooks/useComposeSend'
import {useComposeTemplates} from '../hooks/useComposeTemplates'
import {ComposeHeader} from './ComposeHeader'
import {ComposeToolbar} from './ComposeToolbar'
import {ComposeAttachments} from './ComposeAttachments'
import {SendLaterRow} from './SendLaterRow'

// ComposeInitial pre-fills the compose window, used by reply, reply-all and forward.
// MessageAttachment is an existing email attached to a new message: its id (fetched and rendered as a
// message/rfc822 part at send time) and a display name for its chip.
export interface MessageAttachment {
    id: string
    name: string
}

export interface ComposeInitial {
    // accountId names the account this compose sends from when it is not the selected one: a reply or
    // forward opened from a unified-mailbox row must send from the row's own account. Unset falls back
    // to the selected account.
    accountId?: string
    // from preselects the sender address (used by reply to send as the address the message was delivered
    // to). Empty falls back to the account's primary address.
    from?: string
    to?: string
    cc?: string
    bcc?: string
    subject?: string
    bodyHtml?: string
    messageAttachments?: MessageAttachment[]
    // attachmentPaths pre-attaches files by path, used when the Attach button picks files before opening a
    // fresh compose so the chosen files are already attached.
    attachmentPaths?: string[]
    // attachmentData pre-attaches in-memory files (pasted or dropped, so the webview holds bytes with no
    // path), used when a saved draft reopens the compose exactly as it was stored.
    attachmentData?: DataAttachment[]
    // inReplyToId and replyKind mark this compose as a reply or forward of an existing message, so once it is
    // sent the original can be flagged \Answered (reply / reply-all) or $Forwarded (forward). Both are unset
    // for a fresh compose, a restored draft or an attach-to-new-message.
    inReplyToId?: string
    replyKind?: 'reply' | 'forward'
    // draftId is the stored Drafts-mailbox message this compose was reopened from (see openDraft). Once the
    // edited message has been sent or saved again, that superseded copy is deleted, so finishing a draft
    // never leaves a stale second copy behind. Unset for every other way of opening the composer.
    draftId?: string
}

// Sender is one address the account may send from, offered in the From dropdown.
interface Sender {
    name: string
    address: string
}

interface ComposeModalProps {
    accountId: string
    // senders are the account's sendable addresses (primary first, then identities). When there is more
    // than one, the compose window shows a From dropdown.
    senders: Sender[]
    initial?: ComposeInitial
    // canSaveDraft is false for POP3 accounts, which have no server-side Drafts mailbox to append to.
    canSaveDraft: boolean
    // onMarkReplied / onMarkForwarded record a sent reply or forward on its original message (by id), so the
    // row shows the replied / forwarded glyph at once. They own the server flag, the local cache and the
    // in-memory list update; the composer just reports which original was acted on. Called best-effort after a
    // successful send, so a failure never disrupts the send.
    onMarkReplied: (id: string) => void
    onMarkForwarded: (id: string) => void
    // onDraftSuperseded reports the stored draft this compose was reopened from (initial.draftId) once its
    // replacement has been sent or saved, so the now-stale copy can be removed. Like the marking handlers
    // above, the composer only says which draft was superseded; the handler owns the server delete, the
    // in-memory list and the folder counts.
    onDraftSuperseded: (id: string) => void
    onClose: () => void
}


export function ComposeModal({accountId, senders, initial, canSaveDraft, onMarkReplied, onMarkForwarded, onDraftSuperseded, onClose}: ComposeModalProps) {
    // The chosen From address. It defaults to the reply's delivered-to address when given, otherwise the
    // account's primary (first) sender. The backend validates it against the account's owned addresses.
    const [from, setFrom] = useState(initial?.from || senders[0]?.address || '')
    const [to, setTo] = useState(initial?.to ?? '')
    const [cc, setCc] = useState(initial?.cc ?? '')
    const [bcc, setBcc] = useState(initial?.bcc ?? '')
    const [subject, setSubject] = useState(initial?.subject ?? '')
    const [attachments, setAttachments] = useState<string[]>(initial?.attachmentPaths ?? [])
    const [messageAttachments, setMessageAttachments] = useState<MessageAttachment[]>(initial?.messageAttachments ?? [])
    const [error, setError] = useState('')

    // attemptSendRef lets the editor's key handler call the latest attemptSend without recreating the
    // editor: the editor is built once while attemptSend closes over state that changes each render.
    const attemptSendRef = useRef<() => void>(() => {})
    // noteEditRef bridges the editor's onUpdate to the autosave (created below), for the same reason: the
    // editor is built once while the autosave's noteEdit is recreated each render.
    const noteEditRef = useRef<() => void>(() => {})
    // intakeRef bridges the editor's paste and drop handlers to the latest intakeFiles, again because the
    // editor is built once while intakeFiles closes over per-render state.
    const intakeRef = useRef<(dt: DataTransfer | null) => boolean>(() => false)

    const editor = useEditor({
        extensions: [
            StarterKit.configure({link: EDITOR_LINK_OPTIONS}),
            // allowBase64 keeps a pasted image's data: URI in the document; the backend lifts it into a
            // proper inline MIME part at send time.
            Image.configure({allowBase64: true}),
        ],
        content: initial?.bodyHtml ?? '',
        // Initial focus is the top of the body, not the To field: a reply, reply-all or forward opens
        // ready to type above the quoted text; a fresh compose reaches To with one Shift+Tab.
        autofocus: 'start',
        onUpdate: () => noteEditRef.current(),
        editorProps: {
            ...EDITOR_PASTE_PROPS,
            // Ctrl+Enter (Cmd+Enter on macOS) sends. Returning true stops TipTap from also inserting its
            // default Mod-Enter hard break.
            handleKeyDown: (_view, event) => {
                if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
                    event.preventDefault()
                    attemptSendRef.current()
                    return true
                }
                return false
            },
            // Pasted or dropped files follow the intake rule: images embed at the cursor, other files
            // attach. Returning true stops ProseMirror's own handling for the ones taken.
            handlePaste: (_view, event) => intakeRef.current(event.clipboardData),
            handleDrop: (_view, event) => intakeRef.current(event.dataTransfer),
        },
    })

    // The local draft autosave watches the recipient fields and the editor; noteEdit is the editor's
    // onUpdate, bridged through noteEditRef because the editor is built once (above) before this runs.
    const autosave = useDraftAutosave({accountId, to, cc, bcc, subject, editor})
    noteEditRef.current = autosave.noteEdit

    // The inline link-editing row is the shared hook, seeded from the current selection's link.
    const link = useLinkEditor(editor, normaliseUrl)

    // The separator-correction safeguard offers to fix a wrong address separator on a send attempt. It reads
    // the recipient fields; when the fix is approved it writes the corrected addresses back and marks the
    // draft dirty.
    const correction = useSeparatorCorrection({
        to, cc, bcc, setTo, setCc, setBcc, setError, markDirty: autosave.markDirty,
    })

    // The paste and drop intake (images embed, other files attach) is the shared hook; its take is
    // bridged through intakeRef because the editor is built once (above) before this runs. A paste
    // that carries file paths rather than File objects attaches by path, deduped like the picker.
    const intake = useComposeIntake({
        editor,
        setError,
        markDirty: autosave.markDirty,
        addPaths: (paths) => setAttachments((prev) => [...prev, ...paths.filter((p) => !prev.includes(p))]),
        initial: initial?.attachmentData,
    })
    intakeRef.current = intake.take

    // The contact suggestion pool behind the To, Cc and Bcc autocompletes, loaded lazily on the
    // first touch of a recipient field and shared by all three.
    const contacts = useContactPool()

    // Every discard path (the backdrop, Escape, the close cross, Cancel) goes through requestClose:
    // a compose the user has actually edited and that still holds content must confirm before it is
    // thrown away, so a stray click (such as the one that refocuses the window) can never silently
    // lose a message. An untouched or emptied-out compose closes at once. Send and Save draft call
    // onClose directly, having preserved the message.
    const [confirmDiscard, setConfirmDiscard] = useState(false)
    const composedContent = () =>
        to.trim() !== '' || cc.trim() !== '' || bcc.trim() !== '' || subject.trim() !== '' ||
        (editor?.getText() ?? '').trim() !== ''
    const requestClose = () => {
        if (!autosave.isDirty()) {
            // Nothing was ever written for this compose, so there is no snapshot of it to clear. It is
            // left alone rather than cleared anyway: the recovery slot is a single slot, so clearing it
            // here would throw away a snapshot left by an earlier session that the user has not yet
            // answered for.
            onClose()
            return
        }
        if (composedContent()) {
            setConfirmDiscard(true)
            return
        }
        // Dirty with nothing left in it: closing asks nothing, though a snapshot was written before the
        // user emptied it out. The autosave clears the slot itself in that case, though only after its
        // debounce, which closing the window cancels; so the clear happens here instead.
        discard()
    }

    // discard throws the message away for good, which means the local recovery slot goes with it. Closing
    // alone was not enough and the failure only showed on the NEXT launch: the message left the screen
    // while the snapshot taken as it was written stayed on disk, so the app opened offering to recover a
    // message the user had just watched it discard. Applying a template made it reliable rather than
    // occasional, since a template fills the compose at once and the snapshot is written a second and a
    // half later, whether or not the user typed anything themselves.
    //
    // Stopping the autosave first is the other half. The snapshot is debounced, so one already scheduled
    // would otherwise land after the clear and write the slot straight back.
    const discard = () => {
        autosave.stopAutosave()
        void api.clearDraftRecovery()
        onClose()
    }
    const dismiss = useBackdropDismiss(requestClose)

    // Sending, scheduling, saving as a draft and the attachments the message carries are their own hook.
    const sender = useComposeSend({
        accountId, senders, initial, from, to, cc, bcc, subject, editor, attachments, setAttachments,
        messageAttachments, setMessageAttachments, intake, autosave, correction, setError,
        onMarkReplied, onMarkForwarded, onDraftSuperseded, onClose,
    })
    const {
        sending, savingDraft, attachWarn, setAttachWarn, sendLaterOpen, setSendLaterOpen, sendAtValue,
        setSendAtValue, scheduleAtRef, hasAttachments, attemptSend, send, saveDraft, addAttachments,
        removeAttachment, removeMessageAttachment,
    } = sender
    attemptSendRef.current = () => attemptSend()
    // The template picker is its own hook.
    const {templates, templatePicker, openTemplatePicker, insertTemplate} = useComposeTemplates({
        editor, subject, setSubject, markDirty: autosave.markDirty, intake, setError,
    })

    // The formatting strip is one focus-ring stop (roving tabindex; see useToolbarNav): the tools
    // are data so the toolbar renders and navigates from one list. A separator follows the tools
    // that end a visual group.
    const tools: EditorTool[] = [
        ...formattingTools(editor, link.openLink, {trailingSeparator: true}),
        // The face is an icon rather than the word "Template": among B, I, S, H and the rest, a lone
        // word did not read as a control at all, so the picker went unnoticed and templates could be
        // written but never used. The name it announces is unchanged.
        {glyph: '📄', name: 'Insert template', active: templatePicker, run: () => void openTemplatePicker(), hasPopup: true},
    ]
    const toolbar = useToolbarNav(tools.length)

    // The compose window is movable by its title bar, so a long reply can be pushed aside to re-read the
    // message underneath it. Reopening a stored draft retitles the window, which is the only signal that
    // sending or saving replaces an existing draft rather than adding one.
    const drag = useModalDrag()
    const title = initial?.draftId ? 'Edit draft' : 'New message'

    return (
        <div className="modal-backdrop" {...dismiss}>
            <div ref={drag.ref} style={drag.style}
                 className="modal compose pinned-actions" role="dialog" aria-label={title} onClick={(e) => e.stopPropagation()}
                 onDragOver={(e) => e.preventDefault()}
                 onDrop={(e) => {
                     // Files dropped anywhere on the compose window follow the intake rule (images
                     // embed, other files attach). A drop on the editor itself is handled by the
                     // editor's own handleDrop, so it is skipped here rather than taken twice.
                     if ((e.target as HTMLElement).closest('.ProseMirror')) {
                         return
                     }
                     e.preventDefault()
                     intakeRef.current(e.dataTransfer)
                 }}
                 onKeyDownCapture={(e) => {
                     // The editor handles Ctrl+Enter itself (see editorProps); this covers the
                     // To, Cc, Bcc and Subject fields, where there is no editor to intercept it.
                     if ((e.ctrlKey || e.metaKey) && e.key === 'Enter' &&
                         !(e.target as HTMLElement).closest('.ProseMirror')) {
                         e.preventDefault()
                         attemptSend()
                     }
                 }}>
                <ModalClose onClose={requestClose}/>
                <h2 {...drag.handleProps} className={`modal-title ${drag.handleProps.className}`}>{title}</h2>
                {/* The address block is pinned above the scrolling body, so who the message is from, who it
                    goes to and its subject stay on screen however long the message grows. Measured at the
                    700px minimum window, a long message scrolled the body and carried From and To away. */}
                <ComposeHeader
                    error={error} correction={correction} senders={senders} from={from} setFrom={setFrom}
                    to={to} setTo={setTo} cc={cc} setCc={setCc} bcc={bcc} setBcc={setBcc}
                    subject={subject} setSubject={setSubject} contacts={contacts} markDirty={autosave.markDirty}/>

                <div className="modal-body">
                <ComposeToolbar tools={tools} toolbar={toolbar} templatePicker={templatePicker} templates={templates}
                                insertTemplate={insertTemplate} link={link}/>
                <EditorContent editor={editor} className="compose-editor"/>

                <ComposeAttachments
                    attachments={attachments} messageAttachments={messageAttachments} intake={intake}
                    hasAttachments={hasAttachments} addAttachments={addAttachments} removeAttachment={removeAttachment}
                    removeMessageAttachment={removeMessageAttachment}/>

                {sendLaterOpen && (
                    <SendLaterRow sendAtValue={sendAtValue} setSendAtValue={setSendAtValue}
                                  setSendLaterOpen={setSendLaterOpen} attemptSend={attemptSend}/>
                )}
                </div>
                <div className="modal-actions spread">
                    <button className="btn" onClick={requestClose} disabled={sending || savingDraft}>Cancel</button>
                    <div className="compose-send-group">
                        {canSaveDraft && (
                            <button className="btn" onClick={() => void saveDraft()} disabled={sending || savingDraft}>
                                {savingDraft ? 'Saving...' : 'Save draft'}
                            </button>
                        )}
                        <button
                            className="btn"
                            onClick={() => setSendLaterOpen((open) => !open)}
                            disabled={sending || savingDraft || to.trim() === ''}
                            aria-haspopup="menu"
                            aria-expanded={sendLaterOpen}
                            title="Send at a chosen time"
                        >
                            Send later
                        </button>
                        <button className="btn primary" onClick={() => attemptSend()} disabled={sending || savingDraft || to.trim() === ''}
                                title="Send (Ctrl+Enter)">
                            {sending ? 'Sending...' : 'Send'}
                        </button>
                    </div>
                </div>
            </div>

            {confirmDiscard && (
                <ConfirmDialog
                    title="Discard message?"
                    message="This message has not been sent or saved as a draft. Discard it?"
                    confirmLabel="Discard"
                    onConfirm={discard}
                    onCancel={() => setConfirmDiscard(false)}
                />
            )}
            {attachWarn && (
                <ConfirmDialog
                    title="Attachment reminder"
                    message="Did you want to attach anything before sending?"
                    confirmLabel="Send anyway"
                    busy={sending}
                    onConfirm={() => {
                        setAttachWarn(false)
                        void send(scheduleAtRef.current)
                    }}
                    onCancel={() => setAttachWarn(false)}
                />
            )}
        </div>
    )
}
