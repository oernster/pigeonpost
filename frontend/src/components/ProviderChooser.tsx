import {useBackdropDismiss} from './useBackdropDismiss'
import {ModalClose} from './ModalClose'
import {PROVIDERS, type Provider} from '../accountProviders'

interface ProviderChooserProps {
    error: string
    // busy disables every choice while a Microsoft sign-in is in flight.
    busy: boolean
    onClose: () => void
    onChooseMicrosoft: () => void
    onChooseProvider: (provider: Provider) => void
    onChooseManual: () => void
}

// ProviderChooser is the first step of adding an account: a grid of known providers (their servers already
// pre-filled), Microsoft (which signs in through OAuth) and a manual option for any other provider.
export function ProviderChooser({
    error, busy, onClose, onChooseMicrosoft, onChooseProvider, onChooseManual,
}: ProviderChooserProps) {
    const dismiss = useBackdropDismiss(onClose)
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal setup" role="dialog" aria-label="Add account" onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">Add account</h2>
                <p className="setup-hint">Choose your email provider, or set the servers up yourself.</p>
                {error && <div className="compose-error">{error}</div>}
                <div className="provider-grid">
                    <button className="provider-btn" onClick={onChooseMicrosoft} disabled={busy}>
                        Microsoft
                    </button>
                    {PROVIDERS.map((p) => (
                        <button key={p.id} className="provider-btn" onClick={() => onChooseProvider(p)} disabled={busy}>
                            {p.name}
                        </button>
                    ))}
                </div>
                <button className="btn manual-btn" onClick={onChooseManual} disabled={busy}>Set up manually (other provider)</button>
                <div className="modal-actions">
                    <button className="btn" onClick={onClose} disabled={busy}>Cancel</button>
                </div>
            </div>
        </div>
    )
}
