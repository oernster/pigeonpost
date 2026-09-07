import {useCallback, useState} from 'react'
import {api} from '../api'
import type {RuleImportPlan, RuleImportResult} from '../apiRules'
import {ConfirmDialog} from './ConfirmDialog'
import {ModalClose} from './ModalClose'

// transferPhase is where a file operation has got to: nothing open, describing what an import would
// do while waiting to be agreed to, then reporting what it did.
type transferPhase =
    | {kind: 'idle'}
    | {kind: 'confirm'; plan: RuleImportPlan}
    | {kind: 'done'; message: string}

// planLine says what the file would do, in the terms the reader cares about: how many rules arrive
// and how many replace one they already have.
export function planLine(plan: RuleImportPlan): string {
    const parts: string[] = []
    if (plan.add.length > 0) {
        parts.push(`${plan.add.length} new`)
    }
    if (plan.replace.length > 0) {
        parts.push(`${plan.replace.length} replacing a rule of the same name`)
    }
    const counts = parts.length > 0 ? parts.join(' and ') : 'nothing'
    return `${plan.file} holds ${plan.add.length + plan.replace.length} rule(s): ${counts}.`
}

// resultLine reports what the import did, naming the rules that arrived switched off. A rule is
// disabled when it cannot act here (its destination folder or every account it names is unknown).
// Saying which ones is the difference between a rule the user can go and fix and one that silently
// never runs.
export function resultLine(result: RuleImportResult): string {
    const parts: string[] = []
    if (result.added > 0) {
        parts.push(`${result.added} added`)
    }
    if (result.replaced > 0) {
        parts.push(`${result.replaced} replaced`)
    }
    const done = parts.length > 0 ? parts.join(', ') : 'Nothing to import'
    if (result.disabled.length === 0) {
        return `${done}.`
    }
    return `${done}. Switched off because they cannot run here: ${result.disabled.join(', ')}.`
}

// destructiveLine warns about the imported rules that move or destroy mail. A rule runs unattended, so
// agreeing to an import is agreeing to whatever those rules do on the next sync.
export function destructiveLine(plan: RuleImportPlan): string {
    if (plan.destructive.length === 0) {
        return ''
    }
    return `${plan.destructive.length} of them move or delete mail: ${plan.destructive.join(', ')}.`
}

// importMessage is the whole confirmation: what the file holds, what it would replace, what arrives
// switched off, then what moves or deletes mail last of all, so that line sits nearest the button.
export function importMessage(plan: RuleImportPlan): string {
    const lines = [planLine(plan)]
    if (plan.replace.length > 0) {
        lines.push('A rule of the same name is replaced rather than duplicated.')
    }
    if (plan.disable.length > 0) {
        lines.push(`Arriving switched off, because they cannot run here: ${plan.disable.join(', ')}.`)
    }
    const destructive = destructiveLine(plan)
    if (destructive !== '') {
        lines.push(destructive)
    }
    return lines.join(' ')
}

// RuleTransfer is the export and import surface, kept out of the rules dialog so that dialog stays
// within the module-size limit and so the file handling reads in one place.
export interface RuleTransfer {
    phase: transferPhase
    busy: boolean
    exportRules: () => Promise<void>
    beginImport: () => Promise<void>
    confirmImport: () => Promise<void>
    dismiss: () => void
}

// useRuleTransfer owns the two file operations. Every failure goes to the dialog's own error sink
// rather than a thrown promise, since a cancelled file dialog and a bad file are both ordinary.
export function useRuleTransfer(onChanged: () => void, setError: (message: string) => void): RuleTransfer {
    const [phase, setPhase] = useState<transferPhase>({kind: 'idle'})
    const [busy, setBusy] = useState(false)

    const exportRules = useCallback(async () => {
        setError('')
        setBusy(true)
        try {
            const written = await api.exportRules()
            if (written) {
                setPhase({kind: 'done', message: 'Rules exported.'})
            }
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }, [setError])

    const beginImport = useCallback(async () => {
        setError('')
        setBusy(true)
        try {
            const plan = await api.previewRuleImport()
            if (plan.cancelled) {
                return
            }
            setPhase({kind: 'confirm', plan})
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }, [setError])

    const confirmImport = useCallback(async () => {
        if (phase.kind !== 'confirm') {
            return
        }
        setBusy(true)
        try {
            const result = await api.applyRuleImport(phase.plan.path)
            setPhase({kind: 'done', message: resultLine(result)})
            onChanged()
        } catch (e) {
            setPhase({kind: 'idle'})
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }, [phase, onChanged, setError])

    const dismiss = useCallback(() => setPhase({kind: 'idle'}), [])

    return {phase, busy, exportRules, beginImport, confirmImport, dismiss}
}

interface RuleTransferDialogsProps {
    phase: transferPhase
    busy: boolean
    onConfirm: () => void
    onDismiss: () => void
}

// RuleTransferDialogs renders the confirmation before an import and the report after either operation.
export function RuleTransferDialogs({phase, busy, onConfirm, onDismiss}: RuleTransferDialogsProps) {
    if (phase.kind === 'confirm') {
        return (
            <ConfirmDialog
                title="Import rules"
                message={importMessage(phase.plan)}
                confirmLabel="Import rules"
                busy={busy}
                busyLabel="Importing..."
                onConfirm={onConfirm}
                onCancel={onDismiss}
            />
        )
    }
    if (phase.kind === 'done') {
        return (
            <div className="modal-backdrop top">
                <div className="modal confirm pinned-actions" role="alertdialog" aria-label="Rules">
                    <ModalClose onClose={onDismiss}/>
                    <h2 className="modal-title">Rules</h2>
                    <div className="modal-body">
                        <p className="confirm-message">{phase.message}</p>
                    </div>
                    <div className="modal-actions">
                        <button className="btn primary" onClick={onDismiss} autoFocus>Close</button>
                    </div>
                </div>
            </div>
        )
    }
    return null
}
