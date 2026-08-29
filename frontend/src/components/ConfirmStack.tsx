import type {Confirmation} from '../confirmations'
import {ConfirmDialog} from './ConfirmDialog'

// ConfirmStack renders the destructive confirmations the main window owns. Each is built by the hook that
// owns the action it confirms, so this takes a list of pending ones rather than a prop per dialog: adding
// a destructive action means returning a confirmation from its own hook and adding it to the list App
// passes, with nothing new threaded through here.
//
// Every entry is rendered rather than only the first, which is what the separate blocks did before: the
// gates are independent, so two pending at once would have shown both; this must not quietly become
// one. In practice they are mutually exclusive; the point is that the change is not silent if they stop
// being so.
interface ConfirmStackProps {
    // confirmations may hold nulls, since a hook returns null when nothing of its kind is pending.
    confirmations: (Confirmation | null)[]
}

export function ConfirmStack({confirmations}: ConfirmStackProps) {
    return (
        <>
            {confirmations.map((confirmation, index) => confirmation && (
                <ConfirmDialog
                    // The list is fixed in length and order (one slot per kind of confirmation), so the
                    // index identifies the slot rather than standing in for a missing identity.
                    key={index}
                    title={confirmation.title}
                    message={confirmation.message}
                    confirmLabel={confirmation.confirmLabel}
                    busy={confirmation.busy}
                    defaultConfirm={confirmation.defaultConfirm}
                    onConfirm={confirmation.onConfirm}
                    onCancel={confirmation.onCancel}
                />
            ))}
        </>
    )
}
