import {Rule, RuleCondition, RuleAction} from '../api'

// The tokens the back end recognises, paired with what the user sees. They are kept here rather than
// inline in the modal so the editor, the summary line and the warning text cannot drift apart.

// FIELD_LABELS lead with "All fields", the default a new condition starts on: reaching anywhere in the
// message is what a rule usually wants; narrowing from there is easier than knowing to widen.
export const FIELD_LABELS: Record<string, string> = {
    all: 'All fields',
    from: 'From',
    to: 'To',
    cc: 'Cc',
    anyRecipient: 'Any recipient',
    subject: 'Subject',
    senderDomain: 'Sender domain',
}

// OPERATOR_LABELS are the comparisons a condition can make. Negation is not one of them: it is the
// separate NOT switch on the row, so every comparison has its opposite rather than only "contains".
// The retired notContains token is still read back from a rule written before that (the back end
// folds it into contains with NOT set), so it needs no label here.
export const OPERATOR_LABELS: Record<string, string> = {
    contains: 'contains',
    equals: 'is',
    startsWith: 'starts with',
    endsWith: 'ends with',
}

// NEGATED_OPERATOR_LABELS are the same comparisons as they read with NOT set, for the summary line.
// They are written out rather than composed from a prefix, since English negates each of them
// differently and "not is" is not a sentence.
export const NEGATED_OPERATOR_LABELS: Record<string, string> = {
    contains: "doesn't contain",
    notContains: "doesn't contain",
    equals: 'is not',
    startsWith: "doesn't start with",
    endsWith: "doesn't end with",
}

export const ACTION_LABELS: Record<string, string> = {
    markRead: 'Mark as read',
    flag: 'Flag it',
    moveTo: 'Move to folder',
    destroy: 'Delete permanently',
}

// ACTION_PHRASES are the same actions worded to read inside a sentence, for the summary line. They are
// a separate map rather than a lower-cased ACTION_LABELS, because lower-casing the rendered line would
// also flatten the folder name a move names.
export const ACTION_PHRASES: Record<string, string> = {
    markRead: 'mark as read',
    flag: 'flag it',
    moveTo: 'move to',
    destroy: 'delete permanently',
}

// DESTRUCTIVE_ACTIONS are the kinds that take a message out of the inbox. A rule carrying one is
// confirmed before it is saved, because a rule runs unattended and cannot ask about each message.
export const DESTRUCTIVE_ACTIONS = new Set(['moveTo', 'destroy'])

// EMPTY_CONDITION and EMPTY_ACTION are the rows a new rule and a newly added row start from. A
// condition begins matching every field, case-insensitively, which is what a rule usually wants.
export const EMPTY_CONDITION: RuleCondition = {
    field: 'all',
    operator: 'contains',
    text: '',
    caseSensitive: false,
    negate: false,
}
export const EMPTY_ACTION: RuleAction = {kind: 'markRead', folderId: ''}

// emptyRule is a brand-new, enabled rule with one blank condition and one harmless action.
//
// It starts on "all", so a second condition NARROWS the rule. The default was "any", which reads
// harmlessly while a rule has one condition then turns dangerous the moment a negative one is added:
// "does not contain X" as one arm of an or matches nearly every message, so a rule meant to file a
// handful of senders sweeps the whole mailbox into a folder; a destroying rule empties it. Narrowing is
// someone adding a second line almost always means; widening is available in one click beside it.
export function emptyRule(position: number): Rule {
    return {
        id: '',
        name: '',
        enabled: true,
        position,
        matchMode: 'all',
        stopProcessing: false,
        accountIds: [],
        conditions: [{...EMPTY_CONDITION}],
        actions: [{...EMPTY_ACTION}],
    } as Rule
}

// isDestructive reports whether a rule moves or destroys the messages it matches.
export function isDestructive(rule: Rule): boolean {
    return rule.actions.some((a) => DESTRUCTIVE_ACTIONS.has(a.kind))
}

