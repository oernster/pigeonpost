import {basename} from '../composeAddresses'
import type {MessageAttachment} from './ComposeModal'
import type {useComposeIntake} from '../hooks/useComposeIntake'

interface ComposeAttachmentsProps {
    attachments: string[]
    messageAttachments: MessageAttachment[]
    intake: ReturnType<typeof useComposeIntake>
    hasAttachments: () => boolean
    addAttachments: () => Promise<void>
    removeAttachment: (path: string) => void
    removeMessageAttachment: (id: string) => void
}

// ComposeAttachments is the attach button and the chips for everything the message carries: files picked by
// path, files pasted or dropped and attached emails. Each chip removes its own.
export function ComposeAttachments({
    attachments, messageAttachments, intake, hasAttachments, addAttachments, removeAttachment, removeMessageAttachment,
}: ComposeAttachmentsProps) {
    return (
        <div className="compose-attachments">
            <button type="button" className="btn" onClick={() => void addAttachments()}>
                Attach files
            </button>
            {hasAttachments() && (
                <ul className="attachment-list">
                    {attachments.map((path) => (
                        <li key={path} className="attachment-chip">
                            <span className="attachment-name" title={path}>{basename(path)}</span>
                            <button
                                type="button"
                                className="attachment-remove"
                                aria-label={`Remove ${basename(path)}`}
                                onClick={() => removeAttachment(path)}
                            >
                                &times;
                            </button>
                        </li>
                    ))}
                    {intake.dataAttachments.map((a, index) => (
                        <li key={`${index}-${a.name}`} className="attachment-chip">
                            <span className="attachment-name" title={a.name}>{a.name}</span>
                            <button
                                type="button"
                                className="attachment-remove"
                                aria-label={`Remove ${a.name}`}
                                onClick={() => intake.remove(index)}
                            >
                                &times;
                            </button>
                        </li>
                    ))}
                    {messageAttachments.map((m) => (
                        <li key={m.id} className="attachment-chip">
                            <span className="attachment-icon" aria-hidden="true">{'✉'}</span>
                            <span className="attachment-name" title={m.name}>{m.name}</span>
                            <button
                                type="button"
                                className="attachment-remove"
                                aria-label={`Remove ${m.name}`}
                                onClick={() => removeMessageAttachment(m.id)}
                            >
                                &times;
                            </button>
                        </li>
                    ))}
                </ul>
            )}
        </div>
    )
}
