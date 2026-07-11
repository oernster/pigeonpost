import {useEffect, useState} from 'react'
import {api, Attachment, EmailView, Message} from '../api'
import {formatBytes} from '../readerFormat'

interface ReaderAttachmentsProps {
    message: Message
    // attachments is always non-empty: the reader renders this only when the loaded body carries attachments.
    attachments: Attachment[]
    // onViewEmail hands a parsed .eml attachment to the reader's in-app viewer.
    onViewEmail: (email: EmailView) => void
}

// ReaderAttachments is the attachments block below a message: the count, an optional Save all, and one row
// per attachment with Open and Save. It owns the transient error shown when a save or open fails, cleared
// whenever the shown message changes.
export function ReaderAttachments({message, attachments, onViewEmail}: ReaderAttachmentsProps) {
    const [attachError, setAttachError] = useState('')

    // Clear the error whenever the shown message changes, matching the reader's per-message reset.
    useEffect(() => {
        setAttachError('')
    }, [message.id])

    // saveAttachment writes a received attachment to disk through a native save dialog; its bytes come from
    // the locally cached body, so it works offline once the message has been opened.
    const saveAttachment = async (index: number) => {
        setAttachError('')
        try {
            await api.saveAttachment(message.id, index)
        } catch (e) {
            setAttachError(String(e))
        }
    }

    // openAttachment shows an attached .eml in the in-app viewer (so it never hands off to an external mail
    // client). Any other file opens with the OS default app after writing it to a temporary file.
    const openAttachment = async (index: number, filename: string) => {
        setAttachError('')
        try {
            if (filename.toLowerCase().endsWith('.eml')) {
                onViewEmail(await api.openEmailAttachment(message.id, index))
            } else {
                await api.openAttachment(message.id, index)
            }
        } catch (e) {
            setAttachError(String(e))
        }
    }

    // saveAllAttachments writes every attachment into a folder chosen through a native dialog, in one step.
    const saveAllAttachments = async () => {
        setAttachError('')
        try {
            await api.saveAllAttachments(message.id)
        } catch (e) {
            setAttachError(String(e))
        }
    }

    return (
        <div className="reader-attachments">
            <div className="reader-attachments-title">
                <span>{attachments.length === 1 ? '1 attachment' : `${attachments.length} attachments`}</span>
                {attachments.length > 1 && (
                    <button type="button" className="btn" onClick={() => void saveAllAttachments()}>
                        Save all
                    </button>
                )}
            </div>
            {attachError && <div className="compose-error">{attachError}</div>}
            <ul className="attachment-list">
                {attachments.map((att) => (
                    <li key={att.index} className="attachment-chip">
                        <span className="attachment-name" title={att.filename}>{att.filename}</span>
                        <span className="attachment-size">{formatBytes(att.size)}</span>
                        <button
                            type="button"
                            className="btn"
                            onClick={() => void openAttachment(att.index, att.filename)}
                        >
                            Open
                        </button>
                        <button
                            type="button"
                            className="btn"
                            onClick={() => void saveAttachment(att.index)}
                        >
                            Save
                        </button>
                    </li>
                ))}
            </ul>
        </div>
    )
}
