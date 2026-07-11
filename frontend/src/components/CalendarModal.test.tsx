// Characterization test for the calendar at its stable outer interface (events, accountId, accountEmail,
// accountName, initialEventId, onChanged, onClose). It renders the real modal with its real child dialogs
// (the event form, the scope chooser, the confirm dialogs, the calendars manager, the recurrence editor and
// the time grid) and drives each flow, asserting the DOM plus which api call fired. The interface it pins
// does not move as the modal is decomposed in Phase 2 (the instance loading to useEventInstances, the
// calendars to useCalendars plus CalendarsManager, the open-from-reminder orchestration to
// useOpenFromReminder and the event dialog to EventFormModal), so this suite staying green is the proof each
// extraction preserved behaviour. ../api is stubbed (the one Wails seam); tz and calendarModel are real pure
// modules and run as-is.
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {cleanup, fireEvent, render, screen, waitFor, within} from '@testing-library/react'
import type {ComponentProps} from 'react'
import {CalendarModal} from './CalendarModal'
import {EventScope} from '../api'
import type {Calendar, CalendarEvent, CalendarEventInstance} from '../api'

const apiSpies = vi.hoisted(() => ({
    listCalendars: vi.fn(),
    listEventInstances: vi.fn(),
    saveCalendar: vi.fn(),
    deleteCalendar: vi.fn(),
    saveEvent: vi.fn(),
    saveEventScoped: vi.fn(),
    deleteEvent: vi.fn(),
    deleteEventScoped: vi.fn(),
    sendMeetingRequest: vi.fn(),
    sendMeetingCancel: vi.fn(),
    importEventsFromFile: vi.fn(),
    exportEventsToFile: vi.fn(),
    openExternal: vi.fn(),
}))

// The mock provides EventScope with the real integer values (a runtime enum the modal and the ScopeChooser
// both read) and swaps the api object for spies.
vi.mock('../api', () => ({
    api: apiSpies,
    EventScope: {This: 0, Future: 1, All: 2},
}))

type CalendarProps = ComponentProps<typeof CalendarModal>

// NOON_TODAY is an instant at noon today, kept well clear of midnight so it lands on today's cell in the
// month grid whatever the runner's timezone.
const NOON_TODAY = (() => {
    const now = new Date()
    return new Date(now.getFullYear(), now.getMonth(), now.getDate(), 12, 0, 0, 0).toISOString()
})()

function makeEvent(overrides: Partial<CalendarEvent> = {}): CalendarEvent {
    return {
        id: 'evt-1', uid: 'uid-1', calendarId: '', summary: 'Standup', description: '', location: '',
        start: NOON_TODAY, end: '', allDay: false, recurrence: '', timeZone: '', reminders: [], extra: '',
        organizer: {address: '', commonName: ''}, attendees: [],
        ...overrides,
    } as CalendarEvent
}

function makeInstance(event: CalendarEvent, overrides: Partial<CalendarEventInstance> = {}): CalendarEventInstance {
    return {event, start: event.start, end: event.end, recurrenceId: '', ...overrides} as CalendarEventInstance
}

function makeCalendar(id: string, name: string, colour = '#3b82f6'): Calendar {
    return {id, name, colour} as Calendar
}

function renderCalendar(overrides: Partial<CalendarProps> = {}) {
    const onChanged = vi.fn()
    const onClose = vi.fn()
    const props: CalendarProps = {
        events: [], accountId: 'acc1', accountEmail: 'me@x.com', accountName: 'Me',
        onChanged, onClose,
        ...overrides,
    }
    const view = render(<CalendarModal {...props}/>)
    return {...view, onChanged, onClose}
}

