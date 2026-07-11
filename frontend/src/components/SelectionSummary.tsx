import {Dispatch, SetStateAction} from 'react'
import {Message} from '../api'

export interface SelectionSummaryProps {
    markedIds: Set<string>
    selectedMessages: Message[]
    setBulkToDelete: Dispatch<SetStateAction<Message[] | null>>
    bulkSetRead: (targets: Message[], read: boolean) => Promise<void>
    clearSelection: () => void
}

// SelectionSummary is the reader placeholder shown while more than one message is selected: the selection
// count and the bulk actions (delete, mark read, mark unread, clear). It replaces the single-message reader
// until the multi-selection is cleared.
export function SelectionSummary({markedIds, selectedMessages, setBulkToDelete, bulkSetRead, clearSelection}: SelectionSummaryProps) {
    return (
        <section className="pane reader">
            <div className="empty-state selection-summary">
                <p className="empty-body">{markedIds.size} messages selected</p>
                <div className="selection-actions">
                    <button className="btn danger" onClick={() => setBulkToDelete(selectedMessages)}>Delete</button>
                    <button className="btn" onClick={() => void bulkSetRead(selectedMessages, true)}>Mark read</button>
                    <button className="btn" onClick={() => void bulkSetRead(selectedMessages, false)}>Mark unread</button>
                    <button className="btn" onClick={clearSelection}>Clear selection</button>
                </div>
            </div>
        </section>
    )
}
