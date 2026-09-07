import {useState} from 'react'
import {api, Rule, RuleBackfill as Counts} from '../api'
import {ConfirmDialog} from './ConfirmDialog'

// backfillPhase is where a run has got to: idle, showing what it would do, reporting what it did.
type backfillPhase =
    | {kind: 'idle'}
    | {kind: 'confirm'; rule: Rule; counts: Counts}
    | {kind: 'done'; rule: Rule; counts: Counts}

// countLine turns the counts into a sentence naming each kind of work, so the user confirms against
// real numbers rather than a promise. Only the actions with work in them are named: a rule that just
// marks mail read should not read as though it might also be deleting some.
function countLine(counts: Counts): string {
    const parts: string[] = []
    if (counts.markRead > 0) {
        parts.push(`${counts.markRead} to mark as read`)
    }
    if (counts.flag > 0) {
        parts.push(`${counts.flag} to flag`)
    }
    if (counts.move > 0) {
        parts.push(`${counts.move} to move`)
    }
    if (counts.destroy > 0) {
        parts.push(`${counts.destroy} to delete permanently`)
    }
    return parts.join(', ')
}

// doneLine is countLine in the past tense, for the report after a run.
function doneLine(counts: Counts): string {
    const parts: string[] = []
    if (counts.markRead > 0) {
        parts.push(`${counts.markRead} marked as read`)
    }
    if (counts.flag > 0) {
        parts.push(`${counts.flag} flagged`)
    }
    if (counts.move > 0) {
        parts.push(`${counts.move} moved`)
    }
    if (counts.destroy > 0) {
        parts.push(`${counts.destroy} deleted permanently`)
    }
    return parts.length === 0 ? 'Nothing changed.' : `${parts.join(', ')}.`
}

// acts reports whether there is any work in the counts.
function acts(counts: Counts): boolean {
    return counts.markRead > 0 || counts.flag > 0 || counts.move > 0 || counts.destroy > 0
}

interface RuleBackfillProps {
    // phase is the current state, owned by the caller so one dialog serves the whole rule list.
    phase: backfillPhase
    busy: boolean
    onRun: (rule: Rule) => void
    onDismiss: () => void
}

// RuleBackfillDialogs renders the confirmation shown before a rule is applied to mail already stored,
// and the report shown afterwards. A backfill runs over a whole backlog unattended, so this dialog is
// the only point at which the user can be asked about it at all: it names the exact counts; it says
// plainly when a rule deletes mail outright.
export function RuleBackfillDialogs({phase, busy, onRun, onDismiss}: RuleBackfillProps) {
    if (phase.kind === 'confirm') {
        const destroys = phase.counts.destroy > 0
        const scope =
            `"${phase.rule.name}" was checked against ${phase.counts.scanned} stored message(s) ` +
            `across every folder of the accounts it covers: ${countLine(phase.counts)}. `
        const warning = destroys
            ? 'Deleted mail is removed from the server outright: it does not go to Trash and no copy is kept. ' +
              'This cannot be undone.'
            : 'This runs over mail you have already filed, not just new arrivals.'
        return (
            <ConfirmDialog
                title={destroys ? 'This will delete stored mail' : 'Apply this rule to stored mail'}
                message={scope + warning}
                confirmLabel={destroys ? 'Apply and delete' : 'Apply rule now'}
                busyLabel="Applying..."
                busy={busy}
                onConfirm={() => onRun(phase.rule)}
                onCancel={onDismiss}
            />
        )
    }
    if (phase.kind === 'done') {
        return (
            <ConfirmDialog
                title={`"${phase.rule.name}" applied`}
                message={doneLine(phase.counts)}
                confirmLabel="Close"
                busy={false}
                onConfirm={onDismiss}
                onCancel={onDismiss}
            />
        )
    }
    return null
}

// useRuleBackfill owns the preview-then-run exchange for the rule list: it fetches what a rule would
// do, holds it for confirmation and reports the result. A rule that turns out to change nothing skips
// the confirmation and says so directly, since there is nothing to agree to.
export function useRuleBackfill(onChanged: () => void, onError: (message: string) => void) {
    const [phase, setPhase] = useState<backfillPhase>({kind: 'idle'})
    const [busy, setBusy] = useState(false)

    const start = async (rule: Rule) => {
        setBusy(true)
        try {
            const counts = await api.previewRuleBackfill(rule.id)
            setPhase(acts(counts) ? {kind: 'confirm', rule, counts} : {kind: 'done', rule, counts})
        } catch (e) {
            onError(String(e))
        } finally {
            setBusy(false)
        }
    }

    // run reports what actually happened even when part of it failed, because the counts come back
    // alongside the error: hiding them would leave the user unsure what had already been done.
    const run = async (rule: Rule) => {
        setBusy(true)
        try {
            const counts = await api.runRuleBackfill(rule.id)
            setPhase({kind: 'done', rule, counts})
            onChanged()
        } catch (e) {
            setPhase({kind: 'idle'})
            onError(String(e))
            onChanged()
        } finally {
            setBusy(false)
        }
    }

    return {
        phase,
        busy,
        start: (rule: Rule) => void start(rule),
        run: (rule: Rule) => void run(rule),
        dismiss: () => setPhase({kind: 'idle'}),
    }
}
