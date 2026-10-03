package ics

import (
	"sort"
	"strconv"
	"strings"
	"time"

	goical "github.com/emersion/go-ical"
	"github.com/teambition/rrule-go"
)

// This file resolves a TZID that is neither an IANA name nor a known Windows name (an Outlook display
// name such as "(UTC+01:00) Amsterdam, Berlin"; equally a zone a calendar invents) through the VTIMEZONE the
// file itself defines for it. In order of preference the definition yields: the IANA zone it names (an
// X-LIC-LOCATION property or an IANA name embedded in the TZID as Mozilla writes it); the IANA zone whose
// offsets and transitions match the definition exactly in the value's year; failing both, the
// definition's own offset at that instant, with the value rewritten to UTC.

// propLicLocation is the non-standard property naming the IANA zone a VTIMEZONE describes.
const propLicLocation = "X-LIC-LOCATION"

// utcDateTimeLayout is the RFC 5545 UTC DATE-TIME form a resolved wall clock is rewritten to.
const utcDateTimeLayout = tzDateTimeLayout + "Z"

// Offset field widths: an RFC 5545 UTC offset is a sign, two hour digits, two minute digits and
// optionally two second digits (+0530, -043017).
const (
	offsetHourEnd  = len("+HH")
	offsetShortLen = len("+HHMM")
	offsetLongLen  = len("+HHMMSS")
)

// zoneObservance is one STANDARD or DAYLIGHT block: the offset change it makes and when it happens.
type zoneObservance struct {
	from, to int            // seconds east of UTC before and after each onset
	start    time.Time      // first onset as a wall clock held in UTC fields
	rule     *rrule.ROption // later onsets, nil when the block names a single onset
	rdates   []time.Time    // extra onsets as wall clocks held in UTC fields
}

// customZone is a VTIMEZONE definition read from the file being imported.
type customZone struct {
	hint        string // IANA zone the definition names, empty when it names none
	observances []zoneObservance
}

// customZones reads every VTIMEZONE in the calendar, keyed by TZID. A definition whose blocks cannot all
// be read keeps only its IANA hint, so a half-understood definition never yields a wrong offset.
func customZones(cal *goical.Calendar) map[string]customZone {
	zones := map[string]customZone{}
	for _, child := range cal.Children {
		if child.Name != goical.CompTimezone {
			continue
		}
		tzid := text(child.Props, goical.PropTimezoneID)
		zone := customZone{hint: zoneHint(child, tzid)}
		for _, sub := range child.Children {
			obs, ok := parseObservance(sub)
			if !ok {
				zone.observances = nil
				break
			}
			zone.observances = append(zone.observances, obs)
		}
		zones[tzid] = zone
	}
	return zones
}

// zoneHint returns the IANA zone a VTIMEZONE names, from X-LIC-LOCATION or from a trailing IANA name
// inside the TZID (/mozilla.org/20050126_1/America/New_York); "" when it names none that loads.
func zoneHint(comp *goical.Component, tzid string) string {
	if loc := strings.TrimSpace(text(comp.Props, propLicLocation)); loc != "" {
		if _, err := time.LoadLocation(loc); err == nil {
			return loc
		}
	}
	parts := strings.Split(tzid, "/")
	// Only suffixes of at least two segments are tried: every IANA area zone has a slash; a single
	// segment would let a bare word like "UTC" or "Local" through.
	for i := 0; i < len(parts)-1; i++ {
		name := strings.Join(parts[i:], "/")
		if parts[i] == "" {
			continue
		}
		if _, err := time.LoadLocation(name); err == nil {
			return name
		}
	}
	return ""
}

