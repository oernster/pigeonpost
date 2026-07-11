package ics

import (
	"fmt"
	"strings"
	"time"

	goical "github.com/emersion/go-ical"

	"github.com/oernster/pigeonpost/internal/domain"
)

// alarmActionDisplay and alarmDescription are the ACTION and DESCRIPTION written for a modelled reminder;
// PigeonPost fires on-screen reminders only.
const (
	alarmActionDisplay = "DISPLAY"
	alarmDescription   = "Reminder"
)

// ownedAlarm reports whether a VALARM is one PigeonPost models as an editable reminder: a relative
// duration trigger with a DISPLAY action (an absent action is treated as DISPLAY). Everything else (an
// absolute trigger, an EMAIL or AUDIO action) is left unmodelled so the Extra pass-through preserves it
// verbatim on export rather than stripping it.
func ownedAlarm(child *goical.Component) bool {
	if child.Name != goical.CompAlarm {
		return false
	}
	trigger := child.Props.Get(goical.PropTrigger)
	if trigger == nil {
		return false
	}
	if _, err := trigger.Duration(); err != nil {
		return false
	}
	action := child.Props.Get(goical.PropAction)
	return action == nil || strings.EqualFold(action.Value, alarmActionDisplay)
}

// parseAlarms reads the DISPLAY relative-trigger VALARM children of a VEVENT into modelled reminders.
// Alarms PigeonPost does not model (an absolute trigger, an EMAIL or AUDIO action) are left for the Extra
// pass-through to carry rather than surfaced as editable reminders.
func parseAlarms(comp *goical.Component) []domain.Alarm {
	var alarms []domain.Alarm
	for _, child := range comp.Children {
		if !ownedAlarm(child) {
			continue
		}
		offset, _ := child.Props.Get(goical.PropTrigger).Duration()
		alarms = append(alarms, domain.NewAlarm(offset))
	}
	return alarms
}

// setAlarms re-emits one DISPLAY reminder per modelled alarm while preserving every VALARM PigeonPost
// does not own. Only the owned DISPLAY reminders are removed first (they are replaced from the model), so
// an exotic imported alarm carried through Extra (an absolute trigger, an EMAIL or AUDIO action) survives
// the round-trip instead of being stripped.
func setAlarms(comp *goical.Component, alarms []domain.Alarm) {
	var kept []*goical.Component
	for _, child := range comp.Children {
		if !ownedAlarm(child) {
			kept = append(kept, child)
		}
	}
	comp.Children = kept
	for _, a := range alarms {
		alarm := goical.NewComponent(goical.CompAlarm)
		alarm.Props.SetText(goical.PropAction, alarmActionDisplay)
		trigger := goical.NewProp(goical.PropTrigger)
		trigger.Value = triggerValue(a.Offset())
		alarm.Props.Set(trigger)
		alarm.Props.SetText(goical.PropDescription, alarmDescription)
		comp.Children = append(comp.Children, alarm)
	}
}

// triggerValue renders an alarm offset as an RFC 5545 duration using the largest whole unit, so a 15
// minute reminder writes -PT15M rather than the library's -PT900S. It is DURATION-typed by default, the
// TRIGGER property's own type, so no VALUE parameter is needed.
func triggerValue(offset time.Duration) string {
	sign, magnitude := "", offset
	if magnitude < 0 {
		sign, magnitude = "-", -magnitude
	}
	switch {
	case magnitude == 0:
		return "PT0S"
	case magnitude%(24*time.Hour) == 0:
		return fmt.Sprintf("%sP%dD", sign, magnitude/(24*time.Hour))
	case magnitude%time.Hour == 0:
		return fmt.Sprintf("%sPT%dH", sign, magnitude/time.Hour)
	case magnitude%time.Minute == 0:
		return fmt.Sprintf("%sPT%dM", sign, magnitude/time.Minute)
	default:
		return fmt.Sprintf("%sPT%dS", sign, magnitude/time.Second)
	}
}
