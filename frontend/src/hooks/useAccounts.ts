import {Dispatch, SetStateAction, useCallback, useEffect, useState} from 'react'
import {Account, Folder, api} from '../api'
import type {MessageStore} from './useMessageStore'
import {Confirmation, removeAccountConfirmation} from '../confirmations'

// AccountsDeps is what the account list needs from the rest of App: the selected account and its setter
// (removeAccount resets the view when the open account is the one deleted), the message store and the two
// folder setters (both the list and the open folder are cleared on that removal) and the error sink.
export interface AccountsDeps {
    selectedAccount: string
    setSelectedAccount: Dispatch<SetStateAction<string>>
    store: MessageStore
    setFolders: Dispatch<SetStateAction<Folder[]>>
    setSelectedFolder: Dispatch<SetStateAction<string>>
    setError: (message: string) => void
}

export interface Accounts {
    // confirmation is the pending destructive confirmation this hook owns; null when none is pending.
    // It is built here rather than by the caller, so the wording and the action it describes stay together.
    confirmation: Confirmation | null
    accounts: Account[]
    settingUp: boolean
    setSettingUp: Dispatch<SetStateAction<boolean>>
    accountToEdit: Account | null
    setAccountToEdit: Dispatch<SetStateAction<Account | null>>
    accountToDelete: Account | null
    setAccountToDelete: Dispatch<SetStateAction<Account | null>>
    deleting: boolean
    loadAccounts: () => Promise<void>
    removeAccount: () => Promise<void>
}

// useAccounts owns the account list, the add/edit/remove dialog state, plus the load and remove
// operations. Account SELECTION (selectAccount) stays in App: it cascades into the folder list, the message
// store, the selection and the reader; it also needs loadFolderMessages, so it is a composition-root
// coordinator rather than a leaf of this hook. selectedAccount likewise stays App state, because useFolders
// and useOutbox read it before this hook is set up.
export function useAccounts(deps: AccountsDeps): Accounts {
    const {selectedAccount, setSelectedAccount, store, setFolders, setSelectedFolder, setError} = deps
    const {setMessages, setSelectedMessage} = store

    const [accounts, setAccounts] = useState<Account[]>([])
    const [settingUp, setSettingUp] = useState<boolean>(false)
    const [accountToEdit, setAccountToEdit] = useState<Account | null>(null)
    const [accountToDelete, setAccountToDelete] = useState<Account | null>(null)
    const [deleting, setDeleting] = useState<boolean>(false)

    const loadAccounts = useCallback(async () => {
        try {
            setAccounts(await api.listAccounts())
        } catch (e) {
            setError(String(e))
        }
    }, [])

    useEffect(() => {
        void loadAccounts()
    }, [loadAccounts])

    const removeAccount = useCallback(async () => {
        if (!accountToDelete) {
            return
        }
        setDeleting(true)
        setError('')
        try {
            await api.removeAccount(accountToDelete.id)
            if (accountToDelete.id === selectedAccount) {
                setSelectedAccount('')
                setFolders([])
                setSelectedFolder('')
                setMessages([])
                setSelectedMessage(null)
            }
            await loadAccounts()
            setAccountToDelete(null)
        } catch (e) {
            setError(String(e))
        } finally {
            setDeleting(false)
        }
    }, [accountToDelete, selectedAccount, loadAccounts])

    // The confirmation for removing an account, built beside the removal it describes.
    const confirmation = accountToDelete === null ? null : removeAccountConfirmation(
        accountToDelete.email,
        {
            busy: deleting,
            onConfirm: () => void removeAccount(),
            onCancel: () => setAccountToDelete(null),
        },
    )

    return {
        accounts,
        settingUp, setSettingUp,
        accountToEdit, setAccountToEdit,
        accountToDelete, setAccountToDelete, deleting,
        loadAccounts, removeAccount, confirmation,
    }
}
