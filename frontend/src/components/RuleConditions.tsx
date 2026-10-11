import {Fragment} from 'react'
import {Rule, RuleCondition} from '../api'
import {EMPTY_CONDITION, FIELD_LABELS, OPERATOR_LABELS, isNegative} from './ruleLabels'

interface RuleConditionsProps {
    rule: Rule
    onChange: (rule: Rule) => void
}

// replaceAt returns the list with one entry swapped, the shape every edit here takes.
function replaceAt<T>(list: T[], index: number, value: T): T[] {
    return list.map((item, i) => (i === index ? value : item))
}

// placed sets which list a condition lives in. The retired notContains operator is rewritten on the
// way out, so a rule edited here never carries both spellings of a negation.
function placed(condition: RuleCondition, exception: boolean): RuleCondition {
    return {
        ...condition,
        operator: condition.operator === 'notContains' ? 'contains' : condition.operator,
        negate: exception,
    }
}

// RuleConditions is a rule's conditions as two lists: the ones a message has to match, then the
// exceptions that let a message off whatever else it matches. That is how the engine reads a rule in
// either mode (some or all of the first list, none of the second), so the screen shows the logic rather
// than leaving it to a per-row NOT switch whose word was printed whether it was on or off.
//
// The two lists are views over the one conditions array: a condition's negate flag decides where it
// shows and moving it flips the flag in place. Rows are numbered by their position in that array, so a
// row keeps its number when it moves.
export function RuleConditions({rule, onChange}: RuleConditionsProps) {
    const setConditions = (conditions: RuleCondition[]) => onChange({...rule, conditions} as Rule)
    // joiner is the word printed between rows of the first list, taken from the match mode so the rule
    // reads as the sentence it is. The exceptions are always alternatives: any one lets a message off.
    const joiner = rule.matchMode === 'any' ? 'or' : 'and'
    const indexed = rule.conditions.map((condition, index) => ({condition, index}))
    const matches = indexed.filter(({condition}) => !isNegative(condition))
    const exceptions = indexed.filter(({condition}) => isNegative(condition))

    const card = (condition: RuleCondition, index: number, exception: boolean) => (
        <div className="rule-card">
            <select
                className="rule-field"
                aria-label={`Field ${index + 1}`}
                value={condition.field}
                onChange={(e) => setConditions(replaceAt(rule.conditions, index, {...condition, field: e.target.value}))}
            >
                {Object.entries(FIELD_LABELS).map(([value, label]) => (
                    <option key={value} value={value}>{label}</option>
                ))}
            </select>
            <select
                className="rule-operator"
                aria-label={`Operator ${index + 1}`}
                value={condition.operator === 'notContains' ? 'contains' : condition.operator}
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
                onChange={(e) => setConditions(replaceAt(rule.conditions, index, {...condition, text: e.target.value}))}
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
                        replaceAt(rule.conditions, index, {...condition, caseSensitive: !condition.caseSensitive}),
                    )
                }
            >
                Aa
            </button>
            <button
                className="rule-move"
                aria-label={`Move condition ${index + 1} to ${exception ? 'matches' : 'exceptions'}`}
                title={
                    exception
                        ? 'Move this to the conditions a message has to match'
                        : 'Move this to the exceptions, so a message matching it is left alone'
                }
                onClick={() => setConditions(replaceAt(rule.conditions, index, placed(condition, !exception)))}
            >
                {exception ? 'Include ↑' : 'Exclude ↓'}
            </button>
            {/* Absent, not disabled, on the last remaining condition: a rule needs at least one, so a
                cross there could never be clicked. */}
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
    )

    return (
        <>
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
                {matches.length === 0 && (
                    <p className="rule-empty">No conditions, so every message counts unless an exception below lets it off.</p>
                )}
                {matches.map(({condition, index}, position) => (
                    <Fragment key={`condition-${index}`}>
                        {/* The joiner is spelled out between the rows rather than left to be inferred
                            from the mode dropdown above them: reading a rule top to bottom is how anyone
                            checks what it will do. It follows the mode, so the two cannot disagree. */}
                        {position > 0 && <div className="rule-joiner">{joiner}</div>}
                        {card(condition, index, false)}
                    </Fragment>
                ))}
                <button className="rule-add" onClick={() => setConditions([...rule.conditions, {...EMPTY_CONDITION}])}>
                    + Add condition
                </button>
            </section>

            <section className="rule-section">
                <div className="rule-section-head">
                    <h3 className="rule-section-title">Except when it matches any of these</h3>
                </div>
                {exceptions.length === 0 && <p className="rule-empty">No exceptions.</p>}
                {exceptions.map(({condition, index}, position) => (
                    <Fragment key={`condition-${index}`}>
                        {position > 0 && <div className="rule-joiner">or</div>}
                        {card(condition, index, true)}
                    </Fragment>
                ))}
                <button
                    className="rule-add"
                    onClick={() => setConditions([...rule.conditions, placed(EMPTY_CONDITION, true)])}
                >
                    + Add exception
                </button>
            </section>
        </>
    )
}
