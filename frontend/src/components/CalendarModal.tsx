import {api, CalendarEvent, EventScope} from '../api'
import {categoryEmoji} from '../categories'
import {ModalClose} from './ModalClose'
import {ScopeChooser} from './ScopeChooser'
import {CalendarTimeGrid} from './CalendarTimeGrid'
import {useBackdropDismiss, useEscapeToClose} from './useBackdropDismiss'
import {
    DAYS_IN_WEEK,
    DEFAULT_EVENT_COLOUR,
    MONTH_MAX_LANES,
    VIEW_MODES,
    WEEKDAYS,
    contrastInk,
    dayIndex,
    eventDaySpan,
    layoutWeek,
    monthCells,
    pad,
    weekDays,
} from '../calendarModel'
import {useEventInstances} from '../hooks/useEventInstances'
import {useCalendars} from '../hooks/useCalendars'
import {useCalDAVAccounts} from '../hooks/useCalDAVAccounts'
import {useOpenFromReminder} from '../hooks/useOpenFromReminder'
import {CalendarsManager} from './CalendarsManager'
import {CalDAVAccountsManager} from './CalDAVAccountsManager'
import {EventFormModal} from './EventFormModal'
import {useBanners} from '../hooks/useBanners'
import {useCalendarView} from '../hooks/useCalendarView'
import {isSeries, useEventFormOpening} from '../hooks/useEventFormOpening'


interface CalendarModalProps {
    events: CalendarEvent[]
    // accountId, accountEmail and accountName identify the active account that organises meetings: the
    // organiser written onto a meeting and the sender of its invitations. accountId is empty when no
    // account is selected, which disables sending.
    accountId: string
    accountEmail: string
    accountName: string
    // initialEventId, when set, opens the calendar with that event's dialog already showing. It is how a
    // clicked reminder lands on the event it is about.
    initialEventId?: string
    onChanged: () => void
    onClose: () => void
}

