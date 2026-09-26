// The calendar half of the Wails seam: the calendar, event and meeting types and the calls that read and
// write calendars and events, answer and send meeting messages and sync CalDAV accounts. It lives beside
// api.ts rather than inside it for the reason the filter rules do (see apiRules): a cohesive group of
// calls with its own types, taken out of a module that was over the size limit. The api object spreads
// what is exported here, so callers still reach these through api.* and nothing else changes.
import {
    AddCalDAVAccount,
    ApplyMeetingReply,
    DeleteCalendar,
    DeleteEvent,
    DeleteEventScoped,
    ExportEventsToFile,
    GetEvent,
    GetInvitation,
    ImportEventsFromFile,
    ListCalDAVAccounts,
    ListCalendars,
    ListEventInstances,
    ListEvents,
    RemoveCalDAVAccount,
    RemoveCancelledMeeting,
    RespondToInvitation,
    SaveCalendar,
    SaveEvent,
    SaveEventScoped,
    SendMeetingCancel,
    SendMeetingRequest,
    SyncCalDAV,
} from '../wailsjs/go/main/App'
import {main} from '../wailsjs/go/models'

export type Calendar = main.CalendarDTO
// CalDAVAccount is a configured remote CalDAV account. The password is never part of this view; it lives
// in the OS keychain, exactly as for a mail account.
export type CalDAVAccount = main.CalDAVAccountDTO
export type CalendarEvent = main.EventDTO
export type CalendarEventInstance = main.EventInstanceDTO
export type Invitation = main.InvitationDTO
export type MeetingAttendee = main.AttendeeDTO

// PartStat is the ICS PARTSTAT reply value the reader sends when answering a meeting request.
export type PartStat = 'ACCEPTED' | 'DECLINED' | 'TENTATIVE'

// EventScope mirrors the Go application.EventScope: how far an edit or delete of a recurring occurrence
// reaches. The integer values must match the Go constants.
export enum EventScope {
    This = 0,
    Future = 1,
    All = 2,
}

export interface CalendarInput {
    id: string
    name: string
    colour: string
}

// MeetingOrganizerInput is the organiser written onto an event when it is a meeting. An empty address
// marks an ordinary (non-meeting) event.
export interface MeetingOrganizerInput {
    address: string
    commonName: string
}

// MeetingAttendeeInput is one invited party written onto a meeting event. role and status accept the ICS
// ROLE and PARTSTAT values; empty strings take the domain defaults (REQ-PARTICIPANT and NEEDS-ACTION).
export interface MeetingAttendeeInput {
    address: string
    commonName: string
    role: string
    status: string
    rsvp: boolean
}

export interface CalendarEventInput {
    id: string
    uid: string
    calendarId: string
    summary: string
    description: string
    location: string
    // category is the optional short lowercase category value (the primary iCalendar CATEGORIES value);
    // empty means no category.
    category: string
    start: string
    end: string
    allDay: boolean
    recurrence: string
    timeZone: string
    // reminders are lead times in whole minutes before the event start (0 means at the start).
    reminders: number[]
    extra: string
    // organiser and attendees carry the meeting scheduling data. organiser.address is empty and attendees
    // is empty for an ordinary calendar entry.
    organizer: MeetingOrganizerInput
    attendees: MeetingAttendeeInput[]
}

export const calendarApi = {
    listCalendars: (): Promise<Calendar[]> => ListCalendars(),
    saveCalendar: (req: CalendarInput): Promise<void> => SaveCalendar(main.CalendarRequest.createFrom(req)),
    deleteCalendar: (id: string): Promise<void> => DeleteCalendar(id),
    listEvents: (): Promise<CalendarEvent[]> => ListEvents(),
    listEventInstances: (from: string, to: string): Promise<CalendarEventInstance[]> =>
        ListEventInstances(from, to),
    getEvent: (id: string): Promise<CalendarEvent> => GetEvent(id),
    // saveEvent returns the saved event's id (freshly generated for a new event), so a newly created
    // meeting can send its invitations without a reload.
    saveEvent: (req: CalendarEventInput): Promise<string> => SaveEvent(main.EventRequest.createFrom(req)),
    saveEventScoped: (req: CalendarEventInput, scope: EventScope, occurrence: string): Promise<void> =>
        SaveEventScoped(main.EventRequest.createFrom(req), scope, occurrence),
    deleteEvent: (id: string): Promise<void> => DeleteEvent(id),
    deleteEventScoped: (scope: EventScope, seriesId: string, occurrence: string): Promise<void> =>
        DeleteEventScoped(scope, seriesId, occurrence),
    importEventsFromFile: (): Promise<number> => ImportEventsFromFile(),
    exportEventsToFile: (): Promise<boolean> => ExportEventsToFile(),
    getInvitation: (messageId: string): Promise<Invitation> => GetInvitation(messageId),
    respondToInvitation: (messageId: string, status: PartStat): Promise<void> =>
        RespondToInvitation(messageId, status),
    removeCancelledMeeting: (messageId: string): Promise<void> => RemoveCancelledMeeting(messageId),
    applyMeetingReply: (messageId: string): Promise<void> => ApplyMeetingReply(messageId),
    sendMeetingRequest: (accountId: string, eventId: string): Promise<void> =>
        SendMeetingRequest(accountId, eventId),
    sendMeetingCancel: (accountId: string, eventId: string): Promise<void> =>
        SendMeetingCancel(accountId, eventId),
    // CalDAV remote calendars. listCalDAVAccounts returns the configured DAV accounts; addCalDAVAccount stores
    // an account and its keychain password; removeCalDAVAccount deletes both; syncCalDAV runs the two-way sync
    // for an account (pushes local changes, then reconciles the server's calendars into the local store).
    listCalDAVAccounts: (): Promise<CalDAVAccount[]> => ListCalDAVAccounts(),
    addCalDAVAccount: (displayName: string, baseUrl: string, username: string, password: string): Promise<void> =>
        AddCalDAVAccount(displayName, baseUrl, username, password),
    removeCalDAVAccount: (id: string): Promise<void> => RemoveCalDAVAccount(id),
    syncCalDAV: (id: string): Promise<void> => SyncCalDAV(id),
}
