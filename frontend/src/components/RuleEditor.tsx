import {Rule, RuleCondition, RuleAction} from '../api'
import {
    ACTION_LABELS,
    EMPTY_ACTION,
    EMPTY_CONDITION,
    FIELD_LABELS,
    OPERATOR_LABELS,
} from './ruleLabels'

// FolderChoice is one destination a move action can name: a concrete folder in one account, labelled
// with its account so it is clear a move is account-specific.
export interface FolderChoice {
    id: string
    label: string
    accountId: string
}

// AccountChoice is one account the rule can be limited to.
export interface AccountChoice {
    id: string
    label: string
}

interface RuleEditorProps {
    rule: Rule
    folders: FolderChoice[]
    accounts: AccountChoice[]
    onChange: (rule: Rule) => void
}

// replaceAt returns the list with one entry swapped, the shape every edit here takes.
function replaceAt<T>(list: T[], index: number, value: T): T[] {
    return list.map((item, i) => (i === index ? value : item))
}

// RuleEditor builds one rule: its name, how its conditions combine, the conditions themselves and the
// actions taken when they match. It holds no state of its own; the modal owns the draft and this
// renders it, so there is one place a rule can be in an inconsistent state.
//
// The layout is three labelled sections rather than one run of rows, because a rule reads as a
// sentence (when this matches, do that, with these caveats) and the sections are that sentence's
// clauses. Each condition and action is its own card so a long rule stays scannable.
//
// The remove control is absent, not disabled, on the last remaining condition or action: a rule needs
// at least one of each, so a cross there could never be clicked; a control that is permanently inert
// reads as broken rather than as unavailable.
export function RuleEditor({rule, folders, accounts, onChange}: RuleEditorProps) {
    const setConditions = (conditions: RuleCondition[]) => onChange({...rule, conditions} as Rule)
    const setActions = (actions: RuleAction[]) => onChange({...rule, actions} as Rule)
    const destroying = rule.actions.some((a) => a.kind === 'destroy')

    // A move names one concrete folder, so only folders the rule can actually reach are offered: an
    // account-scoped rule cannot move mail into an account it never sees.
    const reachable = rule.accountIds.length === 0
        ? folders
        : folders.filter((f) => rule.accountIds.includes(f.accountId))

    // Narrowing the scope can strand a destination in an account the rule no longer covers, which would
    // leave a move that silently does nothing. Any such destination is cleared with the same edit, so
    // the rule cannot be saved half-valid: the Save button then blocks on the empty folder.
    const setAccountIds = (accountIds: string[]) => {
        const allowed = accountIds.length === 0
            ? folders
            : folders.filter((f) => accountIds.includes(f.accountId))
        const allowedIds = new Set(allowed.map((f) => f.id))
        const actions = rule.actions.map((a) =>
            a.kind === 'moveTo' && a.folderId !== '' && !allowedIds.has(a.folderId)
                ? {...a, folderId: ''}
                : a,
        )
        onChange({...rule, accountIds, actions} as Rule)
    }

    const toggleAccount = (id: string) =>
        setAccountIds(
            rule.accountIds.includes(id)
                ? rule.accountIds.filter((a) => a !== id)
                : [...rule.accountIds, id],
        )

    return (
        <div className="rule-editor">
            <section className="rule-section">
                <label className="rule-label" htmlFor="rule-name">Rule name</label>
                <input
                    id="rule-name"
                    className="tag-name-input rule-name-input"
                    placeholder="Give the rule a name you will recognise"
                    value={rule.name}
                    autoFocus
                    onChange={(e) => onChange({...rule, name: e.target.value} as Rule)}
                />
            </section>

            <section className="rule-section">
                <div className="rule-section-head">
                    <h3 className="rule-section-title">Applies to</h3>
                    <span className="rule-hint">
                        {rule.accountIds.length === 0
                            ? 'Every account, including any you add later'
                            : `${rule.accountIds.length} of ${accounts.length} accounts`}
                    </span>
                </div>
                <div className="rule-accounts">
                    <button
                        className={`rule-chip${rule.accountIds.length === 0 ? ' on' : ''}`}
                        aria-pressed={rule.accountIds.length === 0}
                        onClick={() => setAccountIds([])}
                    >
                        All accounts
                    </button>
                    {accounts.map((a) => (
                        <button
                            key={a.id}
                            className={`rule-chip${rule.accountIds.includes(a.id) ? ' on' : ''}`}
                            aria-pressed={rule.accountIds.includes(a.id)}
                            onClick={() => toggleAccount(a.id)}
                        >
                            {a.label}
                        </button>
                    ))}
                </div>
            </section>

            <section className="rule-section">
                <div className="rule-section-head">
                    <h3 className="rule-section-title">When a message matches</h3>
                    <select
                        className="rule-mode"
                        aria-label="Match mode"
                        value={rule.matchMode}
                        onChange={(e) => onChange({...rule, matchMode: e.target.value} as Rule)}
                    >
                        <option value="any">any of these apply</option>
                        <option value="all">all of these apply</option>
                    </select>
                </div>

                {rule.conditions.map((condition, index) => (
                    <div className="rule-card" key={`condition-${index}`}>
                        <select
                            className="rule-field"
                            aria-label={`Field ${index + 1}`}
                            value={condition.field}
                            onChange={(e) =>
                                setConditions(replaceAt(rule.conditions, index, {...condition, field: e.target.value}))
                            }
                        >
                            {Object.entries(FIELD_LABELS).map(([value, label]) => (
                                <option key={value} value={value}>{label}</option>
                            ))}
                        </select>
                        <select
                            className="rule-operator"
                            aria-label={`Operator ${index + 1}`}
                            value={condition.operator}
                            onChange={(e) =>
                                setConditions(replaceAt(rule.conditions, index, {...condition, operator: e.target.value}))
                            }
                        >
                            {Object.entries(OPERATOR_LABELS).map(([value, label]) => (
                                <option key={value} value={value}>{label}</option>
                            ))}
                        </select>
                        <input
                            className="tag-name-input rule-text"
                            placeholder="text to match"
                            aria-label={`Match text ${index + 1}`}
                            value={condition.text}
                            onChange={(e) =>
                                setConditions(replaceAt(rule.conditions, index, {...condition, text: e.target.value}))
                            }
                        />
                        <button
                            className={`rule-case${condition.caseSensitive ? ' on' : ''}`}
                            aria-label={`Match case ${index + 1}`}
                            aria-pressed={condition.caseSensitive}
                            title={
                                condition.caseSensitive
                                    ? 'Matching case: "Invoice" will not match "invoice"'
                                    : 'Ignoring case: "Invoice" matches "invoice"'
                            }
                            onClick={() =>
                                setConditions(
                                    replaceAt(rule.conditions, index, {
                                        ...condition,
                                        caseSensitive: !condition.caseSensitive,
                                    }),
                                )
                            }
                        >
                            Aa
                        </button>
                        {rule.conditions.length > 1 && (
                            <button
                                className="rule-remove"
                                aria-label={`Remove condition ${index + 1}`}
                                title="Remove this condition"
                                onClick={() => setConditions(rule.conditions.filter((_, i) => i !== index))}
                            >
                                &times;
                            </button>
                        )}
                    </div>
                ))}
                <button
                    className="rule-add"
                    onClick={() => setConditions([...rule.conditions, {...EMPTY_CONDITION}])}
                >
                    + Add condition
                </button>
            </section>

            <section className="rule-section">
                <div className="rule-section-head">
                    <h3 className="rule-section-title">Then do this</h3>
                </div>

                {rule.actions.map((action, index) => (
                    <div className={`rule-card${action.kind === 'destroy' ? ' danger' : ''}`} key={`action-${index}`}>
                        <select
                            className="rule-action-kind"
                            aria-label={`Action ${index + 1}`}
                            value={action.kind}
                            onChange={(e) =>
                                setActions(replaceAt(rule.actions, index, {kind: e.target.value, folderId: ''}))
                            }
                        >
                            {Object.entries(ACTION_LABELS).map(([value, label]) => (
                                <option key={value} value={value}>{label}</option>
                            ))}
                        </select>
                        {action.kind === 'moveTo' && (
                            <select
                                className="rule-destination"
                                aria-label={`Destination ${index + 1}`}
                                value={action.folderId}
                                onChange={(e) =>
                                    setActions(replaceAt(rule.actions, index, {...action, folderId: e.target.value}))
                                }
                            >
                                <option value="">Choose a folder</option>
                                {reachable.map((f) => (
                                    <option key={f.id} value={f.id}>{f.label}</option>
                                ))}
                            </select>
                        )}
                        {rule.actions.length > 1 && (
                            <button
                                className="rule-remove"
                                aria-label={`Remove action ${index + 1}`}
                                title="Remove this action"
                                onClick={() => setActions(rule.actions.filter((_, i) => i !== index))}
                            >
                                &times;
                            </button>
                        )}
                    </div>
                ))}
                <button className="rule-add" onClick={() => setActions([...rule.actions, {...EMPTY_ACTION}])}>
                    + Add action
                </button>
            </section>

            <section className="rule-section">
                <label className="rule-check">
                    <input
                        type="checkbox"
                        checked={rule.stopProcessing}
                        onChange={(e) => onChange({...rule, stopProcessing: e.target.checked} as Rule)}
                    />
                    <span>Stop running later rules on a message this one matches</span>
                </label>
            </section>

            {destroying && (
                <p className="rule-danger">
                    Delete permanently destroys matching mail on the server. It does not go to Trash, it
                    is not kept locally and it cannot be recovered.
                </p>
            )}
        </div>
    )
}