// CalendarModal shows a month view of events and edits them. It imports and exports iCalendar (.ics) so
// events round-trip with Outlook and Thunderbird. An event with attendees is a meeting: its invitations
// and cancellations are emailed through the active account. Deletion is always confirmed.
export function CalendarModal({events, accountId, accountEmail, accountName, initialEventId, onChanged, onClose}: CalendarModalProps) {
    const dismiss = useBackdropDismiss(onClose)
    // Where the calendar is looking (the date, month or week or day, the header and stepping) is its own hook.
    const {viewDate, setViewDate, viewMode, setViewMode, shift, headerLabel, openDay} = useCalendarView()
    // error, status and busy are the shared user-feedback banners, owned in one hook and read and driven by
    // the calendar shell, the event form and the calendars manager alike.
    const banners = useBanners()
    const {error, status, busy, setError, setStatus, setBusy} = banners
    // The calendars sub-feature (the list, the manager's open state, the calendar being edited and the one
    // pending deletion, plus save and delete) is its own hook; the list colours events and seeds a new event.
    const {
        calendars, managingCals, setManagingCals, calForm, setCalForm, saveCal,
        pendingCalDelete, setPendingCalDelete, confirmCalDelete,
    } = useCalendars({setError, setBusy, onChanged})
    // The event dialog (what it is open on, the attendee being typed, a sent cancellation and the recurring
    // occurrence waiting on a scope) and the ways it opens are their own hook.
    const {
        form, setForm, attendeeDraft, setAttendeeDraft, cancelledSent, setCancelledSent, editScope, setEditScope,
        openNew, openAt, openInstance, openForm, chooseEditScope,
    } = useEventFormOpening({calendars, setError, setStatus})
    // instances are the concrete occurrences shown for the visible range, expanded from the recurring events
    // by the backend and refetched by the application hook; bumpReload forces a refetch after a local change.
    const {instances, bumpReload} = useEventInstances({viewDate, viewMode, events, setError})
    // The remote-calendars (CalDAV) sub-feature: the DAV accounts, the manager's open state, the add form and
    // the two-way sync. A sync reconciles remote events into the local store, so it reloads the calendar.
    const caldav = useCalDAVAccounts({
        setError, setStatus, setBusy, onSynced: () => {
            bumpReload()
            onChanged()
        },
    })

    // The event form and the calendars manager are nested modals that are deliberately not dismissed by a
    // backdrop click (so edits are not dropped), so give each its own Escape close. The active flags mean
    // Escape closes whichever is open before falling through to close the calendar itself.
    useEscapeToClose(() => setForm(null), form !== null)
    // Escape must reset the edit / add form exactly as the close cross and the Done button do (both route
    // through the manager's onClose), so a dismissed edit does not survive and reappear on reopen.
    useEscapeToClose(() => {
        setManagingCals(false)
        setCalForm(null)
    }, managingCals)
    useEscapeToClose(() => {
        caldav.setManaging(false)
        caldav.cancelAdd()
    }, caldav.managing)

    // colourOf resolves an event's colour from its calendar, falling back to the default for events with
    // no calendar. The map is rebuilt each render, which is cheap for the handful of calendars a user has.
    const colourById = new Map(calendars.map((c) => [c.id, c.colour || DEFAULT_EVENT_COLOUR]))
    const colourOf = (e: CalendarEvent) => colourById.get(e.calendarId) ?? DEFAULT_EVENT_COLOUR

    const cells = monthCells(viewDate)
    // spanned reduces each occurrence to the inclusive day range it covers, so the month grid can lay a
    // multi-day event out as one bar across those days rather than a mark on its start day alone.
    const spanned = instances.map((i) => {
        const start = new Date(i.start)
        const {firstDay, lastDay} = eventDaySpan(start.getTime(), new Date(i.end).getTime())
        return {i, start, firstDay, lastDay, key: `${i.event.id}@${i.start}`}
    })
    const barInputs = spanned.map((s) => ({key: s.key, firstDay: s.firstDay, lastDay: s.lastDay}))
    const spannedByKey = new Map(spanned.map((s) => [s.key, s]))
    // A clicked reminder lands on the event it is about: the hook jumps the view to the event and reveals its
    // dialog once the occurrence has loaded. A recurring event opens at series scope so a save reaches the
    // master; a one-off opens directly.
    useOpenFromReminder({
        initialEventId, events, instances, setViewDate,
        onReveal: (inst) => openForm(inst, isSeries(inst) ? EventScope.All : null),
    })

    const doImport = async () => {
        setError('')
        setStatus('')
        try {
            const n = await api.importEventsFromFile()
            if (n > 0) {
                setStatus(`Imported ${n} event${n === 1 ? '' : 's'}.`)
                bumpReload()
                onChanged()
            }
        } catch (e) {
            setError(String(e))
        }
    }

    const doExport = async () => {
        setError('')
        setStatus('')
        try {
            const written = await api.exportEventsToFile()
            if (written) setStatus(`Exported ${events.length} event${events.length === 1 ? '' : 's'}.`)
        } catch (e) {
            setError(String(e))
        }
    }

    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal calendar-modal pinned-actions" role="dialog" aria-label="Calendar" onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onClose}/>
                <h2 className="modal-title">Calendar</h2>
                {error && <div className="compose-error">{error}</div>}
                {status && <div className="setup-hint">{status}</div>}

                <div className="modal-actions cal-toolbar">
                    <button className="btn" aria-label="Previous" onClick={() => shift(-1)}>‹</button>
                    <span className="cal-month">{headerLabel()}</span>
                    <button className="btn" aria-label="Next" onClick={() => shift(1)}>›</button>
                    <button className="btn" onClick={() => setViewDate(new Date())}>Today</button>
                    <span className="cal-viewswitch">
                        {VIEW_MODES.map((m) => (
                            <button key={m} className={'btn cal-view-btn' + (viewMode === m ? ' active' : '')}
                                    aria-pressed={viewMode === m} onClick={() => setViewMode(m)}>
                                {m.charAt(0).toUpperCase() + m.slice(1)}
                            </button>
                        ))}
                    </span>
                    <span className="cal-spacer"/>
                    <button className="btn" onClick={() => setManagingCals(true)}>Calendars</button>
                    <button className="btn" onClick={() => caldav.setManaging(true)}>Remote calendars</button>
                    <button className="btn" onClick={() => void doImport()}>Import…</button>
                    <button className="btn" onClick={() => void doExport()} disabled={events.length === 0}>Export ICS</button>
                </div>

                <div className="modal-body">
                {viewMode === 'month' ? (
                    <div className="cal-grid">
                        {WEEKDAYS.map((w) => (<div key={w} className="cal-weekday">{w}</div>))}
                        {Array.from({length: cells.length / DAYS_IN_WEEK}, (_, wi) => {
                            const week = cells.slice(wi * DAYS_IN_WEEK, wi * DAYS_IN_WEEK + DAYS_IN_WEEK)
                            const {bars, lanes, overflow} = layoutWeek(dayIndex(week[0]), barInputs, MONTH_MAX_LANES)
                            const hasMore = overflow.some((n) => n > 0)
                            const gridTemplateRows =
                                `auto repeat(${lanes}, var(--cal-lane))${hasMore ? ' var(--cal-lane)' : ''} 1fr`
                            return (
                                <div key={wi} className="cal-week" style={{gridTemplateRows}}>
                                    {week.map((day, di) => (
                                        <div key={`c${di}`}
                                             className={'cal-cell' + (day.getMonth() === viewDate.getMonth() ? '' : ' cal-cell-dim')}
                                             style={{gridColumn: di + 1, gridRow: '1 / -1'}}
                                             onClick={() => openNew(day)}/>
                                    ))}
                                    {week.map((day, di) => (
                                        <button key={`d${di}`} className="cal-daynum" title="Open day view"
                                                style={{gridColumn: di + 1, gridRow: 1}}
                                                onClick={(ev) => {
                                                    ev.stopPropagation()
                                                    openDay(day)
                                                }}>{day.getDate()}</button>
                                    ))}
                                    {bars.map((b) => {
                                        const s = spannedByKey.get(b.key)!
                                        const ev = s.i.event
                                        const colour = colourOf(ev)
                                        const isSpan = b.startCol !== b.endCol || b.continuesLeft || b.continuesRight
                                        const showTime = !ev.allDay && !b.continuesLeft
                                        const cls = 'cal-bar ' + (isSpan ? 'cal-bar-span' : 'cal-bar-chip') +
                                            (b.continuesLeft ? ' cont-left' : '') + (b.continuesRight ? ' cont-right' : '')
                                        const gc = `${b.startCol + 1} / ${b.endCol + 2}`
                                        return (
                                            <button key={b.key} className={cls} title={ev.summary}
                                                    style={isSpan
                                                        ? {gridColumn: gc, gridRow: b.lane + 2, background: colour, color: contrastInk(colour)}
                                                        : {gridColumn: gc, gridRow: b.lane + 2, borderLeft: `3px solid ${colour}`}}
                                                    onClick={(evt) => {
                                                        evt.stopPropagation()
                                                        openInstance(s.i)
                                                    }}>
                                                {showTime ? `${pad(s.start.getHours())}:${pad(s.start.getMinutes())} ` : ''}{categoryEmoji(ev.category) && `${categoryEmoji(ev.category)} `}{ev.summary}
                                            </button>
                                        )
                                    })}
                                    {overflow.map((n, di) => (n > 0 ? (
                                        <button key={`m${di}`} className="cal-more"
                                                style={{gridColumn: di + 1, gridRow: lanes + 2}}
                                                onClick={(ev) => {
                                                    ev.stopPropagation()
                                                    openDay(week[di])
                                                }}>+{n} more</button>
                                    ) : null))}
                                </div>
                            )
                        })}
                    </div>
                ) : (
                    <CalendarTimeGrid
                        days={viewMode === 'week' ? weekDays(viewDate) : [viewDate]}
                        instances={instances}
                        colourOf={colourOf}
                        onNewAt={openAt}
                        onEdit={openInstance}
                    />
                )}

                </div>
                <div className="modal-actions spread">
                    <button className="btn" onClick={onClose}>Close</button>
                    <button className="btn primary" onClick={() => openNew(new Date())}>New event</button>
                </div>
            </div>

            {form && (
                <EventFormModal
                    form={form}
                    setForm={setForm}
                    calendars={calendars}
                    accountId={accountId}
                    accountEmail={accountEmail}
                    accountName={accountName}
                    attendeeDraft={attendeeDraft}
                    setAttendeeDraft={setAttendeeDraft}
                    cancelledSent={cancelledSent}
                    setCancelledSent={setCancelledSent}
                    banners={banners}
                    onChanged={onChanged}
                    bumpReload={bumpReload}
                />
            )}

            {editScope && (
                <ScopeChooser
                    title="Edit recurring event"
                    message={`"${editScope.event.summary}" repeats. Which events should this change apply to?`}
                    busy={busy}
                    onChoose={chooseEditScope}
                    onCancel={() => setEditScope(null)}
                />
            )}

            {managingCals && (
                <CalendarsManager
                    calendars={calendars}
                    calForm={calForm}
                    setCalForm={setCalForm}
                    pendingCalDelete={pendingCalDelete}
                    setPendingCalDelete={setPendingCalDelete}
                    saveCal={saveCal}
                    confirmCalDelete={confirmCalDelete}
                    onClose={() => {
                        setManagingCals(false)
                        setCalForm(null)
                    }}
                    busy={busy}
                />
            )}

            {caldav.managing && (
                <CalDAVAccountsManager
                    accounts={caldav.accounts}
                    adding={caldav.adding}
                    startAdd={caldav.startAdd}
                    cancelAdd={caldav.cancelAdd}
                    form={caldav.form}
                    setForm={caldav.setForm}
                    submitAdd={caldav.submitAdd}
                    sync={caldav.sync}
                    syncingId={caldav.syncingId}
                    pendingDelete={caldav.pendingDelete}
                    setPendingDelete={caldav.setPendingDelete}
                    confirmRemove={caldav.confirmRemove}
                    onClose={() => {
                        caldav.setManaging(false)
                        caldav.cancelAdd()
                    }}
                    busy={busy}
                    error={error}
                    status={status}
                />
            )}
        </div>
    )
}
