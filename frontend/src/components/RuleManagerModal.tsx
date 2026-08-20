import {useEffect, useMemo, useState} from 'react'
import {useBackdropDismiss} from './useBackdropDismiss'
import {api, Account, Folder, Rule, RuleInput} from '../api'
import {ModalClose} from './ModalClose'
import {ConfirmDialog} from './ConfirmDialog'
import {RuleEditor, FolderChoice, AccountChoice} from './RuleEditor'
import {destroys, emptyRule, isDestructive, ruleIsComplete, ruleSummary} from './ruleLabels'

interface RuleManagerModalProps {
    accounts: Account[]
    rules: Rule[]
    onChanged: () => void
    onClose: () => void
}

// RuleManagerModal lists the filter rules in the order they run and edits one at a time. Rules run on
// mail arriving in the Inbox: a message matching a rule's conditions has every one of its actions
// applied; the order shown here is the order they are tried in.
export function RuleManagerModal({accounts, rules, onChanged, onClose}: RuleManagerModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    const [draft, setDraft] = useState<Rule | null>(null)
    const [pendingConfirm, setPendingConfirm] = useState<Rule | null>(null)
    const [toDelete, setToDelete] = useState<Rule | null>(null)
    const [folders, setFolders] = useState<Folder[]>([])
    const [error, setError] = useState('')
    const [busy, setBusy] = useState(false)

    // Every account's folders are loaded once, so a move action can name any of them. An account that
    // cannot be read contributes nothing rather than failing the dialog.
    useEffect(() => {
        let cancelled = false
        const load = async () => {
            const lists = await Promise.all(
                accounts.map((a) => api.listFolders(a.id).catch((): Folder[] => [])),
            )
            if (!cancelled) {
                setFolders(lists.flat())
            }
        }
        void load()
        return () => {
            cancelled = true
        }
    }, [accounts])

    const accountName = (id: string) => accounts.find((a) => a.id === id)?.displayName ?? 'a removed account'

    // folderChoices labels each destination with its account, because a move names one concrete folder
    // and so only applies to mail arriving on that account.
    const folderChoices = useMemo<FolderChoice[]>(
        () => folders.map((f) => ({
            id: f.id,
            accountId: f.accountId,
            label: `${accounts.find((a) => a.id === f.accountId)?.displayName ?? 'Account'} / ${f.path}`,
        })),
        [accounts, folders],
    )

    const accountChoices = useMemo<AccountChoice[]>(
        () => accounts.map((a) => ({id: a.id, label: a.displayName})),
        [accounts],
    )

    const folderName = (folderId: string) =>
        folderChoices.find((f) => f.id === folderId)?.label ?? 'a folder'

    const run = async (action: () => Promise<void>) => {
        setBusy(true)
        setError('')
        try {
            await action()
            onChanged()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    // save writes the draft. A rule that moves or destroys mail is confirmed first: it runs unattended,
    // so this is the only point at which the user can be asked about it at all.
    const save = (rule: Rule, confirmed: boolean) => {
        if (isDestructive(rule) && !confirmed) {
            setPendingConfirm(rule)
            return
        }
        setPendingConfirm(null)
        void run(async () => {
            await api.saveRule(rule as RuleInput)
            setDraft(null)
        })
    }

    const remove = (rule: Rule) =>
        void run(async () => {
            await api.deleteRule(rule.id)
            setToDelete(null)
        })

    const toggleEnabled = (rule: Rule) =>
        void run(() => api.saveRule({...rule, enabled: !rule.enabled} as RuleInput))

    // move shifts a rule up or down the running order and writes the whole order back, so the positions
    // stay contiguous however they were edited before.
    const move = (index: number, delta: number) => {
        const target = index + delta
        if (target < 0 || target >= rules.length) {
            return
        }
        const order = rules.map((r) => r.id)
        ;[order[index], order[target]] = [order[target], order[index]]
        void run(() => api.reorderRules(order))
    }

    if (draft) {
        return (
            <>
                <div className="modal-backdrop" {...dismiss}>
                    <div className="modal rule-modal pinned-actions" role="dialog" aria-label="Edit filter rule" onClick={(e) => e.stopPropagation()}>
                        <ModalClose onClose={() => setDraft(null)}/>
                        <h2 className="modal-title">{draft.id === '' ? 'New rule' : 'Edit rule'}</h2>
                        <div className="modal-body">
                            {error && <div className="compose-error">{error}</div>}
                            <RuleEditor
                                rule={draft}
                                folders={folderChoices}
                                accounts={accountChoices}
                                onChange={setDraft}
                            />
                        </div>
                        <div className="modal-actions spread">
                            <button className="btn" onClick={() => setDraft(null)}>Cancel</button>
                            <button
                                className={destroys(draft) ? 'btn danger' : 'btn primary'}
                                onClick={() => save(draft, false)}
                                disabled={busy || !ruleIsComplete(draft)}
                            >
                                {busy ? 'Saving...' : 'Save rule'}
                            </button>
                        </div>
                    </div>
                </div>
                {pendingConfirm && (
                    <ConfirmDialog
                        title={destroys(pendingConfirm) ? 'This rule destroys mail' : 'This rule moves mail'}
                        message={
                            destroys(pendingConfirm)
                                ? 'Mail matching this rule will be deleted from the server as it arrives. ' +
                                  'It does not go to Trash, no copy is kept and you will never see it. ' +
                                  'Rules run unattended, so you will not be asked again.'
                                : 'Mail matching this rule will be moved out of the Inbox as it arrives, ' +
                                  'without being shown there first. Rules run unattended, so you will not be asked again.'
                        }
                        confirmLabel={destroys(pendingConfirm) ? 'Save destroying rule' : 'Save rule'}
                        busy={busy}
                        onConfirm={() => save(pendingConfirm, true)}
                        onCancel={() => setPendingConfirm(null)}
                    />
                )}
            </>
        )
    }

    return (
        <>
            <div className="modal-backdrop" {...dismiss}>
                <div className="modal rule-modal pinned-actions" role="dialog" aria-label="Filter rules" onClick={(e) => e.stopPropagation()}>
                    <ModalClose onClose={onClose}/>
                    <h2 className="modal-title">Filter rules</h2>
                    <div className="modal-body">
                        <p className="setup-hint">
                            Rules run on mail arriving in the Inbox, in the order shown. They never act on mail
                            already in your mailbox.
                        </p>
                        {error && <div className="compose-error">{error}</div>}
                        {rules.length === 0 ? (
                            <p className="empty-body">No rules yet.</p>
                        ) : (
                            <ul className="list rule-list">
                                {rules.map((r, index) => {
                                    const summary = ruleSummary(r, folderName, accountName)
                                    return (
                                        <li key={r.id} className={`rule-row${r.enabled ? '' : ' off'}`}>
                                            <span className="rule-order">
                                                <button
                                                    aria-label={`Move ${r.name} up`}
                                                    title="Run this rule earlier"
                                                    disabled={busy || index === 0}
                                                    onClick={() => move(index, -1)}
                                                >
                                                    &#9650;
                                                </button>
                                                <button
                                                    aria-label={`Move ${r.name} down`}
                                                    title="Run this rule later"
                                                    disabled={busy || index === rules.length - 1}
                                                    onClick={() => move(index, 1)}
                                                >
                                                    &#9660;
                                                </button>
                                            </span>
                                            <span className="item-text">
                                                <span className="item-title" title={r.name}>
                                                    {r.name}
                                                    {destroys(r) && <span className="rule-badge">destroys</span>}
                                                </span>
                                                <span className="item-sub" title={summary}>{summary}</span>
                                            </span>
                                            <button
                                                className={`rule-toggle${r.enabled ? ' on' : ''}`}
                                                aria-label={r.enabled ? `Disable ${r.name}` : `Enable ${r.name}`}
                                                title={r.enabled ? 'Disable this rule' : 'Enable this rule'}
                                                disabled={busy}
                                                onClick={() => toggleEnabled(r)}
                                            >
                                                {r.enabled ? 'On' : 'Off'}
                                            </button>
                                            <button
                                                className="account-action"
                                                aria-label={`Edit ${r.name}`}
                                                title="Edit rule"
                                                disabled={busy}
                                                onClick={() => setDraft(r)}
                                            >
                                                Edit
                                            </button>
                                            <button
                                                className="rule-remove"
                                                aria-label={`Delete ${r.name}`}
                                                title="Delete rule"
                                                disabled={busy}
                                                onClick={() => setToDelete(r)}
                                            >
                                                &times;
                                            </button>
                                        </li>
                                    )
                                })}
                            </ul>
                        )}
                    </div>
                    <div className="modal-actions spread">
                        <button className="btn" onClick={onClose}>Close</button>
                        <button className="btn primary" onClick={() => setDraft(emptyRule(rules.length))}>
                            New rule
                        </button>
                    </div>
                </div>
            </div>
            {toDelete && (
                <ConfirmDialog
                    title="Delete rule"
                    message={`"${toDelete.name}" will be removed. Mail it already acted on is not affected.`}
                    confirmLabel="Delete rule"
                    busy={busy}
                    onConfirm={() => remove(toDelete)}
                    onCancel={() => setToDelete(null)}
                />
            )}
        </>
    )
}
