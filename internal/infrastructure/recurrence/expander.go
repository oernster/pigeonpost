// Package recurrence expands recurring calendar events into concrete occurrences. It implements the
// application.RecurrenceExpander port using the rrule-go library, keeping RRULE parsing out of the
// domain so the domain stays free of third-party dependencies.
package recurrence

import (
	"fmt"
	"strings"
	"time"

	rrule "github.com/teambition/rrule-go"

	"github.com/oernster/pigeonpost/internal/domain"
)

// rrulePrefix is the ICS property name that may prefix a stored rule value; rrule-go expects the value
// alone, so it is stripped when present.
const rrulePrefix = "RRULE:"

// Expander turns a recurring event into its occurrences. It holds no state and is safe to share.
type Expander struct{}

// New returns a recurrence expander.
func New() *Expander { return &Expander{} }

// Expand returns the occurrences of a recurring event whose start falls within the inclusive window
// [from, to]. Each occurrence carries the source event, its own start, an end of start plus the event's
// duration (the zero time when the event has no end) and a RecurrenceID equal to its start. An invalid
// rule yields an error so the caller can decide how to degrade.
func (e *Expander) Expand(event domain.Event, from, to time.Time) ([]domain.EventInstance, error) {
	set, err := buildSet(event, locationOf(event.TimeZone()))
	if err != nil {
		return nil, err
	}
	hasEnd := event.HasEnd()
	duration := event.Duration()
	// Set.Between returns occurrences sorted and already de-duplicated (an RDATE equal to a rule
	// occurrence is emitted once), so each start maps directly to one instance. Between compares instants,
	// so the window can stay in UTC while the set generates in the event's zone.
	starts := set.Between(from, to, true)
	instances := make([]domain.EventInstance, 0, len(starts))
	for _, start := range starts {
		// The occurrence is generated in the event's zone; store it as the absolute instant so downstream
		// code stays zone-agnostic and daylight-saving shifts are already baked in.
		occurrence := start.UTC()
		end := time.Time{}
		if hasEnd {
			end = occurrence.Add(duration)
		}
		instances = append(instances, domain.NewEventInstance(event, occurrence, end, occurrence))
	}
	return instances, nil
}

// TruncateBefore rewrites rule so the series ends before at: any COUNT is dropped and an UNTIL of one
// second before at is set, so the occurrence at at and every later one fall away. All other parts of the
// rule (FREQ, INTERVAL, BYDAY and the rest) are preserved.
func (e *Expander) TruncateBefore(rule string, at time.Time) (string, error) {
	rule = strings.TrimSpace(rule)
	rule = strings.TrimPrefix(rule, rrulePrefix)
	parsed, err := rrule.StrToRRule(rule)
	if err != nil {
		return "", fmt.Errorf("recurrence: parse rule %q: %w", rule, err)
	}
	option := parsed.OrigOptions
	option.Count = 0
	option.Until = at.Add(-time.Second).UTC()
	return option.RRuleString(), nil
}

// buildSet assembles an rrule.Set from the event's rule and recurrence dates, anchored to the event start
// expressed in loc, so occurrences keep the same wall-clock time across daylight-saving changes. When the
// event carries no rule the start is added as an RDATE so it remains an occurrence, per RFC 5545 where
// DTSTART is always part of the recurrence set.
func buildSet(event domain.Event, loc *time.Location) (*rrule.Set, error) {
	set := &rrule.Set{}
	start := event.Start().In(loc)
	if rule := strings.TrimSpace(event.Recurrence()); rule != "" {
		rule = strings.TrimPrefix(rule, rrulePrefix)
		parsed, err := rrule.StrToRRule(rule)
		if err != nil {
			return nil, fmt.Errorf("recurrence: parse rule %q: %w", rule, err)
		}
		set.RRule(parsed)
	} else {
		set.RDate(start)
	}
	set.DTStart(start)
	for _, d := range event.RDates() {
		set.RDate(d.In(loc))
	}
	for _, d := range event.ExDates() {
		set.ExDate(d.In(loc))
	}
	return set, nil
}

// locationOf loads the IANA zone, falling back to UTC for an empty name or an unknown zone so a bad zone
// degrades to floating time rather than failing expansion.
func locationOf(zone string) *time.Location {
	if zone == "" {
		return time.UTC
	}
	if loc, err := time.LoadLocation(zone); err == nil {
		return loc
	}
	return time.UTC
}