beforeEach(() => {
    apiSpies.listCalendars.mockReset().mockResolvedValue([])
    apiSpies.listEventInstances.mockReset().mockResolvedValue([])
    apiSpies.saveCalendar.mockReset().mockResolvedValue(undefined)
    apiSpies.deleteCalendar.mockReset().mockResolvedValue(undefined)
    apiSpies.saveEvent.mockReset().mockResolvedValue('evt-new')
    apiSpies.saveEventScoped.mockReset().mockResolvedValue(undefined)
    apiSpies.deleteEvent.mockReset().mockResolvedValue(undefined)
    apiSpies.deleteEventScoped.mockReset().mockResolvedValue(undefined)
    apiSpies.sendMeetingRequest.mockReset().mockResolvedValue(undefined)
    apiSpies.sendMeetingCancel.mockReset().mockResolvedValue(undefined)
    apiSpies.importEventsFromFile.mockReset().mockResolvedValue(0)
    apiSpies.exportEventsToFile.mockReset().mockResolvedValue(true)
    apiSpies.openExternal.mockReset().mockResolvedValue(undefined)
})

afterEach(() => cleanup())

describe('CalendarModal: basics', () => {
    it('renders the calendar and loads calendars and instances on mount', async () => {
        renderCalendar()
        expect(screen.getByRole('dialog', {name: 'Calendar'})).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'New event'})).toBeInTheDocument()
        await waitFor(() => expect(apiSpies.listCalendars).toHaveBeenCalled())
        expect(apiSpies.listEventInstances).toHaveBeenCalled()
    })

    it('renders a loaded event in the month grid', async () => {
        apiSpies.listEventInstances.mockResolvedValue([makeInstance(makeEvent({summary: 'Standup'}))])
        renderCalendar()
        expect(await screen.findByRole('button', {name: /Standup/})).toBeInTheDocument()
    })
})

describe('CalendarModal: navigation and refetch', () => {
    it('refetches instances when the view moves', async () => {
        renderCalendar()
        await waitFor(() => expect(apiSpies.listEventInstances).toHaveBeenCalled())
        const before = apiSpies.listEventInstances.mock.calls.length
        fireEvent.click(screen.getByRole('button', {name: 'Next'}))
        await waitFor(() => expect(apiSpies.listEventInstances.mock.calls.length).toBeGreaterThan(before))
    })

    it('switches to the week time grid and refetches', async () => {
        renderCalendar()
        await waitFor(() => expect(apiSpies.listEventInstances).toHaveBeenCalled())
        const before = apiSpies.listEventInstances.mock.calls.length
        fireEvent.click(screen.getByRole('button', {name: 'Week'}))
        await waitFor(() => expect(apiSpies.listEventInstances.mock.calls.length).toBeGreaterThan(before))
        expect(screen.getByRole('button', {name: 'Week'})).toHaveAttribute('aria-pressed', 'true')
    })
})

describe('CalendarModal: event form', () => {
    it('opens the new event form from the toolbar', async () => {
        renderCalendar()
        fireEvent.click(screen.getByRole('button', {name: 'New event'}))
        expect(await screen.findByRole('dialog', {name: 'New event'})).toBeInTheDocument()
        expect(screen.getByPlaceholderText('Event title')).toBeInTheDocument()
    })

    it('creates an event, saves it and refetches', async () => {
        const {onChanged} = renderCalendar()
        fireEvent.click(screen.getByRole('button', {name: 'New event'}))
        fireEvent.change(await screen.findByPlaceholderText('Event title'), {target: {value: 'Lunch'}})
        fireEvent.click(screen.getByRole('button', {name: 'Add event'}))
        await waitFor(() => expect(apiSpies.saveEvent).toHaveBeenCalledWith(expect.objectContaining({summary: 'Lunch'})))
        expect(onChanged).toHaveBeenCalled()
        await waitFor(() => expect(apiSpies.listEventInstances.mock.calls.length).toBeGreaterThan(1))
        expect(screen.queryByRole('dialog', {name: 'New event'})).toBeNull()
    })

    it('opens an existing one-off event for editing', async () => {
        apiSpies.listEventInstances.mockResolvedValue([makeInstance(makeEvent({id: 'e1', summary: 'Standup'}))])
        renderCalendar()
        fireEvent.click(await screen.findByRole('button', {name: /Standup/}))
        expect(screen.getByRole('dialog', {name: 'Edit event'})).toBeInTheDocument()
        expect(screen.getByDisplayValue('Standup')).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Delete'})).toBeInTheDocument()
    })
})

