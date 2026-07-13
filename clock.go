package main

import (
	"time"

	"github.com/google/uuid"
)

// systemClock is the production domain.Clock. It is the only place, alongside the composition root,
// where the wall clock is read; the domain never reads it directly.
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// newMessageID produces a unique Message-ID local part for each outgoing message.
func newMessageID() string {
	return uuid.NewString() + "@pigeonpost"
}

// newOutboxID produces a unique identifier for a queued outbox item.
func newOutboxID() string {
	return uuid.NewString()
}

// newRuleID produces a unique identifier for a filter rule.
func newRuleID() string {
	return uuid.NewString()
}

// newTemplateID produces a unique identifier for a message template.
func newTemplateID() string {
	return uuid.NewString()
}

// newContactID produces a unique identifier for a contact or group created in the app.
func newContactID() string {
	return uuid.NewString()
}

// newCalendarID produces a unique identifier for a calendar or event created in the app.
func newCalendarID() string {
	return uuid.NewString()
}

// newCalendarAccountID produces a unique identifier for a CalDAV/CardDAV account added in the app.
func newCalendarAccountID() string {
	return uuid.NewString()
}