// parseObservance reads a STANDARD or DAYLIGHT block, reporting false when any part is unreadable.
func parseObservance(sub *goical.Component) (zoneObservance, bool) {
	startProp := sub.Props.Get(goical.PropDateTimeStart)
	if startProp == nil {
		return zoneObservance{}, false
	}
	start, err := time.ParseInLocation(tzDateTimeLayout, startProp.Value, time.UTC)
	from, fromOK := parseOffset(sub.Props.Get(goical.PropTimezoneOffsetFrom))
	to, toOK := parseOffset(sub.Props.Get(goical.PropTimezoneOffsetTo))
	if err != nil || !fromOK || !toOK {
		return zoneObservance{}, false
	}
	obs := zoneObservance{from: from, to: to, start: start}
	if ruleProp := sub.Props.Get(goical.PropRecurrenceRule); ruleProp != nil {
		opt, err := rrule.StrToROption(ruleProp.Value)
		if err != nil {
			return zoneObservance{}, false
		}
		opt.Dtstart = start
		if _, err = rrule.NewRRule(*opt); err != nil {
			return zoneObservance{}, false
		}
		obs.rule = opt
	}
	for _, prop := range sub.Props[goical.PropRecurrenceDates] {
		for _, raw := range strings.Split(prop.Value, ",") {
			when, err := time.ParseInLocation(tzDateTimeLayout, strings.TrimSpace(raw), time.UTC)
			if err != nil {
				return zoneObservance{}, false
			}
			obs.rdates = append(obs.rdates, when)
		}
	}
	return obs, true
}

// parseOffset reads an RFC 5545 UTC offset (+HHMM or +HHMMSS) into seconds east of UTC.
func parseOffset(prop *goical.Prop) (int, bool) {
	if prop == nil {
		return 0, false
	}
	v := strings.TrimSpace(prop.Value)
	if (len(v) != offsetShortLen && len(v) != offsetLongLen) || (v[0] != '+' && v[0] != '-') {
		return 0, false
	}
	hours, hErr := strconv.Atoi(v[1:offsetHourEnd])
	minutes, mErr := strconv.Atoi(v[offsetHourEnd:offsetShortLen])
	secs := 0
	var sErr error
	if len(v) == offsetLongLen {
		secs, sErr = strconv.Atoi(v[offsetShortLen:])
	}
	if hErr != nil || mErr != nil || sErr != nil {
		return 0, false
	}
	seconds := hours*secondsPerHour + minutes*secondsPerMinute + secs
	if v[0] == '-' {
		seconds = -seconds
	}
	return seconds, true
}

// onsets returns the instants in [lo, hi) at which this block's offset takes effect.
func (o zoneObservance) onsets(lo, hi time.Time) []time.Time {
	shift := time.Duration(o.from) * time.Second
	wallLo, wallHi := lo.Add(shift), hi.Add(shift)
	walls := append([]time.Time{o.start}, o.rdates...)
	if rule := o.ruleNear(wallLo); rule != nil {
		walls = append(walls, rule.Between(wallLo, wallHi, true)...)
	}
	var out []time.Time
	for _, w := range walls {
		if !w.Before(wallLo) && w.Before(wallHi) {
			out = append(out, w.Add(-shift))
		}
	}
	return out
}

// latestOnset returns the last instant at or before at when this block took effect.
func (o zoneObservance) latestOnset(at time.Time) (time.Time, bool) {
	shift := time.Duration(o.from) * time.Second
	wall := at.Add(shift)
	var best time.Time
	for _, w := range append([]time.Time{o.start}, o.rdates...) {
		if !w.After(wall) && w.After(best) {
			best = w
		}
	}
	if rule := o.ruleNear(wall); rule != nil {
		if w := rule.Before(wall, true); !w.IsZero() && w.After(best) {
			best = w
		}
	}
	if best.IsZero() {
		return time.Time{}, false
	}
	return best.Add(-shift), true
}

// ruleNear builds the block's recurrence rule for a query around the wall clock near (nil when the
// block has none). rrule-go stops expanding about 292 years after the rule's start (the range of a
// time.Duration); Outlook starts every block in 1601, so a plain yearly rule (interval one, no COUNT)
// is restarted on its own month, day and time in the year before near: for such a rule that start yields
// exactly the same later onsets. Any other rule is expanded from its real start.
func (o zoneObservance) ruleNear(near time.Time) *rrule.RRule {
	if o.rule == nil {
		return nil
	}
	opt := *o.rule
	if opt.Freq == rrule.YEARLY && opt.Interval <= 1 && opt.Count == 0 {
		if rebased := o.start.AddDate(near.Year()-1-o.start.Year(), 0, 0); rebased.After(o.start) {
			opt.Dtstart = rebased
		}
	}
	rule, err := rrule.NewRRule(opt)
	if err != nil {
		return nil
	}
	return rule
}