describe('CalendarModal: delete', () => {
    it('deletes a one-off event after confirming', async () => {
        apiSpies.listEventInstances.mockResolvedValue([makeInstance(makeEvent({id: 'e1', summary: 'Standup'}))])
        const {onChanged} = renderCalendar()
        fireEvent.click(await screen.findByRole('button', {name: /Standup/}))
        fireEvent.click(screen.getByRole('button', {name: 'Delete'}))
        const confirm = screen.getByRole('alertdialog', {name: 'Delete event'})
        fireEvent.click(within(confirm).getByRole('button', {name: 'Delete'}))
        await waitFor(() => expect(apiSpies.deleteEvent).toHaveBeenCalledWith('e1'))
        expect(onChanged).toHaveBeenCalled()
    })
})

describe('CalendarModal: recurring events', () => {
    it('asks the scope when editing a recurring occurrence', async () => {
        const ev = makeEvent({id: 'r1', summary: 'Weekly sync', recurrence: 'FREQ=WEEKLY'})
        apiSpies.listEventInstances.mockResolvedValue([makeInstance(ev, {recurrenceId: NOON_TODAY})])
        renderCalendar()
        fireEvent.click(await screen.findByRole('button', {name: /Weekly sync/}))
        const scope = screen.getByRole('alertdialog', {name: 'Edit recurring event'})
        fireEvent.click(within(scope).getByRole('button', {name: 'All events'}))
        expect(screen.getByRole('dialog', {name: 'Edit event'})).toBeInTheDocument()
    })

    it('deletes a recurring series at a chosen scope', async () => {
        const ev = makeEvent({id: 'r1', summary: 'Weekly sync', recurrence: 'FREQ=WEEKLY'})
        apiSpies.listEventInstances.mockResolvedValue([makeInstance(ev, {recurrenceId: NOON_TODAY})])
        renderCalendar()
        fireEvent.click(await screen.findByRole('button', {name: /Weekly sync/}))
        fireEvent.click(within(screen.getByRole('alertdialog', {name: 'Edit recurring event'}))
            .getByRole('button', {name: 'All events'}))
        fireEvent.click(screen.getByRole('button', {name: 'Delete'}))
        const del = screen.getByRole('alertdialog', {name: 'Delete recurring event'})
        fireEvent.click(within(del).getByRole('button', {name: 'This event'}))
        await waitFor(() => expect(apiSpies.deleteEventScoped).toHaveBeenCalled())
        expect(apiSpies.deleteEventScoped).toHaveBeenCalledWith(EventScope.This, 'r1', expect.any(String))
    })
})

describe('CalendarModal: meetings', () => {
    it('creates a meeting and sends the invitation', async () => {
        renderCalendar({accountId: 'acc1', accountEmail: 'me@x.com', accountName: 'Me'})
        fireEvent.click(screen.getByRole('button', {name: 'New event'}))
        fireEvent.change(await screen.findByPlaceholderText('Event title'), {target: {value: 'Planning'}})
        fireEvent.change(screen.getByPlaceholderText('Attendee email'), {target: {value: 'a@b.com'}})
        fireEvent.click(screen.getByRole('button', {name: /Add attendee/}))
        fireEvent.click(await screen.findByRole('button', {name: 'Send invitation'}))
        await waitFor(() => expect(apiSpies.saveEvent).toHaveBeenCalledWith(expect.objectContaining({summary: 'Planning'})))
        await waitFor(() => expect(apiSpies.sendMeetingRequest).toHaveBeenCalledWith('acc1', 'evt-new'))
        expect(await screen.findByText(/Invitation sent to 1 attendee/)).toBeInTheDocument()
    })

    it('cancels an existing meeting after confirming', async () => {
        const meeting = makeEvent({
            id: 'm1', summary: 'Review',
            attendees: [{address: 'a@b.com', commonName: '', role: 'REQ-PARTICIPANT', status: 'NEEDS-ACTION', rsvp: true}],
            organizer: {address: 'me@x.com', commonName: 'Me'},
        })
        apiSpies.listEventInstances.mockResolvedValue([makeInstance(meeting)])
        const {onChanged} = renderCalendar({accountId: 'acc1'})
        fireEvent.click(await screen.findByRole('button', {name: /Review/}))
        fireEvent.click(screen.getByRole('button', {name: 'Cancel meeting'}))
        const confirm = screen.getByRole('alertdialog', {name: 'Cancel meeting'})
        fireEvent.click(within(confirm).getByRole('button', {name: 'Send cancellation'}))
        await waitFor(() => expect(apiSpies.sendMeetingCancel).toHaveBeenCalledWith('acc1', 'm1'))
        await waitFor(() => expect(apiSpies.deleteEvent).toHaveBeenCalledWith('m1'))
        expect(onChanged).toHaveBeenCalled()
    })
})

