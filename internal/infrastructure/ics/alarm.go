package ics

import (
	"fmt"
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

// parseAlarms reads the relative-trigger VALARM children of a VEVENT into modelled reminders. An absolute
// or unparseable trigger is skipped rather than failing the import.
func parseAlarms(comp *goical.Component) []domain.Alarm {
	var alarms []domain.Alarm
	for _, child := range comp.Children {
		if child.Name != goical.CompAlarm {
			continue
		}
		trigger := child.Props.Get(goical.PropTrigger)
		if trigger == nil {
			continue
		}
		offset, err := trigger.Duration()
		if err != nil {
			continue
		}
		alarms = append(alarms, domain.NewAlarm(offset))
	}
	return alarms
}

// setAlarms replaces the VEVENT's VALARM children with one DISPLAY reminder per modelled alarm. Existing
// VALARMs (including any preserved through Extra) are removed first so nothing is duplicated; this is why
// an exotic imported alarm (an absolute trigger, an email action) is not preserved.
func setAlarms(comp *goical.Component, alarms []domain.Alarm) {
	var kept []*goical.Component
	for _, child := range comp.Children {
		if child.Name != goical.CompAlarm {
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
