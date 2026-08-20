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

export const OPERATOR_LABELS: Record<string, string> = {
    contains: 'contains',
    notContains: "doesn't contain",
    equals: 'is',
    startsWith: 'starts with',
    endsWith: 'ends with',
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
}
export const EMPTY_ACTION: RuleAction = {kind: 'markRead', folderId: ''}

// emptyRule is a brand-new, enabled rule with one blank condition and one harmless action.
export function emptyRule(position: number): Rule {
    return {
        id: '',
        name: '',
        enabled: true,
        position,
        matchMode: 'any',
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
    const joiner = rule.matchMode === 'any' ? ' or ' : ' and '
    const conditions = rule.conditions.map(conditionText).join(joiner)
    const actions = rule.actions.map((a) => actionText(a, folderName)).join(', ')
    return `${scopeText(rule, accountName)}if ${conditions}, then ${actions}`
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
    return `${FIELD_LABELS[c.field] ?? c.field} ${OPERATOR_LABELS[c.operator] ?? c.operator} "${c.text}"${cased}`
}

// actionText renders one action in the summary line, naming a move's destination folder.
function actionText(a: RuleAction, folderName: (folderId: string) => string): string {
    if (a.kind === 'moveTo') {
        return `move to ${folderName(a.folderId)}`
    }
    return ACTION_PHRASES[a.kind] ?? a.kind
}