// offsetAt returns the zone's UTC offset in seconds at an instant: the offset set by the block that took
// effect most recently; before any block has, the offset the earliest block changes from.
func (z customZone) offsetAt(at time.Time) int {
	found := false
	var latest time.Time
	offset := z.observances[0].from
	earliest := z.observances[0].start
	for _, o := range z.observances {
		if onset, ok := o.latestOnset(at); ok && (!found || onset.After(latest)) {
			found, latest, offset = true, onset, o.to
		}
		if !found && o.start.Before(earliest) {
			earliest, offset = o.start, o.from
		}
	}
	return offset
}

// wallToUTC converts a wall clock (held in UTC fields) in this zone to the UTC instant it names. The
// offset is first taken at the wall clock read as UTC, then re-checked at the resulting instant, which
// settles every value except one inside a skipped or repeated hour.
func (z customZone) wallToUTC(wall time.Time) time.Time {
	off := z.offsetAt(wall)
	at := wall.Add(-time.Duration(off) * time.Second)
	if again := z.offsetAt(at); again != off {
		at = wall.Add(-time.Duration(again) * time.Second)
	}
	return at
}

// transitions lists the zone's real offset changes during a calendar year, in order.
func (z customZone) transitions(year int) []tzTransition {
	lo := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	hi := lo.AddDate(1, 0, 0)
	var out []tzTransition
	for _, o := range z.observances {
		if o.from == o.to {
			continue
		}
		for _, at := range o.onsets(lo, hi) {
			out = append(out, tzTransition{at: at, fromOff: o.from, toOff: o.to})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].at.Before(out[j].at) })
	return out
}

// ianaCandidates are the IANA zones a custom definition may be matched against: one per Windows zone,
// sorted so the match is deterministic.
var ianaCandidates = func() []string {
	seen := map[string]bool{}
	var out []string
	for _, iana := range windowsToIANA {
		if !seen[iana] {
			seen[iana] = true
			out = append(out, iana)
		}
	}
	sort.Strings(out)
	return out
}()

// ianaMatch finds an IANA zone that behaves exactly like this definition during year: the same offset at
// the start and middle of the year and the same transitions at the same instants. Matching to a real zone
// keeps a recurring event's wall-clock time across daylight-saving changes in later years too.
func (z customZone) ianaMatch(year int) (string, bool) {
	jan := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(year, time.July, 1, 0, 0, 0, 0, time.UTC)
	want := z.transitions(year)
	for _, name := range ianaCandidates {
		loc, err := time.LoadLocation(name)
		if err != nil {
			continue
		}
		// The two cheap offset probes rule out almost every candidate before the hourly transition scan.
		if _, off := jan.In(loc).Zone(); off != z.offsetAt(jan) {
			continue
		}
		if _, off := mid.In(loc).Zone(); off != z.offsetAt(mid) {
			continue
		}
		if sameTransitions(findTransitions(loc, year), want) {
			return name, true
		}
	}
	return "", false
}

// sameTransitions reports whether two transition lists are identical.
func sameTransitions(a, b []tzTransition) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].at.Equal(b[i].at) || a[i].fromOff != b[i].fromOff || a[i].toOff != b[i].toOff {
			return false
		}
	}
	return true
}

// resolve rewrites a property whose TZID names this definition so go-ical can read it: to the IANA zone
// the definition names or matches; failing that, to UTC values at the definition's own offset. It reports false
// when the property cannot be resolved (no usable definition; a value that is not a DATE-TIME list),
// leaving the caller to degrade it to floating.
func (z customZone) resolve(prop *goical.Prop) bool {
	if z.hint != "" {
		prop.Params.Set(goical.PropTimezoneID, z.hint)
		return true
	}
	if len(z.observances) == 0 {
		return false
	}
	raws := strings.Split(prop.Value, ",")
	walls := make([]time.Time, len(raws))
	for i, raw := range raws {
		wall, err := time.ParseInLocation(tzDateTimeLayout, strings.TrimSpace(raw), time.UTC)
		if err != nil {
			return false
		}
		walls[i] = wall
	}
	if iana, ok := z.ianaMatch(walls[0].Year()); ok {
		prop.Params.Set(goical.PropTimezoneID, iana)
		return true
	}
	for i, wall := range walls {
		raws[i] = z.wallToUTC(wall).Format(utcDateTimeLayout)
	}
	prop.Value = strings.Join(raws, ",")
	delete(prop.Params, goical.PropTimezoneID)
	return true
}
