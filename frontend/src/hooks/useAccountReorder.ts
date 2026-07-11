import {useState} from 'react'
import type {DragEvent as ReactDragEvent} from 'react'
import {accountDragType, moveId} from '../sidebarDnd'

// useAccountReorder owns the account-list reorder drag: which row is being dragged (dragId) and which row it
// is currently over (dragOverId), both driving the row cue, plus the drag handlers for a row. A drop splices
// the dragged id to the target's position and reports the new order.
export function useAccountReorder(accountIds: string[], onReorderAccounts: (orderedIds: string[]) => void) {
    const [dragId, setDragId] = useState('')
    const [dragOverId, setDragOverId] = useState('')

    // rowDragProps returns the drag handlers to spread onto one account row.
    const rowDragProps = (accountId: string) => ({
        onDragStart: (e: ReactDragEvent<HTMLLIElement>) => {
            setDragId(accountId)
            e.dataTransfer.setData(accountDragType, accountId)
            e.dataTransfer.effectAllowed = 'move'
        },
        onDragEnd: () => {
            setDragId('')
            setDragOverId('')
        },
        onDragOver: (e: ReactDragEvent<HTMLLIElement>) => {
            if (e.dataTransfer.types.includes(accountDragType)) {
                e.preventDefault()
                e.dataTransfer.dropEffect = 'move'
                setDragOverId(accountId)
            }
        },
        onDragLeave: () => setDragOverId((id) => (id === accountId ? '' : id)),
        onDrop: (e: ReactDragEvent<HTMLLIElement>) => {
            e.preventDefault()
            const from = e.dataTransfer.getData(accountDragType)
            setDragId('')
            setDragOverId('')
            if (from && from !== accountId) {
                onReorderAccounts(moveId(accountIds, from, accountId))
            }
        },
    })

    return {dragId, dragOverId, rowDragProps}
}
