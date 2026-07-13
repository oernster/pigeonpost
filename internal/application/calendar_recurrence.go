package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// EventScope selects how far an edit or delete of a recurring occurrence reaches.
type EventScope int

const (
	// ScopeThis affects only the single occurrence.
	ScopeThis EventScope = iota
	// ScopeFuture affects the occurrence and every later one.
	ScopeFuture
	// ScopeAll affects the whole series.
	ScopeAll
)

// ListEventInstances expands every stored event into the concrete occurrences whose time falls within
// the inclusive window [from, to], sorted by start. A recurring event is expanded by the recurrence
// service; an override (RECURRENCE-ID) replaces the generated occurrence it matches; a non-recurring
// event yields a single instance. A malformed rule degrades to a single instance so the event is not
// lost.
func (s *CalendarService) ListEventInstances(ctx context.Context, from, to time.Time) ([]domain.EventInstance, error) {
	events, err := s.store.ListEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("calendar: list events: %w", err)
	}
	masters, direct := groupBySeries(events)
	var instances []domain.EventInstance
	for key, ms := range masters {
		suppressed := overrideStarts(direct[key])
		for _, master := range ms {
			expanded, expandErr := s.recurrence.Expand(master, from, to)
			if expandErr != nil {
				if inst, ok := windowedInstance(master, master.RecurrenceID(), from, to); ok {
					instances = append(instances, inst)
				}
				continue
			}
			for _, inst := range expanded {
				if suppressed[inst.RecurrenceID().Unix()] {
					continue
				}
				instances = append(instances, inst)
			}
		}
	}
	for _, evs := range direct {
		for _, e := range evs {
			if inst, ok := windowedInstance(e, e.RecurrenceID(), from, to); ok {
				instances = append(instances, inst)
			}
		}
	}
	sort.SliceStable(instances, func(i, j int) bool { return instances[i].Start().Before(instances[j].Start()) })
	return instances, nil
}

// groupBySeries splits events by their series key (UID, or id when there is no UID) into the recurring
// masters and the direct events (overrides and non-recurring singletons) that are emitted as-is.
func groupBySeries(events []domain.Event) (masters, direct map[string][]domain.Event) {
	masters = map[string][]domain.Event{}
	direct = map[string][]domain.Event{}
	for _, e := range events {
		key := seriesKey(e)
		switch {
		case e.IsOverride():
			direct[key] = append(direct[key], e)
		case e.IsRecurring():
			masters[key] = append(masters[key], e)
		default:
			direct[key] = append(direct[key], e)
		}
	}
	return masters, direct
}

// seriesKey groups a master with its overrides. Overrides share the master's UID; an in-app series with
// no UID falls back to the master id, which its overrides also carry.
func seriesKey(e domain.Event) string {
	if e.UID() != "" {
		return e.UID()
	}
	return e.ID()
}

// overrideStarts returns the set of occurrence starts that an override replaces, keyed at whole-second
// resolution so the generated occurrence at each is suppressed in favour of the override. iCalendar
// date-times carry no sub-second component, so matching on the second (not the millisecond) is the data's
// native precision: it drops a stray sub-second difference between a RECURRENCE-ID and the generated
// occurrence that would otherwise leave both showing as a duplicate.
func overrideStarts(events []domain.Event) map[int64]bool {
	out := map[int64]bool{}
	for _, e := range events {
		if e.IsOverride() {
			out[e.RecurrenceID().Unix()] = true
		}
	}
	return out
}

// windowedInstance returns the event as a single instance when it overlaps [from, to], carrying the
// given recurrence id.
func windowedInstance(e domain.Event, recurrenceID, from, to time.Time) (domain.EventInstance, bool) {
	if !overlapsWindow(e.Start(), e.End(), from, to) {
		return domain.EventInstance{}, false
	}
	return domain.NewEventInstance(e, e.Start(), e.End(), recurrenceID), true
}

// overlapsWindow reports whether an event with the given start and end (a zero end meaning a
// point-in-time event) intersects the inclusive window [from, to].
func overlapsWindow(start, end, from, to time.Time) bool {
	if start.After(to) {
		return false
	}
	effectiveEnd := end
	if effectiveEnd.IsZero() {
		effectiveEnd = start
	}
	return !effectiveEnd.Before(from)
}

// The reminder scan horizon must cover the longest reminder lead the calendar UI offers, so a due
// reminder for a near-future occurrence is found without expanding the search forever. Expressing it from
// named parts (rather than a bare multiplier) keeps its relationship to that longest lead explicit: if the
// UI ever offers a longer reminder, longestReminderLead is the single value to raise.
const (
	// longestReminderLead is the longest lead the reminder UI offers (one week before the event).
	longestReminderLead = 7 * 24 * time.Hour
	// reminderLeadMargin is a day of slack beyond the longest lead, so an occurrence at the window edge
	// still surfaces its reminder.
	reminderLeadMargin = 24 * time.Hour
	// maxReminderLead is the horizon DueReminders and PendingReminders expand occurrences to.
	maxReminderLead = longestReminderLead + reminderLeadMargin
)

// DueReminder is a reminder that has come due: which event and occurrence it belongs to and when it fired.
type DueReminder struct {
	EventID         string
	Summary         string
	OccurrenceStart time.Time
	TriggerAt       time.Time
}

// DueReminders returns the reminders whose trigger time falls in the half-open window (since, now], across
// every event and its expanded occurrences. The scheduler calls it each tick with the interval since its
// last check, so a reminder is reported exactly once as the window advances.
func (s *CalendarService) DueReminders(ctx context.Context, since, now time.Time) ([]DueReminder, error) {
	instances, err := s.ListEventInstances(ctx, since, now.Add(maxReminderLead))
	if err != nil {
		return nil, fmt.Errorf("calendar: due reminders: %w", err)
	}
	var due []DueReminder
	for _, inst := range instances {
		for _, alarm := range inst.Event().Alarms() {
			trigger := alarm.TriggerAt(inst.Start())
			if trigger.After(since) && !trigger.After(now) {
				due = append(due, newDueReminder(inst, trigger))
			}
		}
	}
	return due, nil
}

// PendingReminders returns reminders for still-upcoming occurrences (starting at or after now, within the
// reminder lead) whose trigger time has already passed. It is called once at launch so a reminder for an
// imminent event is not silently missed when the app was not running at its trigger time; a reminder for
// an event that has already started or passed is not resurrected. The recurring DueReminders check then
// covers triggers that fall due while the app runs, and the two windows do not overlap.
func (s *CalendarService) PendingReminders(ctx context.Context, now time.Time) ([]DueReminder, error) {
	instances, err := s.ListEventInstances(ctx, now, now.Add(maxReminderLead))
	if err != nil {
		return nil, fmt.Errorf("calendar: pending reminders: %w", err)
	}
	var due []DueReminder
	for _, inst := range instances {
		for _, alarm := range inst.Event().Alarms() {
			trigger := alarm.TriggerAt(inst.Start())
			if !trigger.After(now) {
				due = append(due, newDueReminder(inst, trigger))
			}
		}
	}
	return due, nil
}

// newDueReminder builds a DueReminder for an occurrence and one of its alarm's trigger times.
func newDueReminder(inst domain.EventInstance, trigger time.Time) DueReminder {
	return DueReminder{
		EventID:         inst.Event().ID(),
		Summary:         inst.Event().Summary(),
		OccurrenceStart: inst.Start(),
		TriggerAt:       trigger,
	}
}