// destroys reports whether a rule deletes matching messages outright, the one action with nothing to
// undo. The list marks such a rule apart from a merely destructive one.
export function destroys(rule: Rule): boolean {
    return rule.actions.some((a) => a.kind === 'destroy')
}

// ruleIsComplete reports whether a rule can be saved: it needs a name, every condition needs match
// text and every move needs a destination.
export function ruleIsComplete(rule: Rule): boolean {
    if (rule.name.trim() === '' || rule.conditions.length === 0 || rule.actions.length === 0) {
        return false
    }
    if (rule.conditions.some((c) => c.text.trim() === '')) {
        return false
    }
    return !rule.actions.some((a) => a.kind === 'moveTo' && a.folderId === '')
}

// ruleSummary is the one-line description shown under a rule's name, spelling out the whole rule
// rather than only its first condition. A rule limited to some accounts says so up front, since which
// mail a rule can reach matters more than what it then does to it.
export function ruleSummary(
    rule: Rule,
    folderName: (folderId: string) => string,
    accountName: (accountId: string) => string,
): string {
    const conditions = conditionsText(rule)
    const actions = rule.actions.map((a) => actionText(a, folderName)).join(', ')
    return `${scopeText(rule, accountName)}if ${conditions}, then ${actions}`
}

// isNegative reports whether a condition states what a message must NOT be. Such a condition always
// applies, whatever the match mode, which is why the summary sets it apart from the rest. The retired
// notContains operator counts too, so a rule read back before the migration rewrites it reads right.
export function isNegative(condition: RuleCondition): boolean {
    return condition.negate || condition.operator === 'notContains'
}

// conditionsText spells out how a rule's conditions combine, in the same terms the engine applies
// them. Under "all" they are simply joined with and. Under "any" the positives are joined with or and
// bracketed, then the exclusions follow. That is what the rule does: any of these, never those. A rule of exclusions alone has to meet all of them, so they read with and.
function conditionsText(rule: Rule): string {
    const positives = rule.conditions.filter((c) => !isNegative(c)).map(conditionText)
    const negatives = rule.conditions.filter(isNegative).map(conditionText)
    if (rule.matchMode !== 'any' || negatives.length === 0) {
        return rule.conditions.map(conditionText).join(rule.matchMode === 'any' ? ' or ' : ' and ')
    }
    if (positives.length === 0) {
        return negatives.join(' and ')
    }
    const head = positives.length === 1 ? positives[0] : `(${positives.join(' or ')})`
    return `${head} and ${negatives.join(' and ')}`
}

// scopeText opens the summary with the accounts a rule is limited to. It is stated on every rule,
// including the unscoped ones: which mail a rule can reach is the first thing to know about it; a
// blank there would read as "not yet decided" rather than as "all of them".
function scopeText(rule: Rule, accountName: (accountId: string) => string): string {
    if (rule.accountIds.length === 0) {
        return 'On any account, '
    }
    return `On ${rule.accountIds.map(accountName).join(' and ')}, `
}

// conditionText renders one condition in the summary line, noting case sensitivity only when it is on,
// since case-insensitive is the default and saying so every time would be noise.
function conditionText(c: RuleCondition): string {
    const cased = c.caseSensitive ? ' (match case)' : ''
    const labels = isNegative(c) ? NEGATED_OPERATOR_LABELS : OPERATOR_LABELS
    return `${FIELD_LABELS[c.field] ?? c.field} ${labels[c.operator] ?? c.operator} "${c.text}"${cased}`
}

// actionText renders one action in the summary line, naming a move's destination folder.
function actionText(a: RuleAction, folderName: (folderId: string) => string): string {
    if (a.kind === 'moveTo') {
        return `move to ${folderName(a.folderId)}`
    }
    return ACTION_PHRASES[a.kind] ?? a.kind
}
