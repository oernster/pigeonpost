import type {Account} from '../api'
import {useAccountForm} from '../hooks/useAccountForm'
import {ProviderChooser} from './ProviderChooser'
import {AccountDetailsForm} from './AccountDetailsForm'

interface AccountSetupModalProps {
    account?: Account | null
    onClose: () => void
    onSaved: (email: string) => void
}

// AccountSetupModal adds or edits a mail account. It is a thin orchestrator: useAccountForm holds the whole
// form, and this renders either the provider chooser (when adding) or the details form.
export function AccountSetupModal({account, onClose, onSaved}: AccountSetupModalProps) {
    const form = useAccountForm(account, onSaved)
    if (form.step === 'provider') {
        return (
            <ProviderChooser
                error={form.error}
                busy={form.msSigningIn}
                onClose={onClose}
                onChooseMicrosoft={form.chooseMicrosoft}
                onChooseProvider={form.chooseProvider}
                onChooseManual={form.chooseManual}
            />
        )
    }
    return <AccountDetailsForm form={form} onClose={onClose}/>
}
