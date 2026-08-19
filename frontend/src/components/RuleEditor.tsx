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
}

interface RuleEditorProps {
    rule: Rule
    folders: FolderChoice[]
    onChange: (rule: Rule) => void
}

// RuleEditor builds one rule: its name, how its conditions combine, the conditions themselves and the
// actions taken when they match. It holds no state of its own; the modal owns the draft and this
// renders it, so there is one place a rule can be in an inconsistent state.
export function RuleEditor({rule, folders, onChange}: RuleEditorProps) {
    const setConditions = (conditions: RuleCondition[]) => onChange({...rule, conditions} as Rule)
    const setActions = (actions: RuleAction[]) => onChange({...rule, actions} as Rule)
    const replaceAt = <T,>(list: T[], index: number, value: T): T[] =>
        list.map((item, i) => (i === index ? value : item))

    return (
        <div className="rule-form">
            <input
                className="tag-name-input"
                placeholder="Rule name"
                value={rule.name}
                autoFocus
                onChange={(e) => onChange({...rule, name: e.target.value} as Rule)}
            />

            <div className="rule-form-row">
                <span>If</span>
                <select
                    aria-label="Match mode"
                    value={rule.matchMode}
                    onChange={(e) => onChange({...rule, matchMode: e.target.value} as Rule)}
                >
                    <option value="all">all of these match</option>
                    <option value="any">any of these match</option>
                </select>
            </div>

            {rule.conditions.map((condition, index) => (
                <div className="rule-form-row" key={`condition-${index}`}>
                    <select
                        aria-label="Field"
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
                        aria-label="Operator"
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
                        className="tag-name-input"
                        placeholder="text to match"
                        aria-label="Match text"
                        value={condition.text}
                        onChange={(e) =>
                            setConditions(replaceAt(rule.conditions, index, {...condition, text: e.target.value}))
                        }
                    />
                    <button
                        className="account-action delete"
                        aria-label={`Remove condition ${index + 1}`}
                        title="Remove this condition"
                        disabled={rule.conditions.length === 1}
                        onClick={() => setConditions(rule.conditions.filter((_, i) => i !== index))}
                    >
                        &times;
                    </button>
                </div>
            ))}
            <div className="rule-form-row">
                <button
                    className="btn"
                    onClick={() => setConditions([...rule.conditions, {...EMPTY_CONDITION}])}
                >
                    Add condition
                </button>
            </div>

            {rule.actions.map((action, index) => (
                <div className="rule-form-row" key={`action-${index}`}>
                    <span>{index === 0 ? 'then' : 'and'}</span>
                    <select
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
                            aria-label={`Destination ${index + 1}`}
                            value={action.folderId}
                            onChange={(e) =>
                                setActions(replaceAt(rule.actions, index, {...action, folderId: e.target.value}))
                            }
                        >
                            <option value="">Choose a folder</option>
                            {folders.map((f) => (
                                <option key={f.id} value={f.id}>{f.label}</option>
                            ))}
                        </select>
                    )}
                    <button
                        className="account-action delete"
                        aria-label={`Remove action ${index + 1}`}
                        title="Remove this action"
                        disabled={rule.actions.length === 1}
                        onClick={() => setActions(rule.actions.filter((_, i) => i !== index))}
                    >
                        &times;
                    </button>
                </div>
            ))}
            <div className="rule-form-row">
                <button className="btn" onClick={() => setActions([...rule.actions, {...EMPTY_ACTION}])}>
                    Add action
                </button>
            </div>

            <label className="rule-form-row">
                <input
                    type="checkbox"
                    checked={rule.stopProcessing}
                    onChange={(e) => onChange({...rule, stopProcessing: e.target.checked} as Rule)}
                />
                <span>Stop running later rules on a message this one matches</span>
            </label>

            {rule.actions.some((a) => a.kind === 'destroy') && (
                <p className="rule-danger">
                    Delete permanently destroys matching mail on the server. It does not go to Trash, it
                    is not kept locally and it cannot be recovered.
                </p>
            )}
        </div>
    )
}
