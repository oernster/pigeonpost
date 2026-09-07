// The filter-rule half of the Wails seam: the rule types and the calls that read, write, order and
// apply rules. It lives beside api.ts rather than inside it because api.ts is one of the modules over
// the size limit that the guard in src/test/loc.test.ts is ratcheting down; a cohesive group of calls
// with its own types is exactly the kind of thing that should leave it. The api object spreads what is
// exported here, so callers still reach these through api.* and nothing else changes.
import {
    DeleteRule,
    ListRules,
    PreviewRuleBackfill,
    ReorderRules,
    RunRuleBackfill,
    SaveRule,
} from '../wailsjs/go/main/App'
import {main} from '../wailsjs/go/models'

// Rule drops the generated convertValues helper, the same way Message and MessageBody do: the rule
// editor builds and edits rules as plain object literals; Wails hands back plain JSON at runtime
// anyway. The nested condition and action DTOs carry no helper of their own.
export type Rule = Omit<main.RuleDTO, 'convertValues'>
export type RuleCondition = main.RuleConditionDTO
export type RuleAction = main.RuleActionDTO
// RuleBackfill is what applying one rule to the mail already stored would do or did do. The same
// shape carries both, so the confirmation and the result read identically.
export type RuleBackfill = main.RuleBackfillDTO

// RuleInput is the shape sent back to save a rule. It mirrors Rule exactly, so a rule read from the
// back end can be edited and returned without translation; an empty id means a new rule.
export interface RuleInput {
    id: string
    name: string
    enabled: boolean
    position: number
    matchMode: string
    stopProcessing: boolean
    conditions: RuleCondition[]
    actions: RuleAction[]
}

export const rulesApi = {
    listRules: (): Promise<Rule[]> => ListRules(),
    saveRule: (req: RuleInput): Promise<void> => SaveRule(main.RuleDTO.createFrom(req)),
    deleteRule: (ruleId: string): Promise<void> => DeleteRule(ruleId),
    // reorderRules writes the evaluation order: the rule at index i takes position i.
    reorderRules: (orderedIds: string[]): Promise<void> => ReorderRules(orderedIds),
    // previewRuleBackfill reports what applying a rule to the mail already stored would do, changing
    // nothing; runRuleBackfill applies it and reports what it actually did.
    previewRuleBackfill: (ruleId: string): Promise<RuleBackfill> => PreviewRuleBackfill(ruleId),
    runRuleBackfill: (ruleId: string): Promise<RuleBackfill> => RunRuleBackfill(ruleId),
}
