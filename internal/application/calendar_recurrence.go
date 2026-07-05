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
				if suppressed[inst.RecurrenceID().UnixMilli()] {
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

// overrideStarts returns the set of occurrence starts (Unix milliseconds) that an override replaces, so
// the generated occurrence at each is suppressed in favour of the override.
func overrideStarts(events []domain.Event) map[int64]bool {
	out := map[int64]bool{}
	for _, e := range events {
		if e.IsOverride() {
			out[e.RecurrenceID().UnixMilli()] = true
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

// UpdateEventScope applies an edit to a recurring series at the given scope. The occurrence is the
// original start of the instance being edited. in carries the edited fields; its ID and UID identify the
// series master. ScopeAll rewrites the master, ScopeThis writes a single-occurrence override, and
// ScopeFuture splits the series so the occurrence and later ones form a new series carrying the edit.
func (s *CalendarService) UpdateEventScope(ctx context.Context, scope EventScope, in EventInput, occurrence time.Time) error {
	master, err := s.resolveMaster(ctx, in.ID)
	if err != nil {
		return err
	}
	switch scope {
	case ScopeAll:
		return s.saveEditedMaster(ctx, master, in)
	case ScopeThis:
		return s.saveOccurrenceOverride(ctx, master, in, occurrence)
	case ScopeFuture:
		return s.splitSeries(ctx, master, in, occurrence)
	default:
		return fmt.Errorf("calendar: unknown edit scope %d", scope)
	}
}

// saveEditedMaster applies the editable fields from in to the master while preserving its recurrence
// dates, UID and preserved ICS, then saves it. Used for a whole-series edit.
func (s *CalendarService) saveEditedMaster(ctx context.Context, master domain.Event, in EventInput) error {
	edited, err := domain.NewEvent(domain.EventInput{
		ID:          master.ID(),
		UID:         master.UID(),
		CalendarID:  in.CalendarID,
		Summary:     in.Summary,
		Description: in.Description,
		Location:    in.Location,
		Start:       in.Start,
		End:         in.End,
		AllDay:      in.AllDay,
		Recurrence:  in.Recurrence,
		RDates:      master.RDates(),
		ExDates:     master.ExDates(),
		Extra:       master.Extra(),
	})
	if err != nil {
		return fmt.Errorf("calendar: build edited series: %w", err)
	}
	if err := s.store.SaveEvent(ctx, edited); err != nil {
		return fmt.Errorf("calendar: save edited series: %w", err)
	}
	return nil
}

// saveOccurrenceOverride writes (or updates) a single-occurrence override that replaces the occurrence,
// sharing the master's series key and carrying the edited fields but no recurrence rule.
func (s *CalendarService) saveOccurrenceOverride(ctx context.Context, master domain.Event, in EventInput, occurrence time.Time) error {
	overrides, err := s.seriesOverrides(ctx, master)
	if err != nil {
		return err
	}
	id := s.newID()
	for _, o := range overrides {
		if o.RecurrenceID().Equal(occurrence) {
			id = o.ID()
			break
		}
	}
	override, err := domain.NewEvent(domain.EventInput{
		ID:           id,
		UID:          seriesKey(master),
		CalendarID:   in.CalendarID,
		Summary:      in.Summary,
		Description:  in.Description,
		Location:     in.Location,
		Start:        in.Start,
		End:          in.End,
		AllDay:       in.AllDay,
		RecurrenceID: occurrence,
	})
	if err != nil {
		return fmt.Errorf("calendar: build occurrence override: %w", err)
	}
	if err := s.store.SaveEvent(ctx, override); err != nil {
		return fmt.Errorf("calendar: save occurrence override: %w", err)
	}
	return nil
}

// splitSeries truncates the master so it ends before the occurrence, then creates a new series from the
// occurrence onward carrying the edit, and moves any overrides at or after the occurrence to it. When the
// occurrence is the master's own start the whole series is edited instead, as there is nothing to keep
// before it.
func (s *CalendarService) splitSeries(ctx context.Context, master domain.Event, in EventInput, occurrence time.Time) error {
	if !occurrence.After(master.Start()) {
		return s.saveEditedMaster(ctx, master, in)
	}
	truncatedRule, err := s.recurrence.TruncateBefore(master.Recurrence(), occurrence)
	if err != nil {
		return fmt.Errorf("calendar: truncate series: %w", err)
	}
	if err := s.store.SaveEvent(ctx, master.WithRecurrence(truncatedRule)); err != nil {
		return fmt.Errorf("calendar: save truncated series: %w", err)
	}
	newUID := s.newID()
	newSeries, err := domain.NewEvent(domain.EventInput{
		ID:          s.newID(),
		UID:         newUID,
		CalendarID:  in.CalendarID,
		Summary:     in.Summary,
		Description: in.Description,
		Location:    in.Location,
		Start:       in.Start,
		End:         in.End,
		AllDay:      in.AllDay,
		Recurrence:  in.Recurrence,
	})
	if err != nil {
		return fmt.Errorf("calendar: build split series: %w", err)
	}
	if err := s.store.SaveEvent(ctx, newSeries); err != nil {
		return fmt.Errorf("calendar: save split series: %w", err)
	}
	return s.migrateOverrides(ctx, master, newUID, occurrence)
}

// migrateOverrides moves every override of the master at or after the occurrence onto the new series uid,
// so a this-and-future split keeps the future exceptions with the future series.
func (s *CalendarService) migrateOverrides(ctx context.Context, master domain.Event, newUID string, occurrence time.Time) error {
	overrides, err := s.seriesOverrides(ctx, master)
	if err != nil {
		return err
	}
	for _, o := range overrides {
		if o.RecurrenceID().Before(occurrence) {
			continue
		}
		if err := s.store.SaveEvent(ctx, o.WithUID(newUID)); err != nil {
			return fmt.Errorf("calendar: move override %q: %w", o.ID(), err)
		}
	}
	return nil
}

// DeleteEventScope removes part or all of a recurring series. The occurrence is the original start of the
// instance acted on. ScopeThis excludes just that occurrence (and drops any override for it), ScopeFuture
// ends the series before it (dropping later overrides), and ScopeAll removes the master and every
// override.
func (s *CalendarService) DeleteEventScope(ctx context.Context, scope EventScope, seriesID string, occurrence time.Time) error {
	master, err := s.resolveMaster(ctx, seriesID)
	if err != nil {
		return err
	}
	switch scope {
	case ScopeAll:
		return s.deleteWholeSeries(ctx, master)
	case ScopeThis:
		return s.excludeOccurrence(ctx, master, occurrence)
	case ScopeFuture:
		return s.endSeriesBefore(ctx, master, occurrence)
	default:
		return fmt.Errorf("calendar: unknown delete scope %d", scope)
	}
}

// deleteWholeSeries removes the master and all of its overrides.
func (s *CalendarService) deleteWholeSeries(ctx context.Context, master domain.Event) error {
	overrides, err := s.seriesOverrides(ctx, master)
	if err != nil {
		return err
	}
	for _, o := range overrides {
		if err := s.store.DeleteEvent(ctx, o.ID()); err != nil {
			return fmt.Errorf("calendar: delete override %q: %w", o.ID(), err)
		}
	}
	if err := s.store.DeleteEvent(ctx, master.ID()); err != nil {
		return fmt.Errorf("calendar: delete series %q: %w", master.ID(), err)
	}
	return nil
}

// excludeOccurrence adds the occurrence to the master's excluded dates and drops any override for it, so
// the single occurrence disappears while the rest of the series stays.
func (s *CalendarService) excludeOccurrence(ctx context.Context, master domain.Event, occurrence time.Time) error {
	if err := s.deleteOverrideAt(ctx, master, occurrence); err != nil {
		return err
	}
	excluded := append(master.ExDates(), occurrence)
	if err := s.store.SaveEvent(ctx, master.WithExDates(excluded)); err != nil {
		return fmt.Errorf("calendar: exclude occurrence: %w", err)
	}
	return nil
}

// endSeriesBefore truncates the master so it ends before the occurrence and drops overrides at or after
// it. When the occurrence is the master's own start the whole series is removed instead.
func (s *CalendarService) endSeriesBefore(ctx context.Context, master domain.Event, occurrence time.Time) error {
	if !occurrence.After(master.Start()) {
		return s.deleteWholeSeries(ctx, master)
	}
	truncatedRule, err := s.recurrence.TruncateBefore(master.Recurrence(), occurrence)
	if err != nil {
		return fmt.Errorf("calendar: truncate series: %w", err)
	}
	overrides, err := s.seriesOverrides(ctx, master)
	if err != nil {
		return err
	}
	for _, o := range overrides {
		if o.RecurrenceID().Before(occurrence) {
			continue
		}
		if err := s.store.DeleteEvent(ctx, o.ID()); err != nil {
			return fmt.Errorf("calendar: delete future override %q: %w", o.ID(), err)
		}
	}
	if err := s.store.SaveEvent(ctx, master.WithRecurrence(truncatedRule)); err != nil {
		return fmt.Errorf("calendar: save truncated series: %w", err)
	}
	return nil
}

// deleteOverrideAt removes the override that replaces the given occurrence, if one exists.
func (s *CalendarService) deleteOverrideAt(ctx context.Context, master domain.Event, occurrence time.Time) error {
	overrides, err := s.seriesOverrides(ctx, master)
	if err != nil {
		return err
	}
	for _, o := range overrides {
		if o.RecurrenceID().Equal(occurrence) {
			if err := s.store.DeleteEvent(ctx, o.ID()); err != nil {
				return fmt.Errorf("calendar: delete override %q: %w", o.ID(), err)
			}
		}
	}
	return nil
}

// resolveMaster returns the recurring master of the series the given event belongs to, so a scope edit or
// delete works whether the caller passes the master id or the id of one of its overrides. A recurring,
// non-override event is its own master; otherwise the series (matched by series key) is searched for the
// master, falling back to the event itself when the series has none.
func (s *CalendarService) resolveMaster(ctx context.Context, id string) (domain.Event, error) {
	event, err := s.store.GetEvent(ctx, id)
	if err != nil {
		return domain.Event{}, fmt.Errorf("calendar: load series %q: %w", id, err)
	}
	if event.IsRecurring() && !event.IsOverride() {
		return event, nil
	}
	all, err := s.store.ListEvents(ctx)
	if err != nil {
		return domain.Event{}, fmt.Errorf("calendar: resolve series master: %w", err)
	}
	key := seriesKey(event)
	for _, candidate := range all {
		if candidate.IsRecurring() && !candidate.IsOverride() && seriesKey(candidate) == key {
			return candidate, nil
		}
	}
	return event, nil
}

// seriesOverrides returns the override events that belong to the master's series (sharing its series key).
func (s *CalendarService) seriesOverrides(ctx context.Context, master domain.Event) ([]domain.Event, error) {
	all, err := s.store.ListEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("calendar: list series overrides: %w", err)
	}
	key := seriesKey(master)
	var overrides []domain.Event
	for _, e := range all {
		if e.IsOverride() && seriesKey(e) == key {
			overrides = append(overrides, e)
		}
	}
	return overrides, nil
}
