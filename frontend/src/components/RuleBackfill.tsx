import {useEffect, useRef, useState} from 'react'
import {api, Rule} from '../api'
// The rule types and the event name come from their own module rather than through the api re-export:
// api.ts is one of the modules the size guard is ratcheting down; a value export (the event name)
// would have to be a line of its own there rather than riding the existing type re-export.
import {RuleBackfill as Counts, RuleBackfillProgress, ruleBackfillProgressEvent} from '../apiRules'
import {EventsOn} from '../../wailsjs/runtime'
import {ConfirmDialog} from './ConfirmDialog'
import {ModalClose} from './ModalClose'

// backfillPhase is where a run has got to: idle, working (scanning or applying, with a bar), showing
// what it would do, reporting what it did.
type backfillPhase =
    | {kind: 'idle'}
    | {kind: 'working'; rule: Rule}
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

// PERCENT is the full scale of the bar, named so the arithmetic below carries no bare literal.
const PERCENT = 100

// barPercent is how full the bar should be. A phase with nothing to do reads as complete rather than
// as a division by zero; the value is clamped so a late reading can never overrun the track.
export function barPercent(progress: RuleBackfillProgress | null): number {
    if (progress === null || progress.total <= 0) {
        return PERCENT
    }
    return Math.min(PERCENT, Math.round((progress.done / progress.total) * PERCENT))
}

// progressLine says what is happening in words as well as in the bar, because a bar alone does not
// say WHAT is being counted; the two phases count different things.
export function progressLine(progress: RuleBackfillProgress | null): string {
    if (progress === null) {
        return 'Starting...'
    }
    if (progress.phase === 'scanning') {
        return progress.total === 0
            ? 'Looking for folders to check...'
            : `Checking folder ${Math.min(progress.done + 1, progress.total)} of ${progress.total}...`
    }
    if (progress.phase === 'applying') {
        return progress.total === 0
            ? 'Nothing to apply.'
            : `Applying to message ${Math.min(progress.done + 1, progress.total)} of ${progress.total}...`
    }
    return 'Finishing...'
}

interface RuleBackfillProps {
    // phase is the current state, owned by the caller so one dialog serves the whole rule list.
    phase: backfillPhase
    progress: RuleBackfillProgress | null
    busy: boolean
    onRun: (rule: Rule) => void
    onDismiss: () => void
}

// RuleBackfillDialogs renders the three states a backfill passes through: the progress dialog while it
// reads and acts, the confirmation before it acts and the report afterwards. A backfill runs over a
// whole backlog unattended, so the confirmation is the only point at which the user can be asked about
// it at all: it names the exact counts; it says plainly when a rule deletes mail outright.
export function RuleBackfillDialogs({phase, progress, busy, onRun, onDismiss}: RuleBackfillProps) {
    if (phase.kind === 'working') {
        const percent = barPercent(progress)
        return (
            <div className="modal-backdrop top">
                <div className="modal confirm pinned-actions" role="alertdialog" aria-label={`Applying ${phase.rule.name}`}>
                    <h2 className="modal-title">{`Applying "${phase.rule.name}"`}</h2>
                    <div className="modal-body">
                        <p className="confirm-message">{progressLine(progress)}</p>
                        <div
                            className="progress-track"
                            role="progressbar"
                            aria-valuemin={0}
                            aria-valuemax={PERCENT}
                            aria-valuenow={percent}
                            aria-label={progressLine(progress)}
                        >
                            <div className="progress-fill" style={{width: `${percent}%`}}/>
                        </div>
                    </div>
                </div>
            </div>
        )
    }
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
            <div className="modal-backdrop top">
                <div className="modal confirm pinned-actions" role="alertdialog" aria-label={`${phase.rule.name} applied`}>
                    <ModalClose onClose={onDismiss}/>
                    <h2 className="modal-title">{`"${phase.rule.name}" applied`}</h2>
                    <div className="modal-body">
                        <p className="confirm-message">{doneLine(phase.counts)}</p>
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

// useRuleBackfill owns the preview-then-run exchange for the rule list: it fetches what a rule would
// do, holds it for confirmation and reports the result. A rule that turns out to change nothing skips
// the confirmation and says so directly, since there is nothing to agree to.
//
// Both halves show the progress dialog, because both are slow on a real mailbox: the scan reads every
// folder of every account the rule covers, which is most of the wait before the confirmation appears.
export function useRuleBackfill(onChanged: () => void, onError: (message: string) => void) {
    const [phase, setPhase] = useState<backfillPhase>({kind: 'idle'})
    const [progress, setProgress] = useState<RuleBackfillProgress | null>(null)
    const [busy, setBusy] = useState(false)
    // The listener is registered once for the life of the component rather than per run: EventsOn's
    // unsubscribe is what stops it; binding it to a run would race the run's own first reading.
    const running = useRef(false)

    useEffect(() => {
        const off = EventsOn(ruleBackfillProgressEvent, (reading) => {
            if (running.current) {
                setProgress(reading as RuleBackfillProgress)
            }
        })
        return () => off()
    }, [])

    const start = async (rule: Rule) => {
        setBusy(true)
        setProgress(null)
        running.current = true
        setPhase({kind: 'working', rule})
        try {
            const counts = await api.previewRuleBackfill(rule.id)
            setPhase(acts(counts) ? {kind: 'confirm', rule, counts} : {kind: 'done', rule, counts})
        } catch (e) {
            setPhase({kind: 'idle'})
            onError(String(e))
        } finally {
            running.current = false
            setBusy(false)
        }
    }

    // run reports what actually happened even when part of it failed, because the counts come back
    // alongside the error: hiding them would leave the user unsure what had already been done.
    const run = async (rule: Rule) => {
        setBusy(true)
        setProgress(null)
        running.current = true
        setPhase({kind: 'working', rule})
        try {
            const counts = await api.runRuleBackfill(rule.id)
            setPhase({kind: 'done', rule, counts})
            onChanged()
        } catch (e) {
            setPhase({kind: 'idle'})
            onError(String(e))
            onChanged()
        } finally {
            running.current = false
            setBusy(false)
        }
    }

    return {
        phase,
        progress,
        busy,
        start: (rule: Rule) => void start(rule),
        run: (rule: Rule) => void run(rule),
        dismiss: () => setPhase({kind: 'idle'}),
    }
}
