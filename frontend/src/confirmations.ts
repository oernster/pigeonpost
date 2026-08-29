// confirmations holds the wording of every destructive confirmation the main window raises, plus the
// shape a dialog needs to show one.
//
// The copy used to sit in App's JSX beside each dialog, which put App in charge of describing actions it
// does not perform: what deleting a message costs is the delete's own business, not the composition
// root's. Each action's hook now builds its own descriptor from these, so the sentence and the action it
// describes cannot drift apart; the words themselves are pure and testable rather than reachable
// only by opening a dialog.

// Confirmation is one pending destructive action: what it is called, what it will cost, the label on the
// button that does it and the two ways out.
export interface Confirmation {
    title: string
    message: string
    confirmLabel: string
    // busy is true while the action is running, so the dialog can hold its buttons.
    busy: boolean
    // defaultConfirm focuses the destructive button. It is set for the deletes, which are the ones a user
    // reaches repeatedly and expects to confirm with Enter.
    defaultConfirm?: boolean
    onConfirm: () => void
    onCancel: () => void
}

// ConfirmationActions is what an action supplies alongside its own facts: whether it is running and the
// two outcomes.
export interface ConfirmationActions {
    busy: boolean
    onConfirm: () => void
    onCancel: () => void
}

// named gives a message its subject in quotes, falling back to the wording the reader uses for a message
// that has none, so no confirmation ever says Delete "".
function named(subject: string): string {
    return `"${subject || '(no subject)'}"`
}

// TRASH_ROUTE and NO_TRASH_ROUTE are the two fates a deleted message can meet, stated once because the
// single and bulk deletes must not describe them differently.
const NO_TRASH_ROUTE = 'POP3 has no Trash, so'
const IRRECOVERABLE = 'removed from the server and cannot be recovered'

export function cancelSendConfirmation(subject: string, actions: ConfirmationActions): Confirmation {
    return {
        title: 'Cancel send',
        message: `Cancel sending ${named(subject)}? The queued email is discarded and will not be sent.`,
        confirmLabel: 'Cancel send',
        ...actions,
    }
}

export function deleteMessageConfirmation(
    subject: string,
    pop3: boolean,
    actions: ConfirmationActions,
): Confirmation {
    return {
        title: 'Delete message',
        message: pop3
            ? `Delete ${named(subject)}? ${NO_TRASH_ROUTE} it is permanently ${IRRECOVERABLE}.`
            : `Delete ${named(subject)}? It is moved to Trash; it is deleted permanently if it is already in Trash or the account has no Trash folder.`,
        confirmLabel: 'Delete',
        defaultConfirm: true,
        ...actions,
    }
}

export function purgeMessageConfirmation(subject: string, actions: ConfirmationActions): Confirmation {
    return {
        title: 'Delete permanently',
        message: `Permanently delete ${named(subject)}? It is ${IRRECOVERABLE}.`,
        confirmLabel: 'Delete permanently',
        defaultConfirm: true,
        ...actions,
    }
}

export function deleteManyConfirmation(
    count: number,
    pop3: boolean,
    actions: ConfirmationActions,
): Confirmation {
    return {
        title: 'Delete messages',
        message: pop3
            ? `Delete ${count} messages? ${NO_TRASH_ROUTE} they are permanently ${IRRECOVERABLE}.`
            : `Delete ${count} messages? They are moved to Trash; where the account has no Trash folder they are deleted permanently.`,
        confirmLabel: `Delete ${count}`,
        defaultConfirm: true,
        ...actions,
    }
}

export function purgeManyConfirmation(count: number, actions: ConfirmationActions): Confirmation {
    return {
        title: 'Delete permanently',
        message: `Permanently delete ${count} messages? They are ${IRRECOVERABLE}.`,
        confirmLabel: `Delete ${count} permanently`,
        defaultConfirm: true,
        ...actions,
    }
}

export function removeAccountConfirmation(email: string, actions: ConfirmationActions): Confirmation {
    return {
        title: 'Remove account',
        message: `Remove ${email}? Its cached mail is deleted from this device and its password is removed from the keychain. Mail on the server is not affected.`,
        confirmLabel: 'Remove account',
        ...actions,
    }
}

export function deleteFolderConfirmation(name: string, actions: ConfirmationActions): Confirmation {
    return {
        title: 'Delete folder',
        message: `Delete the folder "${name}" on the server? Its messages are removed from this device. This cannot be undone.`,
        confirmLabel: 'Delete folder',
        ...actions,
    }
}
