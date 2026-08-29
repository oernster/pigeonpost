import {describe, expect, it, vi} from 'vitest'
import {
    cancelSendConfirmation,
    deleteFolderConfirmation,
    deleteManyConfirmation,
    deleteMessageConfirmation,
    purgeManyConfirmation,
    purgeMessageConfirmation,
    removeAccountConfirmation,
} from './confirmations'

// actions() is the half an action supplies: whether it is running and the two outcomes.
function actions(busy = false) {
    return {busy, onConfirm: vi.fn(), onCancel: vi.fn()}
}

describe('confirmations', () => {
    it('names a message with no subject rather than quoting an empty string', () => {
        expect(deleteMessageConfirmation('', false, actions()).message).toContain('"(no subject)"')
        expect(purgeMessageConfirmation('', actions()).message).toContain('"(no subject)"')
        expect(cancelSendConfirmation('', actions()).message).toContain('"(no subject)"')
    })

    it('tells a POP3 user their delete cannot be recovered', () => {
        const one = deleteMessageConfirmation('Report', true, actions())
        expect(one.message).toContain('POP3 has no Trash')
        expect(one.message).toContain('cannot be recovered')
        const many = deleteManyConfirmation(3, true, actions())
        expect(many.message).toContain('POP3 has no Trash')
        expect(many.message).toContain('cannot be recovered')
    })

    it('tells an IMAP user their delete goes to Trash', () => {
        expect(deleteMessageConfirmation('Report', false, actions()).message).toContain('moved to Trash')
        expect(deleteManyConfirmation(3, false, actions()).message).toContain('moved to Trash')
    })

    it('counts the messages a bulk action will act on, in its message and on its button', () => {
        const many = deleteManyConfirmation(4, false, actions())
        expect(many.message).toContain('4 messages')
        expect(many.confirmLabel).toBe('Delete 4')
        const purge = purgeManyConfirmation(4, actions())
        expect(purge.message).toContain('4 messages')
        expect(purge.confirmLabel).toBe('Delete 4 permanently')
    })

    it('defaults the button to the destructive action for the deletes only', () => {
        expect(deleteMessageConfirmation('a', false, actions()).defaultConfirm).toBe(true)
        expect(purgeMessageConfirmation('a', actions()).defaultConfirm).toBe(true)
        expect(deleteManyConfirmation(2, false, actions()).defaultConfirm).toBe(true)
        expect(purgeManyConfirmation(2, actions()).defaultConfirm).toBe(true)
        // These three are not reached repeatedly and are not defaulted, so Enter does not remove an
        // account or a folder, nor discard a queued message.
        expect(cancelSendConfirmation('a', actions()).defaultConfirm).toBeUndefined()
        expect(removeAccountConfirmation('a@b.c', actions()).defaultConfirm).toBeUndefined()
        expect(deleteFolderConfirmation('Archive', actions()).defaultConfirm).toBeUndefined()
    })

    it('says what removing an account does and does not touch', () => {
        const c = removeAccountConfirmation('jane@example.com', actions())
        expect(c.title).toBe('Remove account')
        expect(c.message).toContain('jane@example.com')
        expect(c.message).toContain('Mail on the server is not affected')
    })

    it('names the folder it is about to delete', () => {
        const c = deleteFolderConfirmation('Archive', actions())
        expect(c.title).toBe('Delete folder')
        expect(c.message).toContain('"Archive"')
        expect(c.message).toContain('cannot be undone')
    })

    it('carries the busy flag and both outcomes through unchanged', () => {
        const a = actions(true)
        const c = purgeMessageConfirmation('Report', a)
        expect(c.busy).toBe(true)
        c.onConfirm()
        expect(a.onConfirm).toHaveBeenCalled()
        c.onCancel()
        expect(a.onCancel).toHaveBeenCalled()
    })

    it('gives every confirmation a title, a message and a button label', () => {
        const all = [
            cancelSendConfirmation('a', actions()),
            deleteMessageConfirmation('a', false, actions()),
            purgeMessageConfirmation('a', actions()),
            deleteManyConfirmation(2, false, actions()),
            purgeManyConfirmation(2, actions()),
            removeAccountConfirmation('a@b.c', actions()),
            deleteFolderConfirmation('a', actions()),
        ]
        for (const c of all) {
            expect(c.title).not.toBe('')
            expect(c.message).not.toBe('')
            expect(c.confirmLabel).not.toBe('')
            // Every one of them ends its question with a full stop, so the dialogs read alike.
            expect(c.message.endsWith('.')).toBe(true)
        }
    })
})
