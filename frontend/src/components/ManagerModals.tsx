import {Account, CalendarEvent, Contact, Rule, Template} from '../api'
import type {ManagedCollection} from '../hooks/useManagedCollection'
import {ContactsModal} from './ContactsModal'
import {CalendarModal} from './CalendarModal'
import {RuleManagerModal} from './RuleManagerModal'
import {TemplateManagerModal} from './TemplateManagerModal'

// ManagerModals renders the four dialogs that manage the four backend-backed collections: contacts,
// the calendar, the filter rules and the message templates. They are one component because they are one
// shape, each opened from its collection's own `managing` flag, each reloading its collection when it
// changes it and each closing by clearing that flag. Held apart in App they were four near-identical
// blocks whose only real differences are the extra context the calendar and the rules need.
//
// The collections arrive whole rather than as loose props, so adding a fifth manager is one entry here
// and one `useManagedCollection` call in App, with no new prop threaded through both.
interface ManagerModalsProps {
    contacts: ManagedCollection<Contact>
    calendar: ManagedCollection<CalendarEvent>
    rules: ManagedCollection<Rule>
    templates: ManagedCollection<Template>
    // The calendar composes and answers invitations, so it needs the account it is acting as.
    accounts: Account[]
    accountId: string
    accountEmail: string
    accountName: string
    // calendarInitialEvent is the event a clicked reminder lands the calendar on, cleared when it closes
    // so a later open starts on the month rather than back on that event.
    calendarInitialEvent: string | null
    onCalendarClosed: () => void
}

export function ManagerModals({
    contacts, calendar, rules, templates,
    accounts, accountId, accountEmail, accountName,
    calendarInitialEvent, onCalendarClosed,
}: ManagerModalsProps) {
    return (
        <>
            {contacts.managing && (
                <ContactsModal
                    contacts={contacts.items}
                    onChanged={() => void contacts.reload()}
                    onClose={() => contacts.setManaging(false)}
                />
            )}
            {calendar.managing && (
                <CalendarModal
                    events={calendar.items}
                    accountId={accountId}
                    accountEmail={accountEmail}
                    accountName={accountName}
                    initialEventId={calendarInitialEvent ?? undefined}
                    onChanged={() => void calendar.reload()}
                    onClose={() => {
                        calendar.setManaging(false)
                        onCalendarClosed()
                    }}
                />
            )}
            {rules.managing && (
                <RuleManagerModal
                    accounts={accounts}
                    rules={rules.items}
                    onChanged={() => void rules.reload()}
                    onClose={() => rules.setManaging(false)}
                />
            )}
            {templates.managing && (
                <TemplateManagerModal
                    templates={templates.items}
                    onChanged={() => void templates.reload()}
                    onClose={() => templates.setManaging(false)}
                />
            )}
        </>
    )
}