describe('CalendarModal: import and export', () => {
    it('imports events from a file', async () => {
        apiSpies.importEventsFromFile.mockResolvedValue(3)
        const {onChanged} = renderCalendar()
        fireEvent.click(screen.getByRole('button', {name: /Import/}))
        expect(await screen.findByText(/Imported 3 events/)).toBeInTheDocument()
        expect(onChanged).toHaveBeenCalled()
    })

    it('exports events to a file', async () => {
        apiSpies.exportEventsToFile.mockResolvedValue(true)
        renderCalendar({events: [makeEvent({id: 'e1'})]})
        fireEvent.click(screen.getByRole('button', {name: 'Export ICS'}))
        expect(await screen.findByText(/Exported 1 event/)).toBeInTheDocument()
    })
})

describe('CalendarModal: calendars manager', () => {
    it('lists calendars in the manager', async () => {
        apiSpies.listCalendars.mockResolvedValue([makeCalendar('c1', 'Work'), makeCalendar('c2', 'Home')])
        renderCalendar()
        await waitFor(() => expect(apiSpies.listCalendars).toHaveBeenCalled())
        fireEvent.click(screen.getByRole('button', {name: 'Calendars'}))
        const mgr = await screen.findByRole('dialog', {name: 'Calendars'})
        expect(within(mgr).getByText('Work')).toBeInTheDocument()
        expect(within(mgr).getByText('Home')).toBeInTheDocument()
    })

    it('adds a new calendar', async () => {
        renderCalendar()
        fireEvent.click(screen.getByRole('button', {name: 'Calendars'}))
        fireEvent.click(await screen.findByRole('button', {name: /New calendar/}))
        fireEvent.change(screen.getByPlaceholderText('Calendar name'), {target: {value: 'Personal'}})
        fireEvent.click(screen.getByRole('button', {name: 'Add calendar'}))
        await waitFor(() => expect(apiSpies.saveCalendar).toHaveBeenCalledWith(expect.objectContaining({name: 'Personal'})))
    })

    it('deletes a calendar after confirming', async () => {
        apiSpies.listCalendars.mockResolvedValue([makeCalendar('c1', 'Work')])
        const {onChanged} = renderCalendar()
        await waitFor(() => expect(apiSpies.listCalendars).toHaveBeenCalled())
        fireEvent.click(screen.getByRole('button', {name: 'Calendars'}))
        fireEvent.click(await screen.findByRole('button', {name: 'Work'}))
        fireEvent.click(screen.getByRole('button', {name: 'Delete'}))
        const confirm = screen.getByRole('alertdialog', {name: 'Delete calendar'})
        fireEvent.click(within(confirm).getByRole('button', {name: 'Delete'}))
        await waitFor(() => expect(apiSpies.deleteCalendar).toHaveBeenCalledWith('c1'))
        expect(onChanged).toHaveBeenCalled()
    })
})

describe('CalendarModal: open from reminder', () => {
    it('opens the event dialog when launched from a reminder', async () => {
        const ev = makeEvent({id: 'rem1', summary: 'Dentist'})
        apiSpies.listEventInstances.mockResolvedValue([makeInstance(ev)])
        renderCalendar({events: [ev], initialEventId: 'rem1'})
        expect(await screen.findByRole('dialog', {name: 'Edit event'})).toBeInTheDocument()
        expect(screen.getByDisplayValue('Dentist')).toBeInTheDocument()
    })
})

describe('CalendarModal: error handling', () => {
    it('surfaces an instance load error in the banner', async () => {
        apiSpies.listEventInstances.mockRejectedValue('load failed')
        renderCalendar()
        expect(await screen.findByText('load failed')).toBeInTheDocument()
    })
})
